package provider

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"errors"
	"fmt"
	"log"
	"strconv"
	"strings"
	"sync"
	"sync/atomic"
	"time"

	"github.com/go-sql-driver/mysql"
	"gorm.io/gorm"

	"kandaoni.com/anqicms/model"
)

// 渠道连接管理器的运行默认值（规格 §5）：租约 30 秒，每 10 秒续租。
// 关闭等待 5 秒是控制者裁决：钉钉 Run 退出可能耗时约 45 秒，进程关闭不能无限阻塞。
const (
	channelLeaseTTL              = 30 * time.Second
	channelLeaseRenewInterval    = 10 * time.Second
	channelManagerScanInterval   = 30 * time.Second
	channelManagerShutdownWait   = 5 * time.Second
	channelManagerRebuildBacklog = 8
)

// R21：绑定输入尝试限速。同一用户 10 分钟内最多 5 次失败尝试，成功不计数。
const (
	channelBindingAttemptWindow  = 10 * time.Minute
	channelBindingAttemptLimit   = 5
	channelBindingSuccessReply   = "绑定成功"
	channelBindingRateLimitReply = "尝试过于频繁，请稍后再试"
	channelBindingReplyTimeout   = 10 * time.Second
	channelInboxContentMaxRunes  = 4000
	channelManagerOwnerPrefix    = "channel-manager"
)

// channelManagerOptions 是连接管理器的全部可注入项。
// 生产默认值见 defaultChannelManagerOptions；测试替换适配器工厂与周期，不 mock 被测逻辑。
type channelManagerOptions struct {
	// newAdapter 默认指向 NewPlatformAdapter；测试注入假适配器，只替代平台网络。
	newAdapter   func(name string, creds PlatformCredentials) (PlatformAdapter, bool)
	leaseTTL     time.Duration
	renewEvery   time.Duration
	scanEvery    time.Duration
	shutdownWait time.Duration
	// now 是可注入时间源（租约与绑定限速共用），测试不 sleep 真实时间。
	now func() time.Time
}

func defaultChannelManagerOptions() channelManagerOptions {
	return channelManagerOptions{
		newAdapter:   NewPlatformAdapter,
		leaseTTL:     channelLeaseTTL,
		renewEvery:   channelLeaseRenewInterval,
		scanEvery:    channelManagerScanInterval,
		shutdownWait: channelManagerShutdownWait,
		now:          time.Now,
	}
}

func (o channelManagerOptions) withDefaults() channelManagerOptions {
	defaults := defaultChannelManagerOptions()
	if o.newAdapter == nil {
		o.newAdapter = defaults.newAdapter
	}
	if o.leaseTTL <= 0 {
		o.leaseTTL = defaults.leaseTTL
	}
	if o.renewEvery <= 0 {
		o.renewEvery = defaults.renewEvery
	}
	if o.scanEvery <= 0 {
		o.scanEvery = defaults.scanEvery
	}
	if o.shutdownWait <= 0 {
		o.shutdownWait = defaults.shutdownWait
	}
	if o.now == nil {
		o.now = defaults.now
	}
	return o
}

// channelManagerRegistry 是「每站点一个连接管理器」的登记处。
// 用包级表而不是 Website 字段：Website 会被 cloneWithContext 按值拷贝，
// 值字段里的 mutex/状态会跟着复制，StartChannelManager 的幂等性当场失效。
var channelManagerRegistry = struct {
	sync.Mutex
	bySite map[uint]*channelManager
}{bySite: make(map[uint]*channelManager)}

// StartChannelManager 为当前站点启动渠道连接管理器。
// 幂等：同一站点重复调用不会产生第二份 goroutine。只对 Enabled == 1 的渠道建立连接。
func (w *Website) StartChannelManager(ctx context.Context) {
	w.startChannelManager(ctx, defaultChannelManagerOptions())
}

// StopChannelManager 取消全部运行上下文并等待 worker 退出（上限 5 秒）。
func (w *Website) StopChannelManager() {
	w.stopChannelManager()
}

// RebuildChannel 强制重建单个渠道的连接（配置保存后由 T9 调用，立即生效）。
func (w *Website) RebuildChannel(channelId uint) error {
	channelManagerRegistry.Lock()
	manager := channelManagerRegistry.bySite[w.Id]
	channelManagerRegistry.Unlock()
	if manager == nil {
		return errors.New("渠道管理器未启动")
	}
	if channelId == 0 {
		return errors.New("渠道不存在")
	}
	var channel model.MessageChannel
	if err := w.DB.Where("id = ? AND site_id = ?", channelId, w.Id).Take(&channel).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return errors.New("渠道不存在")
		}
		return errors.New("读取渠道失败")
	}
	manager.requestRebuild(channelId)
	return nil
}

func (w *Website) startChannelManager(ctx context.Context, opts channelManagerOptions) {
	if w == nil || w.DB == nil {
		return
	}
	channelManagerRegistry.Lock()
	if _, exists := channelManagerRegistry.bySite[w.Id]; exists {
		channelManagerRegistry.Unlock()
		return
	}
	manager := newChannelManager(w, opts)
	channelManagerRegistry.bySite[w.Id] = manager
	channelManagerRegistry.Unlock()
	manager.start(ctx)
}

func (w *Website) stopChannelManager() {
	channelManagerRegistry.Lock()
	manager := channelManagerRegistry.bySite[w.Id]
	delete(channelManagerRegistry.bySite, w.Id)
	channelManagerRegistry.Unlock()
	if manager != nil {
		manager.stop()
	}
}

// channelManager 管理一个站点全部渠道的连接生命周期。
type channelManager struct {
	site *Website
	opts channelManagerOptions
	// owner 是本实例的连接持有者标识，写入 ChannelLease.Owner。
	owner string

	ctx    context.Context
	cancel context.CancelFunc

	mu      sync.Mutex
	workers map[uint]*channelWorker

	rebuildCh chan channelRebuildRequest
	wg        sync.WaitGroup
	// abandoned 在停止等待超时后置位：仍在运行的 worker 不得释放租约，
	// 否则新实例会立刻抢到租约并与尚未退出的旧连接并存。
	abandoned atomic.Bool

	bindMu          sync.Mutex
	bindingAttempts map[string]*bindingAttemptState
}

type channelRebuildRequest struct {
	channelId uint
	done      chan struct{}
}

type bindingAttemptState struct {
	windowStart time.Time
	failures    int
}

func newChannelManager(w *Website, opts channelManagerOptions) *channelManager {
	return &channelManager{
		site:            w,
		opts:            opts.withDefaults(),
		owner:           newChannelManagerOwner(w.Id),
		workers:         make(map[uint]*channelWorker),
		rebuildCh:       make(chan channelRebuildRequest, channelManagerRebuildBacklog),
		bindingAttempts: make(map[string]*bindingAttemptState),
	}
}

// newChannelManagerOwner 生成实例级 owner 标识：站点号 + 随机段。
// 随机段让同一站点的两个进程（或测试里的两个实例）互不冒充。
func newChannelManagerOwner(siteId uint) string {
	raw := make([]byte, 8)
	if _, err := rand.Read(raw); err != nil {
		return fmt.Sprintf("%s-%d-%d", channelManagerOwnerPrefix, siteId, time.Now().UnixNano())
	}
	return fmt.Sprintf("%s-%d-%s", channelManagerOwnerPrefix, siteId, hex.EncodeToString(raw))
}

func (m *channelManager) start(parent context.Context) {
	if parent == nil {
		parent = context.Background()
	}
	m.ctx, m.cancel = context.WithCancel(parent)
	m.wg.Add(1)
	go func() {
		defer m.wg.Done()
		m.scanLoop()
	}()
}

func (m *channelManager) stop() {
	if m.cancel == nil {
		return
	}
	m.cancel()
	done := make(chan struct{})
	go func() {
		m.wg.Wait()
		close(done)
	}()
	select {
	case <-done:
	case <-time.After(m.opts.shutdownWait):
		m.abandoned.Store(true)
		log.Printf("[channel] 站点 %d 渠道管理器停止超时（%s），仍有连接未退出，租约留给自然过期",
			m.site.Id, m.opts.shutdownWait)
	}
}

func (m *channelManager) scanLoop() {
	ticker := time.NewTicker(m.opts.scanEvery)
	defer ticker.Stop()
	m.reconcile()
	for {
		select {
		case <-m.ctx.Done():
			return
		case request := <-m.rebuildCh:
			m.rebuildOne(request.channelId)
			close(request.done)
		case <-ticker.C:
			m.reconcile()
		}
	}
}

// requestRebuild 让扫描循环立即处理一次单渠道重建，并等待其完成。
// 若管理器正在停止则直接返回，不阻塞调用方。
func (m *channelManager) requestRebuild(channelId uint) {
	request := channelRebuildRequest{channelId: channelId, done: make(chan struct{})}
	select {
	case m.rebuildCh <- request:
	case <-m.ctx.Done():
		return
	}
	select {
	case <-request.done:
	case <-m.ctx.Done():
	}
}

func (m *channelManager) enabledChannels() ([]model.MessageChannel, error) {
	if !m.site.DB.Migrator().HasTable(&model.MessageChannel{}) {
		// 老库可能还没迁移渠道表：不阻断站点初始化，也不 panic。
		return nil, nil
	}
	var channels []model.MessageChannel
	err := m.site.DB.Where("site_id = ? AND enabled = ?", m.site.Id, 1).Find(&channels).Error
	return channels, err
}

// reconcile 对比「已启用渠道」与「正在运行的 worker」：
// 消失/禁用 → 停连接并释放租约；配置变化 → 停旧连接（保持租约）后重建；新增 → 启动。
func (m *channelManager) reconcile() {
	channels, err := m.enabledChannels()
	if err != nil {
		log.Printf("[channel] 站点 %d 读取渠道配置失败：%s", m.site.Id, sanitizePlatformMessage(err.Error()))
		return
	}
	desired := make(map[uint]model.MessageChannel, len(channels))
	for _, channel := range channels {
		desired[channel.Id] = channel
	}

	m.mu.Lock()
	ids := make([]uint, 0, len(m.workers))
	for id := range m.workers {
		ids = append(ids, id)
	}
	m.mu.Unlock()

	for _, id := range ids {
		worker := m.worker(id)
		if worker == nil {
			continue
		}
		channel, exists := desired[id]
		if !exists {
			// 渠道被删除或禁用：租约不再有意义，正常释放。
			m.stopWorker(id, true)
			continue
		}
		if worker.signature != channelConfigSignatureOf(&channel) {
			// 配置变化：保持租约（不释放再抢，避免被其它实例截走），
			// 先 cancel 旧 Run 并等它退出，再按新配置建立连接。
			m.stopWorker(id, false)
			m.startWorker(channel)
		}
	}

	for _, channel := range channels {
		if m.worker(channel.Id) == nil {
			m.startWorker(channel)
		}
	}
}

// rebuildOne 是 RebuildChannel 的立即生效路径：无论配置是否变化都强制重建。
func (m *channelManager) rebuildOne(channelId uint) {
	var channel model.MessageChannel
	err := m.site.DB.Where("id = ? AND site_id = ?", channelId, m.site.Id).Take(&channel).Error
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			m.stopWorker(channelId, true)
			return
		}
		log.Printf("[channel] 站点 %d 读取渠道 %d 失败：%s", m.site.Id, channelId, sanitizePlatformMessage(err.Error()))
		return
	}
	if channel.Enabled != 1 {
		m.stopWorker(channelId, true)
		return
	}
	m.stopWorker(channelId, false)
	m.startWorker(channel)
}

func (m *channelManager) worker(id uint) *channelWorker {
	m.mu.Lock()
	defer m.mu.Unlock()
	return m.workers[id]
}

func (m *channelManager) startWorker(channel model.MessageChannel) {
	if m.ctx == nil || m.ctx.Err() != nil {
		return
	}
	ctx, cancel := context.WithCancel(m.ctx)
	worker := &channelWorker{
		manager:   m,
		channel:   channel,
		signature: channelConfigSignatureOf(&channel),
		ctx:       ctx,
		cancel:    cancel,
		done:      make(chan struct{}),
	}
	worker.releaseOnStop.Store(true)

	m.mu.Lock()
	m.workers[channel.Id] = worker
	m.mu.Unlock()

	m.wg.Add(1)
	go func() {
		defer m.wg.Done()
		worker.run()
	}()
}

// stopWorker 取消单个 worker 并等待其退出。
// releaseLease=false 用于重建：旧连接退出后租约仍归本实例，新连接直接续租。
func (m *channelManager) stopWorker(channelId uint, releaseLease bool) {
	m.mu.Lock()
	worker := m.workers[channelId]
	delete(m.workers, channelId)
	m.mu.Unlock()
	if worker == nil {
		return
	}
	if !releaseLease {
		worker.releaseOnStop.Store(false)
	}
	worker.cancel()
	<-worker.done
}

// ---------- 租约 ----------

// leaseSeconds 返回租约时长（秒）。规格值是 30 秒；测试注入更短值。
func (m *channelManager) leaseSeconds() int64 {
	seconds := int64(m.opts.leaseTTL / time.Second)
	if seconds < 1 {
		seconds = 1
	}
	return seconds
}

func (m *channelManager) nowUnix() int64 {
	return m.opts.now().Unix()
}

// acquireLease 尝试取得应用身份对应的连接租约。
//
// 获取/续租/接管全部走条件更新，并以 RowsAffected 判定结果：
//   - 已过期 → 接管，FencingVersion + 1（换主）；
//   - 同一 owner → 续租，FencingVersion 不变；
//   - 其他 owner 且未过期 → 返回 nil（本实例不建立连接）。
//
// 身份字段（AppIdentityHash）只在 NewChannelLease 构造时写入，不用 Updates(map) 改。
func (m *channelManager) acquireLease(channel *model.MessageChannel) *model.ChannelLease {
	hash := model.ComputeAppIdentityHash(channel.Platform, channel.AppId, channel.OrgId)
	now := m.nowUnix()
	expiresAt := now + m.leaseSeconds()

	// 步骤 1：接管已过期租约。条件更新保证只有一个实例能拿到。
	if lease, ok, err := m.takeoverExpiredLease(hash, expiresAt); err != nil {
		log.Printf("[channel] 站点 %d 接管租约失败：%s", m.site.Id, sanitizePlatformMessage(err.Error()))
		return nil
	} else if ok {
		return lease
	}

	// 步骤 2：已有未过期租约。
	var lease model.ChannelLease
	err := m.site.DB.Where("app_identity_hash = ?", hash).Take(&lease).Error
	if err == nil {
		if lease.Owner != m.owner {
			// 其他实例仍持有租约：不抢。
			return nil
		}
		// 同一 owner 续租：栅栏版本不变。
		if !m.extendLease(hash, m.owner, lease.FencingVersion, expiresAt) {
			return nil
		}
		lease.ExpiresAt = expiresAt
		return &lease
	}
	if !errors.Is(err, gorm.ErrRecordNotFound) {
		log.Printf("[channel] 站点 %d 读取租约失败：%s", m.site.Id, sanitizePlatformMessage(err.Error()))
		return nil
	}

	// 步骤 3：新建租约。必须用 NewChannelLease 派生应用身份摘要。
	newLease := model.NewChannelLease(m.site.Id, channel, m.owner, expiresAt)
	// 首个 owner 的栅栏版本从 1 起：0 容易和「未初始化」混淆。
	newLease.FencingVersion = 1
	if err := m.site.DB.Create(newLease).Error; err != nil {
		if isDuplicateKeyError(err) {
			// 并发插入：对方刚建好租约。只有它已经过期才允许接管。
			if lease, ok, takeoverErr := m.takeoverExpiredLease(hash, expiresAt); takeoverErr != nil {
				log.Printf("[channel] 站点 %d 接管并发租约失败：%s", m.site.Id, sanitizePlatformMessage(takeoverErr.Error()))
				return nil
			} else if ok {
				return lease
			}
			return nil
		}
		log.Printf("[channel] 站点 %d 写入租约失败：%s", m.site.Id, sanitizePlatformMessage(err.Error()))
		return nil
	}
	return newLease
}

func (m *channelManager) takeoverExpiredLease(hash string, expiresAt int64) (*model.ChannelLease, bool, error) {
	now := m.nowUnix()
	res := m.site.DB.Model(&model.ChannelLease{}).
		Where("app_identity_hash = ? AND expires_at <= ?", hash, now).
		Updates(map[string]interface{}{
			"site_id":         m.site.Id,
			"owner":           m.owner,
			"expires_at":      expiresAt,
			"fencing_version": gorm.Expr("fencing_version + 1"),
		})
	if res.Error != nil {
		return nil, false, res.Error
	}
	if res.RowsAffected != 1 {
		return nil, false, nil
	}
	var lease model.ChannelLease
	if err := m.site.DB.Where("app_identity_hash = ?", hash).Take(&lease).Error; err != nil {
		return nil, false, err
	}
	return &lease, true, nil
}

// renewLease 续租。条件更新匹配不到本实例的 owner + fencing 即视为失去租约。
func (m *channelManager) renewLease(channelId uint, lease *model.ChannelLease) bool {
	if lease == nil {
		return false
	}
	expiresAt := m.nowUnix() + m.leaseSeconds()
	if !m.extendLease(lease.AppIdentityHash, lease.Owner, lease.FencingVersion, expiresAt) {
		log.Printf("[channel] 站点 %d 渠道 %d 续租未匹配到本实例的租约", m.site.Id, channelId)
		return false
	}
	lease.ExpiresAt = expiresAt
	return true
}

// extendLease 以 owner + fencing 为条件延长到期时间，并确认租约仍归本实例。
//
// 注意 MySQL 的 affected-rows 语义：值未变化时也返回 0。续租间隔小于 1 秒时
// （测试注入的短周期），expires_at 可能落在同一秒而「写入相同值」，RowsAffected 为 0
// 并不代表失去租约。因此 0 行时回读一次：owner + fencing 仍是自己即为成功；
// 被换主会让两者不同，按失去租约处理。RowsAffected > 1 在唯一索引下不可能，直接判失败。
func (m *channelManager) extendLease(hash, owner string, fencingVersion, expiresAt int64) bool {
	res := m.site.DB.Model(&model.ChannelLease{}).
		Where("app_identity_hash = ? AND owner = ? AND fencing_version = ?", hash, owner, fencingVersion).
		Update("expires_at", expiresAt)
	if res.Error != nil {
		log.Printf("[channel] 站点 %d 续租失败：%s", m.site.Id, sanitizePlatformMessage(res.Error.Error()))
		return false
	}
	if res.RowsAffected == 1 {
		return true
	}
	if res.RowsAffected == 0 {
		var current model.ChannelLease
		if err := m.site.DB.Where("app_identity_hash = ?", hash).Take(&current).Error; err != nil {
			return false
		}
		return current.Owner == owner && current.FencingVersion == fencingVersion
	}
	return false
}

// releaseLease 只在 owner + fencing 仍匹配时删除租约。
func (m *channelManager) releaseLease(lease *model.ChannelLease) {
	if lease == nil {
		return
	}
	res := m.site.DB.
		Where("app_identity_hash = ? AND owner = ? AND fencing_version = ?",
			lease.AppIdentityHash, lease.Owner, lease.FencingVersion).
		Delete(&model.ChannelLease{})
	if res.Error != nil {
		log.Printf("[channel] 站点 %d 释放租约失败：%s", m.site.Id, sanitizePlatformMessage(res.Error.Error()))
	}
}

// ---------- R21 绑定尝试限速 ----------

func (m *channelManager) allowBindingAttempt(key string) bool {
	now := m.opts.now()
	m.bindMu.Lock()
	defer m.bindMu.Unlock()
	state := m.bindingAttempts[key]
	if state == nil || now.Sub(state.windowStart) >= channelBindingAttemptWindow {
		return true
	}
	return state.failures < channelBindingAttemptLimit
}

func (m *channelManager) recordBindingFailure(key string) {
	now := m.opts.now()
	m.bindMu.Lock()
	defer m.bindMu.Unlock()
	state := m.bindingAttempts[key]
	if state == nil || now.Sub(state.windowStart) >= channelBindingAttemptWindow {
		m.bindingAttempts[key] = &bindingAttemptState{windowStart: now, failures: 1}
		return
	}
	state.failures++
}

func (m *channelManager) clearBindingFailures(key string) {
	m.bindMu.Lock()
	defer m.bindMu.Unlock()
	delete(m.bindingAttempts, key)
}

// ---------- 通用工具 ----------

// channelConfigSignature 是判断「需要重建」的配置指纹（控制者裁决的字段集合）。
type channelConfigSignature struct {
	Id        uint
	Revision  int
	Enabled   int
	AppId     string
	OrgId     string
	AppSecret string
}

func channelConfigSignatureOf(channel *model.MessageChannel) channelConfigSignature {
	return channelConfigSignature{
		Id:        channel.Id,
		Revision:  channel.Revision,
		Enabled:   channel.Enabled,
		AppId:     channel.AppId,
		OrgId:     channel.OrgId,
		AppSecret: channel.AppSecret,
	}
}

// isDuplicateKeyError 识别 MySQL 1062（唯一键冲突）。
// 项目没有开启 gorm TranslateError，只能从驱动错误类型判断，外加文本兜底。
func isDuplicateKeyError(err error) bool {
	if err == nil {
		return false
	}
	var mysqlErr *mysql.MySQLError
	if errors.As(err, &mysqlErr) {
		return mysqlErr.Number == 1062
	}
	message := err.Error()
	return strings.Contains(message, "Duplicate entry") || strings.Contains(message, "1062")
}

// channelBindingAttemptKey 是 R21 的限速键摘要：站点 + 渠道 + 平台用户。
func channelBindingAttemptKey(siteId, channelId uint, userId string) string {
	return channelDigest(
		strconv.FormatUint(uint64(siteId), 10),
		strconv.FormatUint(uint64(channelId), 10),
		userId,
	)
}

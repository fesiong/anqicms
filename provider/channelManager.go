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
	// 未取得租约 / 失去租约后的重试退避：1 秒起，60 秒封顶（规格 §5）。
	channelRetryBackoffBase = time.Second
	channelRetryBackoffMax  = 60 * time.Second
)

var (
	// errChannelWorkerStopTimeout 表示旧连接没有在时限内退出。
	// 调用方不得在此情况下建立新连接：否则新旧连接会同时接收平台消息。
	errChannelWorkerStopTimeout = errors.New("旧连接未在时限内退出，请稍后重试")
	// errChannelManagerStopped 表示管理器已停止，拒绝新的重建请求。
	errChannelManagerStopped = errors.New("渠道管理器已停止")
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
	newAdapter func(name string, creds PlatformCredentials) (PlatformAdapter, bool)
	// resolveOrg 在建连前自动获取组织标识（默认 ResolvePlatformOrganization）。
	// 取不到不算失败：连接按「首条入站事件锚定」模式建立。测试注入固定值/错误。
	resolveOrg   func(ctx context.Context, platform string, creds PlatformCredentials) (string, error)
	leaseTTL     time.Duration
	renewEvery   time.Duration
	scanEvery    time.Duration
	shutdownWait time.Duration
	// retryBackoffBase/retryBackoffMax 控制「未取得租约 / 失去租约」后的重试退避。
	retryBackoffBase time.Duration
	retryBackoffMax  time.Duration
	// inboxPollEvery 是入站消费者的空闲轮询间隔（默认 1 秒，测试注入更短值）。
	inboxPollEvery time.Duration
	// now 是可注入时间源（租约与绑定限速共用），测试不 sleep 真实时间。
	now func() time.Time
}

func defaultChannelManagerOptions() channelManagerOptions {
	return channelManagerOptions{
		newAdapter:       NewPlatformAdapter,
		resolveOrg:       ResolvePlatformOrganization,
		leaseTTL:         channelLeaseTTL,
		renewEvery:       channelLeaseRenewInterval,
		scanEvery:        channelManagerScanInterval,
		shutdownWait:     channelManagerShutdownWait,
		retryBackoffBase: channelRetryBackoffBase,
		retryBackoffMax:  channelRetryBackoffMax,
		inboxPollEvery:   channelInboxPollInterval,
		now:              time.Now,
	}
}

func (o channelManagerOptions) withDefaults() channelManagerOptions {
	defaults := defaultChannelManagerOptions()
	if o.newAdapter == nil {
		o.newAdapter = defaults.newAdapter
	}
	if o.resolveOrg == nil {
		o.resolveOrg = defaults.resolveOrg
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
	if o.retryBackoffBase <= 0 {
		o.retryBackoffBase = defaults.retryBackoffBase
	}
	if o.retryBackoffMax < o.retryBackoffBase {
		o.retryBackoffMax = defaults.retryBackoffMax
	}
	if o.inboxPollEvery <= 0 {
		o.inboxPollEvery = defaults.inboxPollEvery
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

// channelManagerOnInit 控制「站点初始化时是否自动建立渠道长连接」。
//
// 默认关闭：测试与命令行工具会频繁调用 InitWebsite，若默认开启，
// 只要库里存在 enabled=1 的渠道就会用真实凭据建连并写真实租约。
// 进程入口 bootstrap.Serve 必须在 InitWebsites 之前显式打开。
var channelManagerOnInit atomic.Bool

// EnableChannelManagerOnInit 打开「站点初始化时启动渠道管理器」的开关。
// 只应由进程入口（bootstrap.Serve）在 InitWebsites 之前调用一次。
//
// 它只写包级原子量，不读数据库：全新安装时启动阶段还没有 config.json
// （GetDefaultDB() == nil），开关也必须在那一刻就打开，安装流程随后调用的
// InitWebsite 才能启动管理器。
func EnableChannelManagerOnInit() {
	channelManagerOnInit.Store(true)
}

// startChannelManagerOnInit 是「站点初始化时启动渠道管理器」的唯一入口。
// 生产直接调用 StartChannelManager；测试替换它，在带 enabled 渠道的开发库上
// 也能验证「开关打开才启动」这条接线，而不会用真实凭据去连平台。
var startChannelManagerOnInit = func(w *Website) {
	w.StartChannelManager(context.Background())
}

// StartChannelManager 为当前站点启动渠道连接管理器。
// 幂等：同一站点（同一 *Website/DB 身份）重复调用不会产生第二份 goroutine。
// 只对 Enabled == 1 的渠道建立连接。
//
// ctx 只能是进程级上下文：管理器生命周期跟随它，传入请求级 ctx 会在请求结束时
// 把该站点的全部渠道连接一起取消。
func (w *Website) StartChannelManager(ctx context.Context) {
	w.startChannelManager(ctx, defaultChannelManagerOptions())
}

// StopChannelManager 取消全部运行上下文并等待 worker 退出（上限 5 秒）。
func (w *Website) StopChannelManager() {
	w.stopChannelManager()
}

// RebuildChannel 强制重建单个渠道的连接（配置保存后由 T9 调用，立即生效）。
// 旧连接未在时限内退出时返回错误：此时不会建立新连接，下一次扫描会继续尝试。
//
// 刻意**不**先校验渠道行是否存在：删除渠道后同样要立刻通知管理器，
// 而那时行已经没了。存在性判断由 rebuildOne 统一做（读不到行 → 停 worker 并释放租约）。
func (w *Website) RebuildChannel(channelId uint) error {
	channelManagerRegistry.Lock()
	manager := channelManagerRegistry.bySite[w.Id]
	channelManagerRegistry.Unlock()
	if channelId == 0 {
		return errors.New("渠道不存在")
	}
	if manager == nil {
		// 与 requestRebuild 的内部失败用同一个哨兵：调用方只需要区分
		// 「管理器没在跑（可忽略）」与「旧连接没退出（需回执给管理员）」。
		return errChannelManagerStopped
	}
	return manager.requestRebuild(channelId)
}

// currentChannelRuntime 返回渠道在本进程内的连接运行态（只读，R87）。
//
// running 表示该渠道的 worker 已取得租约并在跑适配器；leaseExpiresAt 取自 worker
// 持有的租约快照（0 表示当前没有租约）。只暴露这两个值：
// 应用身份摘要、owner 原文都属于「换主」判据，不能进后台响应。
// 多实例部署时这是「本进程视角」，不是集群共识。
func currentChannelRuntime(siteId, channelId uint) (running bool, leaseExpiresAt int64) {
	if siteId == 0 || channelId == 0 {
		return false, 0
	}
	channelManagerRegistry.Lock()
	manager := channelManagerRegistry.bySite[siteId]
	channelManagerRegistry.Unlock()
	if manager == nil {
		return false, 0
	}
	worker := manager.worker(channelId)
	if worker == nil {
		return false, 0
	}
	if lease := worker.leaseSnapshot(); lease != nil {
		leaseExpiresAt = lease.ExpiresAt
	}
	if worker.abandoned.Load() {
		return false, leaseExpiresAt
	}
	return worker.currentState() == channelWorkerRunning, leaseExpiresAt
}

func (w *Website) startChannelManager(ctx context.Context, opts channelManagerOptions) {
	if w == nil || w.DB == nil {
		return
	}
	channelManagerRegistry.Lock()
	if existing, exists := channelManagerRegistry.bySite[w.Id]; exists {
		if existing.servesSite(w) {
			channelManagerRegistry.Unlock()
			return
		}
		// 同一个 site_id 换成了新的 *Website / 新 DB 句柄（InitWebsite 重入、
		// 站点重新连库）：旧管理器还绑在旧句柄上，必须先停掉再重建。
		delete(channelManagerRegistry.bySite, w.Id)
		channelManagerRegistry.Unlock()
		log.Printf("[channel] 站点 %d 的渠道管理器绑定了旧的站点/数据库句柄，先停止旧管理器再重建", w.Id)
		existing.stop()
		channelManagerRegistry.Lock()
	}
	manager := newChannelManager(w, opts)
	channelManagerRegistry.bySite[w.Id] = manager
	channelManagerRegistry.Unlock()
	manager.start(ctx)
}

// servesSite 判断管理器是否仍然服务于同一个站点实例与同一个 DB 句柄。
func (m *channelManager) servesSite(w *Website) bool {
	return m.site == w && m.site.DB == w.DB
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

	rebuildCh chan *channelRebuildRequest
	wg        sync.WaitGroup
	// abandoned 在停止等待超时后置位：仍在运行的 worker 不得释放租约，
	// 否则新实例会立刻抢到租约并与尚未退出的旧连接并存。
	abandoned atomic.Bool

	// retries 记录「未取得租约 / 失去租约」渠道的退避状态（1s 起、60s 封顶）。
	// 按渠道而非按 worker 记录：重试会替换 worker，退避必须跨 worker 延续。
	retryMu sync.Mutex
	retries map[uint]*channelRetryState

	bindMu          sync.Mutex
	bindingAttempts map[string]*bindingAttemptState
}

type channelRebuildRequest struct {
	channelId uint
	done      chan struct{}
	err       error
}

// channelRetryState 是单个渠道的可重试退避状态。
type channelRetryState struct {
	backoff time.Duration
	nextAt  time.Time
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
		rebuildCh:       make(chan *channelRebuildRequest, channelManagerRebuildBacklog),
		retries:         make(map[uint]*channelRetryState),
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
			request.err = m.rebuildOne(request.channelId)
			close(request.done)
		case <-ticker.C:
			m.reconcile()
		}
	}
}

// requestRebuild 让扫描循环立即处理一次单渠道重建，并等待其完成。
// 旧连接未在时限内退出时返回错误；管理器正在停止时同样返回错误，不阻塞调用方。
func (m *channelManager) requestRebuild(channelId uint) error {
	if m.ctx == nil {
		return errChannelManagerStopped
	}
	request := &channelRebuildRequest{channelId: channelId, done: make(chan struct{})}
	select {
	case m.rebuildCh <- request:
	case <-m.ctx.Done():
		return errChannelManagerStopped
	}
	select {
	case <-request.done:
		return request.err
	case <-m.ctx.Done():
		return errChannelManagerStopped
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

// resolveOrganizationFor 在建连前自动向平台索要组织标识（默认实现：
// ResolvePlatformOrganization）。失败不阻断建连：worker 会转为事件锚定模式。
func (m *channelManager) resolveOrganizationFor(ctx context.Context, channel *model.MessageChannel) (string, error) {
	return m.opts.resolveOrg(ctx, channel.Platform, PlatformCredentials{
		AppId:          channel.AppId,
		AppSecret:      channel.AppSecret,
		OrganizationId: channel.OrgId,
	})
}

// persistChannelOrganization 把自动获取/锚定到的组织标识写入渠道行。
//
// 条件更新：只在本行组织标识仍为空时写入。管理员手工保存的值、或另一条路径
// （另一实例锚定/后台保存）先写入的值优先，后台自动获取绝不覆盖它们。
// 应用身份摘要必须同步更新：唯一索引与连接租约都按它取键，摘要停在旧值会让
// 「同一企业应用全局唯一」的判断失真。
//
// 返回值是写入后库里的实际组织标识。CAS 未命中（已被别人写入）时也返回库中现值，
// 由调用方决定采用哪个；行已不存在时返回错误。
func (m *channelManager) persistChannelOrganization(channel *model.MessageChannel, org string) (string, error) {
	org = strings.TrimSpace(org)
	if org == "" {
		return "", errors.New("组织标识不能为空")
	}
	res := m.site.DB.Model(&model.MessageChannel{}).
		Where("`id` = ? AND `site_id` = ? AND (`org_id` IS NULL OR TRIM(`org_id`) = '')", channel.Id, m.site.Id).
		Updates(map[string]any{
			"org_id":            org,
			"app_identity_hash": model.ComputeAppIdentityHash(channel.Platform, channel.AppId, org),
			"updated_time":      m.nowUnix(),
		})
	if res.Error != nil {
		return "", res.Error
	}
	if res.RowsAffected == 1 {
		channel.OrgId = org
		return org, nil
	}
	var current model.MessageChannel
	if err := m.site.DB.Where("`id` = ? AND `site_id` = ?", channel.Id, m.site.Id).Take(&current).Error; err != nil {
		return "", err
	}
	currentOrg := strings.TrimSpace(current.OrgId)
	if currentOrg == "" {
		// 条件更新没命中、回读仍是空：只可能是并发写删把行改回去了。
		return "", errors.New("组织标识写入未生效")
	}
	channel.OrgId = currentOrg
	return currentOrg, nil
}

// leaseIdentityStillValid 判断 worker 手里的租约摘要是否仍对应渠道当前的应用身份。
//
// 组织标识被自动获取/锚定后摘要会变化。此时旧摘要的租约必须释放：没有人会再用
// 旧摘要取租约，它会永远留在表里成为孤儿行；重建时若误保留，还会让「旧连接已退出、
// 新连接复用租约」的前提不成立。
func (m *channelManager) leaseIdentityStillValid(worker *channelWorker, channel *model.MessageChannel) bool {
	lease := worker.leaseSnapshot()
	if lease == nil {
		return true
	}
	return lease.AppIdentityHash == model.ComputeAppIdentityHash(channel.Platform, channel.AppId, channel.OrgId)
}

// reconcile 对比「已启用渠道」与「正在运行的 worker」：
// 消失/禁用 → 停连接并释放租约；配置变化 → 停旧连接（保持租约）后重建；新增 → 启动。
// 「未取得租约 / 失去租约」的 worker 按退避重试；旧连接未退出前绝不启动新连接。
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
			if m.stopWorker(id, true) {
				m.clearRetry(id)
			}
			continue
		}
		if worker.abandoned.Load() {
			// 上一轮停止超时：旧连接仍在跑。继续等它退出，退出前不建新连接。
			if m.stopWorker(id, worker.pendingReleaseValue()) {
				log.Printf("[channel] 站点 %d 渠道 %d 的旧连接已退出，恢复该渠道", m.site.Id, id)
			}
			continue
		}
		if worker.configSignature() != channelConfigSignatureOf(&channel) {
			// 配置变化：保持租约（不释放再抢，避免被其它实例截走），
			// 先 cancel 旧 Run 并等它退出，再按新配置建立连接。
			// 例外：应用身份摘要变了（组织标识被自动获取/锚定）——旧摘要的租约
			// 不能再保留，否则它会成为永远不会被再取的孤儿行。
			m.clearRetry(id)
			if !m.stopWorker(id, !m.leaseIdentityStillValid(worker, &channel)) {
				// 旧连接没在时限内退出：本轮不重建，等下一轮。
				continue
			}
			m.startWorker(channel)
			continue
		}
		if worker.currentState() != channelWorkerRetryable {
			// 终态（Run 已返回）：不自动重连，等配置变化或显式重建。
			continue
		}
		if !m.retryDue(id) {
			continue
		}
		// 可重试：先移除退出的 worker，再重新走一遍「抢租约 → Run」。
		if !m.stopWorker(id, true) {
			continue
		}
		m.startWorker(channel)
	}

	for _, channel := range channels {
		if m.worker(channel.Id) == nil {
			m.startWorker(channel)
		}
	}
}

// rebuildOne 是 RebuildChannel 的立即生效路径：无论配置是否变化都强制重建。
// 返回错误表示旧连接未在时限内退出（此时不建立新连接）。
// 「渠道已删除 / 已禁用」同样要等旧连接退出才返回成功：旧连接还在收消息时
// 回一句成功，会让保存配置的管理员以为已经停了。
func (m *channelManager) rebuildOne(channelId uint) error {
	var channel model.MessageChannel
	err := m.site.DB.Where("id = ? AND site_id = ?", channelId, m.site.Id).Take(&channel).Error
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			if m.stopWorker(channelId, true) {
				m.clearRetry(channelId)
				return nil
			}
			return errChannelWorkerStopTimeout
		}
		log.Printf("[channel] 站点 %d 读取渠道 %d 失败：%s", m.site.Id, channelId, sanitizePlatformMessage(err.Error()))
		return errors.New("读取渠道失败")
	}
	if channel.Enabled != 1 {
		if m.stopWorker(channelId, true) {
			m.clearRetry(channelId)
			return nil
		}
		return errChannelWorkerStopTimeout
	}
	m.clearRetry(channelId)
	// 应用身份摘要变化（组织标识被自动获取/锚定）时旧租约必须释放，
	// 否则它会留在表里成为孤儿行；摘要未变时保持租约，避免被其它实例截走。
	releaseLease := true
	if worker := m.worker(channelId); worker != nil {
		releaseLease = !m.leaseIdentityStillValid(worker, &channel)
	}
	if !m.stopWorker(channelId, releaseLease) {
		return errChannelWorkerStopTimeout
	}
	m.startWorker(channel)
	return nil
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
		manager: m,
		channel: channel,
		ctx:     ctx,
		cancel:  cancel,
		done:    make(chan struct{}),
	}
	worker.signature.Store(channelConfigSignatureOf(&channel))
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

// stopWorker 取消单个 worker 并在 shutdownWait 内等待其退出。
// releaseLease=false 用于重建：旧连接退出后租约仍归本实例，新连接直接续租。
//
// 返回 false 表示旧连接没有在时限内退出：此时 worker 仍留在表里并标记 abandoned，
// 租约不释放、也不会启动新连接；下一轮扫描继续尝试停止/重建。
func (m *channelManager) stopWorker(channelId uint, releaseLease bool) bool {
	m.mu.Lock()
	worker := m.workers[channelId]
	m.mu.Unlock()
	if worker == nil {
		return true
	}
	// 记录释放意图：超时后下一轮重试沿用同一个意图，不因重试把租约误释放。
	worker.pendingRelease.Store(releaseLease)
	if !releaseLease {
		worker.releaseOnStop.Store(false)
	}
	worker.cancel()
	return m.awaitWorkerStop(channelId, worker)
}

// awaitWorkerStop 等 worker 退出，超时按 abandoned 语义处理。
func (m *channelManager) awaitWorkerStop(channelId uint, worker *channelWorker) bool {
	if worker.waitExit(m.opts.shutdownWait) {
		m.removeWorker(channelId, worker)
		// 正常路径下 worker 自己已经释放过租约（releaseOnStop），这里只处理
		// 「超时放弃时被抑制了释放」的收尾，条件删除保证重复释放无副作用。
		if worker.pendingReleaseValue() && !worker.leaseLost.Load() && !m.abandoned.Load() {
			m.releaseLease(worker.leaseSnapshot())
		}
		return true
	}
	worker.abandoned.Store(true)
	// 旧连接还没退出：此时释放租约会让别的实例立刻建立第二条连接。
	worker.releaseOnStop.Store(false)
	log.Printf("[channel] 站点 %d 渠道 %d 旧连接未在 %s 内退出，保留租约并等待其退出后重建",
		m.site.Id, channelId, m.opts.shutdownWait)
	return false
}

// removeWorker 仅在表里仍是同一个 worker 时删除，避免误删刚替换上的新 worker。
func (m *channelManager) removeWorker(channelId uint, worker *channelWorker) {
	m.mu.Lock()
	if m.workers[channelId] == worker {
		delete(m.workers, channelId)
	}
	m.mu.Unlock()
}

// ---------- 可重试退避 ----------

// noteWorkerRunning 在 worker 真正开始 Run 时调用：该渠道的重试退避清零。
func (m *channelManager) noteWorkerRunning(worker *channelWorker) {
	m.mu.Lock()
	current := m.workers[worker.channel.Id] == worker
	m.mu.Unlock()
	if !current {
		return
	}
	m.clearRetry(worker.channel.Id)
}

// noteWorkerRetryable 为可重试退出安排下一次尝试（1s 起，每次翻倍，60s 封顶）。
func (m *channelManager) noteWorkerRetryable(channelId uint) {
	m.retryMu.Lock()
	defer m.retryMu.Unlock()
	state := m.retries[channelId]
	if state == nil {
		state = &channelRetryState{}
		m.retries[channelId] = state
	}
	if state.backoff <= 0 {
		state.backoff = m.opts.retryBackoffBase
	} else {
		state.backoff *= 2
		if state.backoff > m.opts.retryBackoffMax {
			state.backoff = m.opts.retryBackoffMax
		}
	}
	state.nextAt = m.opts.now().Add(state.backoff)
}

// retryDue 判断该渠道是否已到下一次尝试时间。
func (m *channelManager) retryDue(channelId uint) bool {
	m.retryMu.Lock()
	defer m.retryMu.Unlock()
	state := m.retries[channelId]
	if state == nil {
		return true
	}
	return !m.opts.now().Before(state.nextAt)
}

func (m *channelManager) clearRetry(channelId uint) {
	m.retryMu.Lock()
	defer m.retryMu.Unlock()
	delete(m.retries, channelId)
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
// 只认驱动返回的 *mysql.MySQLError：文本兜底（例如 strings.Contains(err, "1062")）
// 会把错误消息里恰好带这个数字的其它错误（如连接号、ID）误判成唯一键冲突并静默吞掉。
// 项目没有开启 gorm TranslateError，驱动错误类型是唯一可靠依据。
func isDuplicateKeyError(err error) bool {
	if err == nil {
		return false
	}
	var mysqlErr *mysql.MySQLError
	if errors.As(err, &mysqlErr) {
		return mysqlErr.Number == 1062
	}
	return false
}

// channelBindingAttemptKey 是 R21 的限速键摘要：站点 + 渠道 + 平台用户。
func channelBindingAttemptKey(siteId, channelId uint, userId string) string {
	return channelDigest(
		strconv.FormatUint(uint64(siteId), 10),
		strconv.FormatUint(uint64(channelId), 10),
		userId,
	)
}

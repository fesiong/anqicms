package provider

import (
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"
	"log"
	"strings"
	"sync"
	"testing"
	"time"

	"gorm.io/gorm"
	"kandaoni.com/anqicms/model"
)

// ============================================================
// 测试夹具
// ============================================================

// channelManagerTestSite 连接本机开发库（站点 1），迁移本任务用到的表。
//
// InitWebsite 会用生产参数启动站点 1 的渠道管理器；测试必须注入假适配器与短周期，
// 所以先停掉生产管理器（否则它会用真实凭据去连网络并抢走测试租约）。
// 同一个测试二进制里每个用例都会重复停/起，StartChannelManager 本身是幂等的。
func channelManagerTestSite(t *testing.T) *Website {
	t.Helper()
	w := initTestSite(t)
	if w == nil || w.DB == nil {
		t.Fatal("站点 1 未初始化，无法连接开发库")
	}
	err := w.DB.AutoMigrate(
		&model.MessageChannel{},
		&model.ChannelLease{},
		&model.ChannelInbox{},
		&model.ChannelBinding{},
		&model.ChannelBindingToken{},
		&model.ChannelTarget{},
		&model.ChannelApproval{},
	)
	if err != nil {
		t.Fatalf("迁移渠道管理器相关表失败：%v", err)
	}
	w.StopChannelManager()
	t.Cleanup(func() { w.StopChannelManager() })
	return w
}

// newManagerTestChannel 自建一条渠道，AppId/OrgId/AppSecret 都带随机后缀
// （uk_app_identity_hash 全局唯一，固定值会撞上残留行）。
// 清理以自己创建的渠道 ID 为轴，绝不按 site_id 批量删。
func newManagerTestChannel(t *testing.T, w *Website, enabled bool) *model.MessageChannel {
	t.Helper()
	suffix := fmt.Sprintf("%d", time.Now().UnixNano())
	enabledValue := 0
	if enabled {
		enabledValue = 1
	}
	ch := &model.MessageChannel{
		SiteId:    w.Id,
		Platform:  model.ChannelPlatformFeishu,
		Name:      "管理器测试-" + suffix,
		Enabled:   enabledValue,
		AppId:     "mgr_app_" + suffix,
		OrgId:     "mgr_org_" + suffix,
		AppSecret: "mgr_secret_" + suffix,
		Revision:  1,
	}
	if err := w.DB.Create(ch).Error; err != nil {
		t.Fatalf("创建测试渠道失败：%v", err)
	}
	hash := model.ComputeAppIdentityHash(ch.Platform, ch.AppId, ch.OrgId)

	t.Cleanup(func() {
		var bindingIds []uint
		if err := w.DB.Model(&model.ChannelBinding{}).Where("channel_id = ?", ch.Id).Pluck("id", &bindingIds).Error; err != nil {
			t.Logf("清理渠道 %d 时读取绑定失败：%v", ch.Id, err)
		}
		if len(bindingIds) > 0 {
			if err := w.DB.Where("binding_id IN ?", bindingIds).Delete(&model.ChannelApproval{}).Error; err != nil {
				t.Logf("清理渠道 %d 审批失败：%v", ch.Id, err)
			}
		}
		for _, spec := range []struct {
			name  string
			model interface{}
		}{
			{"入站任务", &model.ChannelInbox{}},
			{"绑定", &model.ChannelBinding{}},
			{"令牌", &model.ChannelBindingToken{}},
			{"目标", &model.ChannelTarget{}},
		} {
			if err := w.DB.Where("channel_id = ?", ch.Id).Delete(spec.model).Error; err != nil {
				t.Logf("清理渠道 %d 的%s失败：%v", ch.Id, spec.name, err)
			}
		}
		if err := w.DB.Where("app_identity_hash = ?", hash).Delete(&model.ChannelLease{}).Error; err != nil {
			t.Logf("清理渠道 %d 租约失败：%v", ch.Id, err)
		}
		if err := w.DB.Where("id = ? AND site_id = ?", ch.Id, w.Id).Delete(&model.MessageChannel{}).Error; err != nil {
			t.Logf("清理渠道 %d 失败：%v", ch.Id, err)
		}
	})
	return ch
}

func channelAppHash(ch *model.MessageChannel) string {
	return model.ComputeAppIdentityHash(ch.Platform, ch.AppId, ch.OrgId)
}

// fakeChannelAdapter 只替代平台网络：Run 阻塞到上下文取消，Send 记录调用。
type fakeChannelAdapter struct {
	appId string

	mu        sync.Mutex
	runCalls  int
	sink      InboundSink
	sendCalls []fakeSendCall

	started chan struct{}
	exited  chan struct{}

	// exitDelay：收到取消后再拖延这么久才返回（模拟 SDK 退出耗时）。
	exitDelay time.Duration
	// ignoreCancel：完全不响应上下文取消（模拟钉钉 dialer 不感知 ctx）。
	ignoreCancel bool
	runErr       error

	startOnce sync.Once
	exitOnce  sync.Once
}

type fakeSendCall struct {
	target TargetRef
	msg    OutboundMessage
}

func newFakeChannelAdapter() *fakeChannelAdapter {
	return &fakeChannelAdapter{
		started: make(chan struct{}),
		exited:  make(chan struct{}),
	}
}

func (f *fakeChannelAdapter) Name() string { return "fake" }

func (f *fakeChannelAdapter) Run(ctx context.Context, creds PlatformCredentials, sink InboundSink) error {
	f.mu.Lock()
	f.runCalls++
	f.sink = sink
	f.mu.Unlock()
	f.startOnce.Do(func() { close(f.started) })

	if f.ignoreCancel {
		time.Sleep(f.exitDelay)
	} else {
		select {
		case <-ctx.Done():
		case <-time.After(10 * time.Second):
		}
		if f.exitDelay > 0 {
			time.Sleep(f.exitDelay)
		}
	}
	f.exitOnce.Do(func() { close(f.exited) })
	return f.runErr
}

func (f *fakeChannelAdapter) Send(ctx context.Context, target TargetRef, msg OutboundMessage) (SendResult, error) {
	f.mu.Lock()
	f.sendCalls = append(f.sendCalls, fakeSendCall{target: target, msg: msg})
	f.mu.Unlock()
	return SendResult{PlatformMessageId: "fake-message"}, nil
}

func (f *fakeChannelAdapter) runCount() int {
	f.mu.Lock()
	defer f.mu.Unlock()
	return f.runCalls
}

func (f *fakeChannelAdapter) sendCount() int {
	f.mu.Lock()
	defer f.mu.Unlock()
	return len(f.sendCalls)
}

func (f *fakeChannelAdapter) sends() []fakeSendCall {
	f.mu.Lock()
	defer f.mu.Unlock()
	out := make([]fakeSendCall, len(f.sendCalls))
	copy(out, f.sendCalls)
	return out
}

func (f *fakeChannelAdapter) waitSink(t *testing.T) InboundSink {
	t.Helper()
	deadline := time.Now().Add(3 * time.Second)
	for time.Now().Before(deadline) {
		f.mu.Lock()
		sink := f.sink
		f.mu.Unlock()
		if sink != nil {
			return sink
		}
		time.Sleep(5 * time.Millisecond)
	}
	t.Fatal("等待假适配器拿到入站 sink 超时")
	return nil
}

func (f *fakeChannelAdapter) waitSends(t *testing.T, want int) []fakeSendCall {
	t.Helper()
	deadline := time.Now().Add(3 * time.Second)
	for time.Now().Before(deadline) {
		if calls := f.sends(); len(calls) >= want {
			return calls
		}
		time.Sleep(5 * time.Millisecond)
	}
	t.Fatalf("等待假适配器发送第 %d 条回执超时", want)
	return nil
}

// fakeChannelFactory 是可替换的适配器工厂接缝。
type fakeChannelFactory struct {
	mu       sync.Mutex
	adapters map[string][]*fakeChannelAdapter
	build    func(appId string, index int) *fakeChannelAdapter
}

func newFakeChannelFactory(build func(appId string, index int) *fakeChannelAdapter) *fakeChannelFactory {
	if build == nil {
		build = func(string, int) *fakeChannelAdapter { return newFakeChannelAdapter() }
	}
	return &fakeChannelFactory{
		adapters: make(map[string][]*fakeChannelAdapter),
		build:    build,
	}
}

func (f *fakeChannelFactory) newAdapter(name string, creds PlatformCredentials) (PlatformAdapter, bool) {
	f.mu.Lock()
	defer f.mu.Unlock()
	index := len(f.adapters[creds.AppId])
	adapter := f.build(creds.AppId, index)
	adapter.appId = creds.AppId
	f.adapters[creds.AppId] = append(f.adapters[creds.AppId], adapter)
	return adapter, true
}

func (f *fakeChannelFactory) adaptersFor(appId string) []*fakeChannelAdapter {
	f.mu.Lock()
	defer f.mu.Unlock()
	return append([]*fakeChannelAdapter(nil), f.adapters[appId]...)
}

func (f *fakeChannelFactory) adapterCount(appId string) int {
	return len(f.adaptersFor(appId))
}

func (f *fakeChannelFactory) runCount(appId string) int {
	total := 0
	for _, adapter := range f.adaptersFor(appId) {
		total += adapter.runCount()
	}
	return total
}

func (f *fakeChannelFactory) waitForRun(t *testing.T, appId string, want int) *fakeChannelAdapter {
	t.Helper()
	deadline := time.Now().Add(3 * time.Second)
	for time.Now().Before(deadline) {
		adapters := f.adaptersFor(appId)
		if len(adapters) >= want {
			adapter := adapters[want-1]
			select {
			case <-adapter.started:
				return adapter
			default:
			}
		}
		time.Sleep(5 * time.Millisecond)
	}
	t.Fatalf("等待渠道 %s 的第 %d 次 Run 调用超时", appId, want)
	return nil
}

func testChannelManagerOptions(factory *fakeChannelFactory) channelManagerOptions {
	opts := defaultChannelManagerOptions()
	opts.newAdapter = factory.newAdapter
	opts.leaseTTL = 5 * time.Second
	opts.renewEvery = 20 * time.Millisecond
	opts.scanEvery = 20 * time.Millisecond
	opts.shutdownWait = 500 * time.Millisecond
	return opts
}

// startTestChannelManager 安装测试管理器；清理登记在渠道清理之后（LIFO），
// 保证先停 worker 再删渠道/租约行。
func startTestChannelManager(t *testing.T, w *Website, opts channelManagerOptions) {
	t.Helper()
	w.StopChannelManager()
	w.startChannelManager(context.Background(), opts)
	t.Cleanup(func() { w.StopChannelManager() })
}

// fakeChannelClock 是可注入时间源：限速窗口测试不 sleep 真实时间。
type fakeChannelClock struct {
	mu sync.Mutex
	at time.Time
}

func newFakeChannelClock() *fakeChannelClock {
	return &fakeChannelClock{at: time.Now()}
}

func (c *fakeChannelClock) Now() time.Time {
	c.mu.Lock()
	defer c.mu.Unlock()
	return c.at
}

func (c *fakeChannelClock) Advance(d time.Duration) {
	c.mu.Lock()
	c.at = c.at.Add(d)
	c.mu.Unlock()
}

type lockedLogBuffer struct {
	mu  sync.Mutex
	buf bytes.Buffer
}

func (b *lockedLogBuffer) Write(p []byte) (int, error) {
	b.mu.Lock()
	defer b.mu.Unlock()
	return b.buf.Write(p)
}

func (b *lockedLogBuffer) String() string {
	b.mu.Lock()
	defer b.mu.Unlock()
	return b.buf.String()
}

func captureProviderLogs(t *testing.T) *lockedLogBuffer {
	t.Helper()
	buf := &lockedLogBuffer{}
	previous := log.Writer()
	log.SetOutput(buf)
	t.Cleanup(func() { log.SetOutput(previous) })
	return buf
}

func waitFakeExit(t *testing.T, adapter *fakeChannelAdapter, timeout time.Duration) {
	t.Helper()
	select {
	case <-adapter.exited:
	case <-time.After(timeout):
		t.Fatal("等待假适配器退出超时")
	}
}

// wantEventDigest 用字面协议独立算入站去重摘要（R22）：站点 \x1f 渠道 \x1f 平台 \x1f EventId。
// 刻意不复用被测的 channelDigest：否则改分隔符/参数顺序时生产与断言同步漂移，断言永远成立。
func wantEventDigest(siteId, channelId uint, platform, eventId string) string {
	joined := fmt.Sprintf("%d\x1f%d\x1f%s\x1f%s", siteId, channelId, platform, eventId)
	return fmt.Sprintf("%x", sha256.Sum256([]byte(joined)))
}

func bindingTestEvent(ch *model.MessageChannel, userId, text string, seq int) InboundEvent {
	return InboundEvent{
		EventId:        fmt.Sprintf("bind-evt-%d-%d", time.Now().UnixNano(), seq),
		Kind:           InboundKindText,
		OrganizationId: ch.OrgId,
		AppId:          ch.AppId,
		UserId:         userId,
		ChatId:         "p2p-" + userId,
		ChatType:       ChatTypeP2P,
		Text:           text,
	}
}

// ============================================================
// 简报要求的 5 个测试
// ============================================================

// TestChannelLeasePreventsSecondOwner 锁定租约的互斥性：同一企业应用同时只允许一个 owner。
// 反向对照不止看返回值：第二个 owner 的假适配器 Run 必须一次都没被调用。
func TestChannelLeasePreventsSecondOwner(t *testing.T) {
	w := channelManagerTestSite(t)
	ch := newManagerTestChannel(t, w, true)

	registryA := newFakeChannelFactory(nil)
	startTestChannelManager(t, w, testChannelManagerOptions(registryA))

	first := registryA.waitForRun(t, ch.AppId, 1)

	// 幂等：重复 Start 不得产生第二份管理器/第二份连接。
	w.StartChannelManager(context.Background())
	if got := registryA.runCount(ch.AppId); got != 1 {
		t.Fatalf("重复 StartChannelManager 后 Run 调用次数为 %d，应为 1", got)
	}

	wantHash := channelAppHash(ch)
	var lease model.ChannelLease
	if err := w.DB.Where("app_identity_hash = ?", wantHash).Take(&lease).Error; err != nil {
		t.Fatalf("没有按应用摘要写入租约：%v", err)
	}
	if lease.AppIdentityHash != wantHash || lease.AppIdentityHash == "" {
		t.Fatalf("租约应用摘要不正确：got %q want %q", lease.AppIdentityHash, wantHash)
	}
	if strings.TrimSpace(lease.Owner) == "" {
		t.Fatal("租约没有 owner")
	}
	if lease.FencingVersion < 1 {
		t.Fatalf("新租约的栅栏版本应 >= 1，实际 %d", lease.FencingVersion)
	}

	// 第二个 owner：独立实例，同一数据库、同一应用摘要。
	registryB := newFakeChannelFactory(nil)
	managerB := newChannelManager(w, testChannelManagerOptions(registryB))
	ctxB, cancelB := context.WithCancel(context.Background())
	managerB.start(ctxB)
	defer func() {
		cancelB()
		managerB.stop()
	}()

	deadline := time.Now().Add(300 * time.Millisecond)
	for time.Now().Before(deadline) {
		time.Sleep(10 * time.Millisecond)
	}

	if got := registryB.runCount(ch.AppId); got != 0 {
		t.Fatalf("第二个 owner 没拿到租约却调用了 Run %d 次", got)
	}
	// 反向对照：第二个管理器确实尝试过（构造了适配器），上面的 0 不是「根本没扫描」空转。
	if got := registryB.adapterCount(ch.AppId); got == 0 {
		t.Fatal("第二个 manager 没有构造任何适配器，Run=0 的断言可能是空转")
	}
	select {
	case <-first.exited:
		t.Fatal("第一个 owner 的连接被错误取消了")
	default:
	}
}

// TestLostLeaseCancelsRunningChannel 锁定「续租 RowsAffected != 1 即失去租约」：
// 立即取消该渠道的 Run，且不自动快速重连、不删除新 owner 的租约。
func TestLostLeaseCancelsRunningChannel(t *testing.T) {
	w := channelManagerTestSite(t)
	ch := newManagerTestChannel(t, w, true)

	registry := newFakeChannelFactory(nil)
	startTestChannelManager(t, w, testChannelManagerOptions(registry))
	first := registry.waitForRun(t, ch.AppId, 1)

	// 模拟被另一个实例抢走：owner 换成别人、到期时间延长、栅栏版本递增。
	intruder := fmt.Sprintf("intruder-%d", time.Now().UnixNano())
	res := w.DB.Model(&model.ChannelLease{}).
		Where("app_identity_hash = ?", channelAppHash(ch)).
		Updates(map[string]interface{}{
			"owner":           intruder,
			"expires_at":      time.Now().Add(time.Hour).Unix(),
			"fencing_version": gorm.Expr("fencing_version + 1"),
		})
	if res.Error != nil {
		t.Fatalf("模拟换主失败：%v", res.Error)
	}
	if res.RowsAffected != 1 {
		t.Fatalf("模拟换主影响行数为 %d，应为 1", res.RowsAffected)
	}

	waitFakeExit(t, first, 3*time.Second)

	// 终态：配置没变就不自动重连。
	time.Sleep(150 * time.Millisecond)
	if got := registry.runCount(ch.AppId); got != 1 {
		t.Fatalf("失去租约后自动重连了：Run 调用 %d 次，应为 1", got)
	}

	// 旧 owner 不得把新 owner 的租约删掉。
	var lease model.ChannelLease
	if err := w.DB.Where("app_identity_hash = ?", channelAppHash(ch)).Take(&lease).Error; err != nil {
		t.Fatalf("新 owner 的租约被删除了：%v", err)
	}
	if lease.Owner != intruder {
		t.Fatalf("租约 owner 变成 %q，应为 %q", lease.Owner, intruder)
	}
}

// TestChannelRebuildOnRevisionChange 锁定重建语义：
// 先取消旧 Run 并等其退出，再按新配置重建；重建保持租约（owner / 栅栏版本不变）。
func TestChannelRebuildOnRevisionChange(t *testing.T) {
	w := channelManagerTestSite(t)
	ch := newManagerTestChannel(t, w, true)

	registry := newFakeChannelFactory(nil)
	startTestChannelManager(t, w, testChannelManagerOptions(registry))
	first := registry.waitForRun(t, ch.AppId, 1)

	var leaseBefore model.ChannelLease
	if err := w.DB.Where("app_identity_hash = ?", channelAppHash(ch)).Take(&leaseBefore).Error; err != nil {
		t.Fatalf("读取重建前租约失败：%v", err)
	}

	// 配置 revision 变化由周期扫描发现。
	if res := w.DB.Model(&model.MessageChannel{}).
		Where("id = ? AND site_id = ?", ch.Id, w.Id).
		Update("revision", ch.Revision+1); res.Error != nil || res.RowsAffected != 1 {
		t.Fatalf("更新渠道 revision 失败：err=%v rows=%d", res.Error, res.RowsAffected)
	}
	second := registry.waitForRun(t, ch.AppId, 2)

	// 旧连接必须先退出，新连接才能建立。
	select {
	case <-first.exited:
	default:
		t.Fatal("旧 Run 还没退出就建立了新连接")
	}

	var leaseAfter model.ChannelLease
	if err := w.DB.Where("app_identity_hash = ?", channelAppHash(ch)).Take(&leaseAfter).Error; err != nil {
		t.Fatalf("重建后租约消失了：%v", err)
	}
	if leaseAfter.Owner != leaseBefore.Owner {
		t.Fatalf("重建换了租约 owner：%q -> %q", leaseBefore.Owner, leaseAfter.Owner)
	}
	if leaseAfter.FencingVersion != leaseBefore.FencingVersion {
		t.Fatalf("重建不应递增栅栏版本：%d -> %d", leaseBefore.FencingVersion, leaseAfter.FencingVersion)
	}

	// 显式 RebuildChannel 是立即生效路径：revision 不变也强制重建。
	if err := w.RebuildChannel(ch.Id); err != nil {
		t.Fatalf("显式重建失败：%v", err)
	}
	registry.waitForRun(t, ch.AppId, 3)
	select {
	case <-second.exited:
	default:
		t.Fatal("显式重建时旧 Run 还没退出")
	}
}

// TestChannelDuplicateEventIsIgnored 锁定入站去重：相同 EventHash 只落库一次，重复视为成功。
func TestChannelDuplicateEventIsIgnored(t *testing.T) {
	w := channelManagerTestSite(t)
	ch := newManagerTestChannel(t, w, true)

	registry := newFakeChannelFactory(nil)
	startTestChannelManager(t, w, testChannelManagerOptions(registry))
	adapter := registry.waitForRun(t, ch.AppId, 1)
	sink := adapter.waitSink(t)

	eventId := fmt.Sprintf("evt-%d", time.Now().UnixNano())
	ev := InboundEvent{
		EventId:        eventId,
		Kind:           InboundKindText,
		OrganizationId: ch.OrgId,
		AppId:          ch.AppId,
		UserId:         "dup-user",
		ChatId:         "dup-chat",
		ChatType:       ChatTypeGroup,
		Text:           "  你好，测试  ",
	}
	for i := 0; i < 2; i++ {
		if err := sink(context.Background(), ev); err != nil {
			t.Fatalf("第 %d 次入站处理返回错误（重复事件应视为成功）：%v", i+1, err)
		}
	}

	hash := wantEventDigest(w.Id, ch.Id, ch.Platform, eventId)
	var count int64
	if err := w.DB.Model(&model.ChannelInbox{}).Where("event_hash = ?", hash).Count(&count).Error; err != nil {
		t.Fatalf("统计入站任务失败：%v", err)
	}
	if count != 1 {
		t.Fatalf("相同事件落库 %d 条，应为 1", count)
	}

	var rows []model.ChannelInbox
	if err := w.DB.Where("event_hash = ?", hash).Find(&rows).Error; err != nil {
		t.Fatalf("读取入站任务失败：%v", err)
	}
	if rows[0].Status != model.InboxStatusPending {
		t.Fatalf("入站任务状态应为 pending，实际 %q", rows[0].Status)
	}
	if rows[0].Content != "你好，测试" {
		t.Fatalf("入站正文应保存去空白后的脱敏文本，实际 %q", rows[0].Content)
	}

	// 并发重复同样是成功且只落一条（SDK 会并发调用 sink）。
	const parallel = 8
	parallelEventId := fmt.Sprintf("evt-parallel-%d", time.Now().UnixNano())
	parallelEvent := ev
	parallelEvent.EventId = parallelEventId
	var wg sync.WaitGroup
	errCh := make(chan error, parallel)
	for i := 0; i < parallel; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			errCh <- sink(context.Background(), parallelEvent)
		}()
	}
	wg.Wait()
	close(errCh)
	for err := range errCh {
		if err != nil {
			t.Fatalf("并发重复事件返回错误：%v", err)
		}
	}
	var parallelCount int64
	parallelHash := wantEventDigest(w.Id, ch.Id, ch.Platform, parallelEventId)
	if err := w.DB.Model(&model.ChannelInbox{}).Where("event_hash = ?", parallelHash).Count(&parallelCount).Error; err != nil {
		t.Fatalf("统计并发入站任务失败：%v", err)
	}
	if parallelCount != 1 {
		t.Fatalf("并发重复事件落库 %d 条，应为 1", parallelCount)
	}
}

// TestShutdownWaitsForChannelWorkers 锁定关闭语义：
// Stop 等待 worker 退出（有时限）；协作式渠道释放租约，超时未退出的渠道不释放。
func TestShutdownWaitsForChannelWorkers(t *testing.T) {
	w := channelManagerTestSite(t)
	cooperative := newManagerTestChannel(t, w, true)
	stubborn := newManagerTestChannel(t, w, true)

	registry := newFakeChannelFactory(func(appId string, index int) *fakeChannelAdapter {
		adapter := newFakeChannelAdapter()
		if appId == stubborn.AppId {
			adapter.ignoreCancel = true
			adapter.exitDelay = 600 * time.Millisecond
		} else {
			adapter.exitDelay = 40 * time.Millisecond
		}
		return adapter
	})
	opts := testChannelManagerOptions(registry)
	opts.shutdownWait = 120 * time.Millisecond
	startTestChannelManager(t, w, opts)

	cooperativeAdapter := registry.waitForRun(t, cooperative.AppId, 1)
	stubbornAdapter := registry.waitForRun(t, stubborn.AppId, 1)

	startedAt := time.Now()
	w.StopChannelManager()
	elapsed := time.Since(startedAt)

	if elapsed < 100*time.Millisecond {
		t.Fatalf("Stop 在等待上限前就返回了（%s），没有等待 worker", elapsed)
	}
	if elapsed > 500*time.Millisecond {
		t.Fatalf("Stop 没有在等待上限附近返回（%s）", elapsed)
	}
	select {
	case <-cooperativeAdapter.exited:
	default:
		t.Fatal("协作式 worker 在 Stop 返回时仍未退出")
	}
	select {
	case <-stubbornAdapter.exited:
		t.Fatal("不响应取消的 worker 在等待上限内退出了，无法验证超时路径")
	default:
	}

	// 超时未退出的渠道不得释放租约：让旧连接自然过期，避免新旧连接并存。
	var stubbornLease model.ChannelLease
	err := w.DB.Where("app_identity_hash = ?", channelAppHash(stubborn)).Take(&stubbornLease).Error
	if errors.Is(err, gorm.ErrRecordNotFound) {
		t.Fatal("未在等待上限内退出的渠道释放了租约")
	}
	if err != nil {
		t.Fatalf("读取未退出渠道的租约失败：%v", err)
	}

	// 正常退出的渠道必须释放租约（owner + fencing 匹配时删除）。
	var cooperativeLease model.ChannelLease
	err = w.DB.Where("app_identity_hash = ?", channelAppHash(cooperative)).Take(&cooperativeLease).Error
	if err == nil {
		t.Fatal("正常退出的渠道没有释放租约")
	}
	if !errors.Is(err, gorm.ErrRecordNotFound) {
		t.Fatalf("读取正常退出渠道的租约失败：%v", err)
	}

	// 等不响应取消的 worker 收尾，避免泄漏到后续测试。
	waitFakeExit(t, stubbornAdapter, 3*time.Second)
}

// ============================================================
// 本任务补充的测试
// ============================================================

// TestBindingMessageConsumedWithoutStoringToken 锁定绑定消息的处理：
// 私聊 token → 绑定成功、inbox 无这条、日志与库内都不得出现 token 明文。
func TestBindingMessageConsumedWithoutStoringToken(t *testing.T) {
	w := channelManagerTestSite(t)
	ch := newManagerTestChannel(t, w, true)
	admin := newTestAdmin(t, w, "bindmsg")

	plain, _, err := w.CreateBindingToken(ch.Id, admin.Id)
	if err != nil {
		t.Fatalf("创建绑定令牌失败：%v", err)
	}

	registry := newFakeChannelFactory(nil)
	startTestChannelManager(t, w, testChannelManagerOptions(registry))
	adapter := registry.waitForRun(t, ch.AppId, 1)
	sink := adapter.waitSink(t)

	logs := captureProviderLogs(t)

	userId := fmt.Sprintf("bind-user-%d", time.Now().UnixNano())
	ev := bindingTestEvent(ch, userId, " "+plain+" ", 0)
	if err := sink(context.Background(), ev); err != nil {
		t.Fatalf("绑定消息处理返回错误：%v", err)
	}

	var binding model.ChannelBinding
	if err := w.DB.Where("channel_id = ? AND platform_user_id = ? AND revoked_at = 0", ch.Id, userId).Take(&binding).Error; err != nil {
		t.Fatalf("绑定没有落库：%v", err)
	}
	if binding.AdminId != admin.Id {
		t.Fatalf("绑定管理员为 %d，应为 %d", binding.AdminId, admin.Id)
	}

	// 绑定消息绝不写入 ChannelInbox。
	var inboxCount int64
	if err := w.DB.Model(&model.ChannelInbox{}).Where("channel_id = ?", ch.Id).Count(&inboxCount).Error; err != nil {
		t.Fatalf("统计入站任务失败：%v", err)
	}
	if inboxCount != 0 {
		t.Fatalf("绑定消息被写入了入站任务（%d 条）", inboxCount)
	}

	// 回执：固定中文，发给该私聊用户。
	calls := adapter.waitSends(t, 1)
	if calls[0].msg.Text != "绑定成功" {
		t.Fatalf("绑定回执为 %q，应为 %q", calls[0].msg.Text, "绑定成功")
	}
	if calls[0].target.Kind != TargetKindPrivate {
		t.Fatalf("绑定回执目标类型为 %q，应为 %q", calls[0].target.Kind, TargetKindPrivate)
	}
	if calls[0].target.PlatformTargetId != userId {
		t.Fatalf("绑定回执发给了 %q，应为 %q", calls[0].target.PlatformTargetId, userId)
	}

	// 令牌已被消费。
	var token model.ChannelBindingToken
	if err := w.DB.Where("token_hash = ?", hashBindingToken(plain)).Take(&token).Error; err != nil {
		t.Fatalf("读取绑定令牌失败：%v", err)
	}
	if token.ConsumedAt == 0 {
		t.Fatal("绑定令牌没有被消费")
	}

	// token 明文绝不出现：日志、库内正文都不行。
	if strings.Contains(logs.String(), plain) {
		t.Fatal("日志里出现了绑定令牌明文")
	}
	var leaked int64
	if err := w.DB.Model(&model.ChannelInbox{}).Where("content LIKE ?", "%"+plain+"%").Count(&leaked).Error; err != nil {
		t.Fatalf("检查入站正文是否泄露令牌失败：%v", err)
	}
	if leaked != 0 {
		t.Fatalf("入站正文里出现了绑定令牌明文（%d 条）", leaked)
	}
}

// TestBindingAttemptsRateLimited 锁定 R21：同一用户 10 分钟内最多 5 次失败尝试。
// 超限后必须直接回固定文案，不再调用 ConsumeBindingToken——用「有效令牌未被消费」证明。
func TestBindingAttemptsRateLimited(t *testing.T) {
	w := channelManagerTestSite(t)
	ch := newManagerTestChannel(t, w, true)
	admin := newTestAdmin(t, w, "ratelimit")

	clock := newFakeChannelClock()
	registry := newFakeChannelFactory(nil)
	opts := testChannelManagerOptions(registry)
	opts.now = clock.Now
	startTestChannelManager(t, w, opts)

	adapter := registry.waitForRun(t, ch.AppId, 1)
	sink := adapter.waitSink(t)

	userId := fmt.Sprintf("limit-user-%d", time.Now().UnixNano())
	invalidToken := base64.RawURLEncoding.EncodeToString(bytes.Repeat([]byte{0xAB}, 32))

	for i := 0; i < 5; i++ {
		ev := bindingTestEvent(ch, userId, invalidToken, i)
		if err := sink(context.Background(), ev); err != nil {
			t.Fatalf("第 %d 次失败尝试返回错误：%v", i+1, err)
		}
	}
	calls := adapter.waitSends(t, 5)
	if calls[0].msg.Text != "绑定令牌无效或已过期" {
		t.Fatalf("失败回执为 %q，应为静态文案 %q", calls[0].msg.Text, "绑定令牌无效或已过期")
	}

	// 第 6 次用有效令牌：限速生效时不得消费令牌。
	plain, _, err := w.CreateBindingToken(ch.Id, admin.Id)
	if err != nil {
		t.Fatalf("创建有效绑定令牌失败：%v", err)
	}
	ev := bindingTestEvent(ch, userId, plain, 5)
	if err := sink(context.Background(), ev); err != nil {
		t.Fatalf("超限尝试返回错误：%v", err)
	}
	calls = adapter.waitSends(t, 6)
	if calls[5].msg.Text != "尝试过于频繁，请稍后再试" {
		t.Fatalf("超限回执为 %q，应为 %q", calls[5].msg.Text, "尝试过于频繁，请稍后再试")
	}

	var token model.ChannelBindingToken
	if err := w.DB.Where("token_hash = ?", hashBindingToken(plain)).Take(&token).Error; err != nil {
		t.Fatalf("读取有效令牌失败：%v", err)
	}
	if token.ConsumedAt != 0 {
		t.Fatal("超限后仍然调用了 ConsumeBindingToken：有效令牌被消费")
	}
	var bindingCount int64
	if err := w.DB.Model(&model.ChannelBinding{}).Where("channel_id = ? AND platform_user_id = ?", ch.Id, userId).Count(&bindingCount).Error; err != nil {
		t.Fatalf("统计绑定失败：%v", err)
	}
	if bindingCount != 0 {
		t.Fatalf("超限后仍创建了 %d 条绑定", bindingCount)
	}

	// 时间窗过去后允许重试：成功不计数。
	clock.Advance(11 * time.Minute)
	ev = bindingTestEvent(ch, userId, plain, 6)
	if err := sink(context.Background(), ev); err != nil {
		t.Fatalf("窗口过后的重试返回错误：%v", err)
	}
	calls = adapter.waitSends(t, 7)
	if calls[6].msg.Text != "绑定成功" {
		t.Fatalf("窗口过后的回执为 %q，应为 %q", calls[6].msg.Text, "绑定成功")
	}
}

// TestTokenLikeEventWithoutEventIdIsDropped 锁定 R22：EventId 为空的事件直接丢弃，
// 即使正文长得像绑定 token 也不得进入绑定路径（不消费、不回复、不落库）。
func TestTokenLikeEventWithoutEventIdIsDropped(t *testing.T) {
	w := channelManagerTestSite(t)
	ch := newManagerTestChannel(t, w, true)
	admin := newTestAdmin(t, w, "noeventid")

	plain, _, err := w.CreateBindingToken(ch.Id, admin.Id)
	if err != nil {
		t.Fatalf("创建绑定令牌失败：%v", err)
	}

	registry := newFakeChannelFactory(nil)
	startTestChannelManager(t, w, testChannelManagerOptions(registry))
	adapter := registry.waitForRun(t, ch.AppId, 1)
	sink := adapter.waitSink(t)

	userId := fmt.Sprintf("no-event-user-%d", time.Now().UnixNano())
	ev := bindingTestEvent(ch, userId, plain, 0)
	ev.EventId = ""
	if err := sink(context.Background(), ev); err != nil {
		t.Fatalf("空事件标识处理返回错误：%v", err)
	}
	time.Sleep(100 * time.Millisecond)

	var token model.ChannelBindingToken
	if err := w.DB.Where("token_hash = ?", hashBindingToken(plain)).Take(&token).Error; err != nil {
		t.Fatalf("读取绑定令牌失败：%v", err)
	}
	if token.ConsumedAt != 0 {
		t.Fatal("空事件标识的消息仍进入了绑定路径：令牌被消费")
	}
	var bindingCount int64
	if err := w.DB.Model(&model.ChannelBinding{}).Where("channel_id = ?", ch.Id).Count(&bindingCount).Error; err != nil {
		t.Fatalf("统计绑定失败：%v", err)
	}
	if bindingCount != 0 {
		t.Fatalf("空事件标识的消息创建了 %d 条绑定", bindingCount)
	}
	var inboxCount int64
	if err := w.DB.Model(&model.ChannelInbox{}).Where("channel_id = ?", ch.Id).Count(&inboxCount).Error; err != nil {
		t.Fatalf("统计入站任务失败：%v", err)
	}
	if inboxCount != 0 {
		t.Fatalf("空事件标识的消息被落库（%d 条），空摘要会互相撞唯一键", inboxCount)
	}
	if got := adapter.sendCount(); got != 0 {
		t.Fatalf("空事件标识的消息触发了 %d 次回复", got)
	}
}

// TestChannelLeaseHashNotSerialized 锁定全局约束：后台响应不得包含应用身份摘要。
func TestChannelLeaseHashNotSerialized(t *testing.T) {
	lease := model.NewChannelLease(1,
		&model.MessageChannel{Platform: model.ChannelPlatformFeishu, AppId: "serialize_app", OrgId: "serialize_org"},
		"owner-serialize", 1700000000)
	if lease.AppIdentityHash == "" {
		t.Fatal("租约摘要为空，序列化断言会空转")
	}
	raw, err := json.Marshal(lease)
	if err != nil {
		t.Fatalf("序列化租约失败：%v", err)
	}
	text := string(raw)
	if strings.Contains(text, "app_identity_hash") {
		t.Fatalf("租约 JSON 包含 app_identity_hash：%s", text)
	}
	if strings.Contains(text, lease.AppIdentityHash) {
		t.Fatalf("租约 JSON 包含摘要值：%s", text)
	}
	// 反向对照：不是整个结构体被丢空，其它字段仍应可读。
	for _, expected := range []string{`"site_id":1`, `"owner":"owner-serialize"`, `"expires_at":1700000000`} {
		if !strings.Contains(text, expected) {
			t.Fatalf("租约 JSON 缺少 %s：%s", expected, text)
		}
	}
}

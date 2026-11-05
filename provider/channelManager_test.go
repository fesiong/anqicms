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

// waitAdapterCount 等工厂为同一 AppId 构造出第 want 个适配器。
// 用适配器数量而不是 Run 次数衡量「重试尝试」：抢租约失败时也会构造适配器但不 Run。
func waitAdapterCount(t *testing.T, factory *fakeChannelFactory, appId string, want int, timeout time.Duration) {
	t.Helper()
	deadline := time.Now().Add(timeout)
	for time.Now().Before(deadline) {
		if factory.adapterCount(appId) >= want {
			return
		}
		time.Sleep(5 * time.Millisecond)
	}
	t.Fatalf("等待渠道 %s 的第 %d 次建连尝试超时（当前 %d 次）", appId, want, factory.adapterCount(appId))
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

	// 终态：配置没变就不立即重连。失去租约是可重试退出，但必须等退避窗口
	// （默认 1s，本测试 150ms 内远未到期），所以这里 Run 仍应是 1 次。
	time.Sleep(150 * time.Millisecond)
	if got := registry.runCount(ch.AppId); got != 1 {
		t.Fatalf("退避窗口内就自动重连了：Run 调用 %d 次，应为 1", got)
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
		// 群聊必须显式 @ 机器人才会落 Inbox（§7 第一层门禁）。
		MentionedBot: true,
		Text:         "  你好，测试  ",
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
	// 上界放宽到「等待上限 + 1s」：CI 抖动下只多等一个量级的余量，下界不放。
	if maxElapsed := opts.shutdownWait + time.Second; elapsed > maxElapsed {
		t.Fatalf("Stop 没有在等待上限附近返回（%s，上限 %s）", elapsed, maxElapsed)
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

// ============================================================
// 审查修复（租约重试、停止超时、站点生命周期、群聊门禁）
// ============================================================

// TestChannelLeaseRetriedAfterOpponentReleases 锁定「没拿到租约不是永久占位」：
// 对手释放后，本实例必须按退避自动重试并最终 Run；退避未到期时不得狂查库。
func TestChannelLeaseRetriedAfterOpponentReleases(t *testing.T) {
	w := channelManagerTestSite(t)
	ch := newManagerTestChannel(t, w, true)
	hash := channelAppHash(ch)

	clock := newFakeChannelClock()
	opponent := fmt.Sprintf("opponent-%d", time.Now().UnixNano())
	opponentLease := model.NewChannelLease(w.Id, ch, opponent, clock.Now().Add(5*time.Minute).Unix())
	opponentLease.FencingVersion = 1
	if err := w.DB.Create(opponentLease).Error; err != nil {
		t.Fatalf("创建对手租约失败：%v", err)
	}

	registry := newFakeChannelFactory(nil)
	opts := testChannelManagerOptions(registry)
	opts.now = clock.Now
	opts.retryBackoffBase = time.Second
	opts.retryBackoffMax = 4 * time.Second
	startTestChannelManager(t, w, opts)

	// 首轮：构造了适配器但没拿到租约，Run 不执行。
	waitAdapterCount(t, registry, ch.AppId, 1, 3*time.Second)
	time.Sleep(100 * time.Millisecond)
	if got := registry.runCount(ch.AppId); got != 0 {
		t.Fatalf("没拿到租约却调用了 Run %d 次", got)
	}

	if err := w.DB.Where("app_identity_hash = ?", hash).Delete(&model.ChannelLease{}).Error; err != nil {
		t.Fatalf("删除对手租约失败：%v", err)
	}
	// 假时钟没动 → 退避未到期 → 不得立刻重试（否则每轮扫描都在查库）。
	time.Sleep(150 * time.Millisecond)
	if got := registry.adapterCount(ch.AppId); got != 1 {
		t.Fatalf("退避未到期就重试了：建连尝试 %d 次，应为 1", got)
	}

	// 退避到期：下一轮扫描重新抢租约并真正 Run。
	// 注意第一次尝试只构造了适配器没有 Run，所以第二次尝试对应 adapters[1]。
	clock.Advance(1100 * time.Millisecond)
	waitAdapterCount(t, registry, ch.AppId, 2, 3*time.Second)
	registry.waitForRun(t, ch.AppId, 2)
	if got := registry.runCount(ch.AppId); got != 1 {
		t.Fatalf("重试后 Run 调用 %d 次，应为 1", got)
	}

	var lease model.ChannelLease
	if err := w.DB.Where("app_identity_hash = ?", hash).Take(&lease).Error; err != nil {
		t.Fatalf("重试后没有重新写入租约：%v", err)
	}
	if lease.Owner == opponent || strings.TrimSpace(lease.Owner) == "" {
		t.Fatalf("重试后的租约 owner 为 %q，应为本实例", lease.Owner)
	}
}

// TestLostLeaseRetriesWithBackoff 锁定「失去租约后按指数退避重试」：
// 重试必须重新抢租约（抢不到不 Run），且退避逐次翻倍，不能每轮扫描狂试。
func TestLostLeaseRetriesWithBackoff(t *testing.T) {
	w := channelManagerTestSite(t)
	ch := newManagerTestChannel(t, w, true)

	clock := newFakeChannelClock()
	registry := newFakeChannelFactory(nil)
	opts := testChannelManagerOptions(registry)
	opts.now = clock.Now
	opts.retryBackoffBase = time.Second
	opts.retryBackoffMax = 8 * time.Second
	startTestChannelManager(t, w, opts)

	first := registry.waitForRun(t, ch.AppId, 1)

	// 对手接管（未过期）：续租失败 → 取消 Run → 可重试退出。
	intruder := fmt.Sprintf("intruder-%d", time.Now().UnixNano())
	res := w.DB.Model(&model.ChannelLease{}).
		Where("app_identity_hash = ?", channelAppHash(ch)).
		Updates(map[string]interface{}{
			"owner":           intruder,
			"expires_at":      clock.Now().Add(time.Hour).Unix(),
			"fencing_version": gorm.Expr("fencing_version + 1"),
		})
	if res.Error != nil || res.RowsAffected != 1 {
		t.Fatalf("模拟换主失败：err=%v rows=%d", res.Error, res.RowsAffected)
	}
	waitFakeExit(t, first, 3*time.Second)

	// 第一次退避（1s）未到期：不重试。
	time.Sleep(150 * time.Millisecond)
	if got := registry.adapterCount(ch.AppId); got != 1 {
		t.Fatalf("退避未到期就重试了：建连尝试 %d 次", got)
	}

	// 进行第一次重试：对手租约未过期，抢不到，Run 不得再次发生。
	clock.Advance(1100 * time.Millisecond)
	waitAdapterCount(t, registry, ch.AppId, 2, 3*time.Second)
	time.Sleep(100 * time.Millisecond)
	if got := registry.runCount(ch.AppId); got != 1 {
		t.Fatalf("重试时没抢到租约却调用了 Run：%d 次", got)
	}

	// 第二次退避应翻倍到 2s：推进 1.5s 不得重试，再推进 0.7s 才重试。
	clock.Advance(1500 * time.Millisecond)
	time.Sleep(100 * time.Millisecond)
	if got := registry.adapterCount(ch.AppId); got != 2 {
		t.Fatalf("退避没有翻倍：距上次尝试 1.5s 就重试了（建连尝试 %d 次）", got)
	}
	clock.Advance(700 * time.Millisecond)
	waitAdapterCount(t, registry, ch.AppId, 3, 3*time.Second)
	if got := registry.runCount(ch.AppId); got != 1 {
		t.Fatalf("重试时没抢到租约却调用了 Run：%d 次", got)
	}
}

// TestChannelRebuildTimeoutKeepsLeaseAndAvoidsSecondRun 锁定停止超时的 abandoned 语义：
// RebuildChannel 在时限内返回错误；不释放租约、不起第二条连接；
// 旧连接退出后下一轮扫描继续完成重建。
func TestChannelRebuildTimeoutKeepsLeaseAndAvoidsSecondRun(t *testing.T) {
	w := channelManagerTestSite(t)
	ch := newManagerTestChannel(t, w, true)

	registry := newFakeChannelFactory(func(appId string, index int) *fakeChannelAdapter {
		adapter := newFakeChannelAdapter()
		if index == 0 {
			// 模拟钉钉 SDK 不感知 ctx：退出远慢于 shutdownWait。
			adapter.ignoreCancel = true
			adapter.exitDelay = 400 * time.Millisecond
		}
		return adapter
	})
	opts := testChannelManagerOptions(registry)
	opts.shutdownWait = 80 * time.Millisecond
	startTestChannelManager(t, w, opts)

	first := registry.waitForRun(t, ch.AppId, 1)

	startedAt := time.Now()
	err := w.RebuildChannel(ch.Id)
	elapsed := time.Since(startedAt)
	if err == nil {
		t.Fatal("旧连接未在时限内退出时 RebuildChannel 必须返回错误")
	}
	if !strings.Contains(err.Error(), "退出") {
		t.Fatalf("RebuildChannel 错误不可读：%v", err)
	}
	if elapsed < opts.shutdownWait {
		t.Fatalf("RebuildChannel 没等满时限就返回了（%s）", elapsed)
	}
	if limit := opts.shutdownWait + time.Second; elapsed > limit {
		t.Fatalf("RebuildChannel 超出时限（%s > %s）", elapsed, limit)
	}

	// 旧连接还活着：租约不得释放，也不得建立第二条连接。
	var lease model.ChannelLease
	if err := w.DB.Where("app_identity_hash = ?", channelAppHash(ch)).Take(&lease).Error; err != nil {
		t.Fatalf("停止超时后租约被释放了：%v", err)
	}
	select {
	case <-first.exited:
		t.Fatal("忽略取消的适配器在时限内退出了，无法验证超时路径")
	default:
	}
	if got := registry.runCount(ch.AppId); got != 1 {
		t.Fatalf("旧连接未退出就启动了新连接：Run %d 次", got)
	}

	// 旧连接退出后，下一轮扫描继续完成重建：先退出、再 Run，租约保持不变。
	waitFakeExit(t, first, 3*time.Second)
	registry.waitForRun(t, ch.AppId, 2)
	select {
	case <-first.exited:
	default:
		t.Fatal("旧连接没退出就建立了新连接")
	}
	var after model.ChannelLease
	if err := w.DB.Where("app_identity_hash = ?", channelAppHash(ch)).Take(&after).Error; err != nil {
		t.Fatalf("重建后租约消失了：%v", err)
	}
	if after.Owner != lease.Owner || after.FencingVersion != lease.FencingVersion {
		t.Fatalf("超时重建改了租约归属：owner %q→%q fencing %d→%d",
			lease.Owner, after.Owner, lease.FencingVersion, after.FencingVersion)
	}
}

// TestChannelLifecycleStopsOnRemoveWebsite 锁定站点删除路径：先停管理器再从注册表摘除，
// 正常退出的连接按规则释放租约（避免用过期租约挡住别人接管）。
func TestChannelLifecycleStopsOnRemoveWebsite(t *testing.T) {
	w := channelManagerTestSite(t)

	fakeId := uint(900000) + uint(time.Now().UnixNano()%100000)
	fakeSite := &Website{Id: fakeId, Mysql: w.Mysql, DB: w.DB, Initialed: true}
	websites.Set(fakeSite)
	t.Cleanup(func() {
		fakeSite.StopChannelManager()
		websites.Delete(fakeId)
	})

	ch := newManagerTestChannel(t, fakeSite, true)
	registry := newFakeChannelFactory(nil)
	fakeSite.startChannelManager(context.Background(), testChannelManagerOptions(registry))
	adapter := registry.waitForRun(t, ch.AppId, 1)

	RemoveWebsite(fakeId, false)

	waitFakeExit(t, adapter, 3*time.Second)
	if _, ok := websites.Get(fakeId); ok {
		t.Fatal("RemoveWebsite 后站点仍在注册表里")
	}
	if manager := channelManagerRegistry.bySite[fakeId]; manager != nil {
		t.Fatal("RemoveWebsite 后渠道管理器仍在注册表里")
	}
	var lease model.ChannelLease
	err := w.DB.Where("app_identity_hash = ?", channelAppHash(ch)).Take(&lease).Error
	if err == nil {
		t.Fatal("站点删除后正常退出的连接没有释放租约")
	}
	if !errors.Is(err, gorm.ErrRecordNotFound) {
		t.Fatalf("读取租约失败：%v", err)
	}
}

// TestChannelLifecycleRebuildsOnSiteReinit 锁定 InitWebsite 重入：
// 同一 site_id 换了新的 *Website / DB 句柄时，旧管理器必须先停、旧连接先退出，
// 再换上新管理器，不能留着绑旧句柄的管理器继续续租重连。
func TestChannelLifecycleRebuildsOnSiteReinit(t *testing.T) {
	w := channelManagerTestSite(t)
	ch := newManagerTestChannel(t, w, true)

	registry := newFakeChannelFactory(nil)
	startTestChannelManager(t, w, testChannelManagerOptions(registry))
	first := registry.waitForRun(t, ch.AppId, 1)

	// 等价于 InitWebsite 重入：新的 Website 指针 + 新的 DB 句柄。
	replacement := &Website{
		Id:        w.Id,
		Mysql:     w.Mysql,
		DB:        w.DB.Session(&gorm.Session{}),
		Initialed: true,
	}
	t.Cleanup(func() { replacement.StopChannelManager() })
	replacement.startChannelManager(context.Background(), testChannelManagerOptions(registry))

	manager := channelManagerRegistry.bySite[w.Id]
	if manager == nil {
		t.Fatal("同 site_id 重启后注册表里没有管理器")
	}
	if manager.site != replacement {
		t.Fatal("注册表里仍绑着旧的 *Website，管理器没有被重建")
	}
	select {
	case <-first.exited:
	default:
		t.Fatal("旧管理器没有停止旧连接")
	}
	registry.waitForRun(t, ch.AppId, 2)
}

// TestChannelLifecycleInitSwitch 锁定显式开关：InitWebsite 默认不启动渠道管理器
// （否则所有既有 provider 测试都会用生产参数真实建连），打开后才启动。
func TestChannelLifecycleInitSwitch(t *testing.T) {
	w := channelManagerTestSite(t)

	// 保护：开发库里若真有启用渠道，打开开关会让本测试用生产参数真实建连。
	var enabledCount int64
	if err := w.DB.Model(&model.MessageChannel{}).Where("site_id = ? AND enabled = ?", w.Id, 1).Count(&enabledCount).Error; err != nil {
		t.Fatalf("统计启用渠道失败：%v", err)
	}
	if enabledCount > 0 {
		t.Skipf("站点 %d 存在 %d 个启用渠道，跳过以免真实建连", w.Id, enabledCount)
	}

	initSite := func() {
		dbSite, err := GetDBWebsiteInfo(1)
		if err != nil {
			t.Fatalf("读取站点失败：%v", err)
		}
		dbSite.Status = 1
		InitWebsite(dbSite)
	}

	// 开关关闭（默认）：再次 InitWebsite 也不得启动管理器。
	// 关键在于 fixture 已经停过一次，这里断言的是「InitWebsite 自己没启动」。
	initSite()
	if manager := channelManagerRegistry.bySite[w.Id]; manager != nil {
		t.Fatal("开关关闭时 InitWebsite 启动了渠道管理器")
	}

	previous := channelManagerOnInit.Load()
	channelManagerOnInit.Store(true)
	t.Cleanup(func() {
		channelManagerOnInit.Store(previous)
		w.StopChannelManager()
	})

	initSite()
	if manager := channelManagerRegistry.bySite[w.Id]; manager == nil {
		t.Fatal("开关打开后 InitWebsite 应启动渠道管理器")
	}
}

// TestChannelLifecycleGroupMessageGate 锁定规格 §7 第一层门禁：
// 群聊未 @ 机器人的消息直接丢弃、不落 Inbox、不回复；@ 了的照常落库。
func TestChannelLifecycleGroupMessageGate(t *testing.T) {
	w := channelManagerTestSite(t)
	ch := newManagerTestChannel(t, w, true)

	registry := newFakeChannelFactory(nil)
	startTestChannelManager(t, w, testChannelManagerOptions(registry))
	adapter := registry.waitForRun(t, ch.AppId, 1)
	sink := adapter.waitSink(t)

	unmentioned := InboundEvent{
		EventId:        fmt.Sprintf("group-quiet-%d", time.Now().UnixNano()),
		Kind:           InboundKindText,
		OrganizationId: ch.OrgId,
		AppId:          ch.AppId,
		UserId:         "group-user",
		ChatId:         "group-chat",
		ChatType:       ChatTypeGroup,
		MentionedBot:   false,
		Text:           "大家好，今天下午开会",
	}
	if err := sink(context.Background(), unmentioned); err != nil {
		t.Fatalf("群聊未 @ 的消息处理返回错误：%v", err)
	}
	time.Sleep(50 * time.Millisecond)

	var quietCount int64
	if err := w.DB.Model(&model.ChannelInbox{}).
		Where("event_hash = ?", wantEventDigest(w.Id, ch.Id, ch.Platform, unmentioned.EventId)).
		Count(&quietCount).Error; err != nil {
		t.Fatalf("统计入站任务失败：%v", err)
	}
	if quietCount != 0 {
		t.Fatalf("群聊未 @ 机器人的消息落库了（%d 条）", quietCount)
	}
	if got := adapter.sendCount(); got != 0 {
		t.Fatalf("群聊未 @ 机器人的消息触发了 %d 次回复", got)
	}

	// 反向对照：@ 了机器人就必须落库，证明上面的 0 不是整条链路空转。
	mentioned := unmentioned
	mentioned.EventId = unmentioned.EventId + "-at"
	mentioned.MentionedBot = true
	mentioned.Text = "请帮我看看今天的订单"
	if err := sink(context.Background(), mentioned); err != nil {
		t.Fatalf("@ 机器人的消息处理返回错误：%v", err)
	}
	var atCount int64
	if err := w.DB.Model(&model.ChannelInbox{}).
		Where("event_hash = ?", wantEventDigest(w.Id, ch.Id, ch.Platform, mentioned.EventId)).
		Count(&atCount).Error; err != nil {
		t.Fatalf("统计入站任务失败：%v", err)
	}
	if atCount != 1 {
		t.Fatalf("@ 机器人的群聊消息落库 %d 条，应为 1", atCount)
	}
}

// TestChannelLifecycleInboxEventHashNotSerialized 锁定全局约束：后台响应不得包含事件摘要。
func TestChannelLifecycleInboxEventHashNotSerialized(t *testing.T) {
	inbox := &model.ChannelInbox{
		SiteId:    1,
		ChannelId: 2,
		Content:   "你好",
		Status:    model.InboxStatusPending,
		EventHash: strings.Repeat("ab", 32),
	}
	raw, err := json.Marshal(inbox)
	if err != nil {
		t.Fatalf("序列化入站任务失败：%v", err)
	}
	text := string(raw)
	if strings.Contains(text, "event_hash") {
		t.Fatalf("入站任务 JSON 包含 event_hash：%s", text)
	}
	if strings.Contains(text, inbox.EventHash) {
		t.Fatalf("入站任务 JSON 包含事件摘要值：%s", text)
	}
	// 反向对照：不是整个结构体被丢空。
	for _, expected := range []string{`"site_id":1`, `"channel_id":2`, `"status":"pending"`} {
		if !strings.Contains(text, expected) {
			t.Fatalf("入站任务 JSON 缺少 %s：%s", expected, text)
		}
	}
}

package provider

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"
	"sync"
	"testing"
	"time"

	"kandaoni.com/anqicms/model"
)

// ============================================================
// 假适配器：只替代平台网络，不 mock 业务路径。
// 每次 Send 按序返回脚本里的结果（用完之后一律成功），并记录目标与文本。
// ============================================================

type notifyFakeResponse struct {
	result SendResult
	err    error
}

type notifyFakeCall struct {
	target TargetRef
	text   string
}

type notifyFakeAdapter struct {
	mu        sync.Mutex
	responses []notifyFakeResponse
	calls     []notifyFakeCall
}

func newNotifyFakeAdapter(responses ...notifyFakeResponse) *notifyFakeAdapter {
	return &notifyFakeAdapter{responses: responses}
}

func (a *notifyFakeAdapter) Name() string { return "notify-fake" }

func (a *notifyFakeAdapter) Run(ctx context.Context, creds PlatformCredentials, sink InboundSink) error {
	<-ctx.Done()
	return nil
}

func (a *notifyFakeAdapter) Send(ctx context.Context, target TargetRef, msg OutboundMessage) (SendResult, error) {
	a.mu.Lock()
	defer a.mu.Unlock()
	index := len(a.calls)
	a.calls = append(a.calls, notifyFakeCall{target: target, text: msg.Text})
	if index < len(a.responses) {
		return a.responses[index].result, a.responses[index].err
	}
	return SendResult{PlatformMessageId: fmt.Sprintf("notify-fake-%d", index+1)}, nil
}

func (a *notifyFakeAdapter) callCount() int {
	a.mu.Lock()
	defer a.mu.Unlock()
	return len(a.calls)
}

func (a *notifyFakeAdapter) callsFor(platformTargetId string) int {
	a.mu.Lock()
	defer a.mu.Unlock()
	count := 0
	for _, call := range a.calls {
		if call.target.PlatformTargetId == platformTargetId {
			count++
		}
	}
	return count
}

// notifyTestOptions 返回直接驱动 runNotifyCycle 的测试参数。
// newAdapter 永远返回同一个假适配器：用例只关心发送结果，不关心渠道实例化。
func notifyTestOptions(adapter PlatformAdapter, now func() time.Time) notifyWorkerOptions {
	if now == nil {
		now = time.Now
	}
	return notifyWorkerOptions{
		newAdapter: func(string, PlatformCredentials) (PlatformAdapter, bool) {
			return adapter, true
		},
		now:         now,
		interval:    time.Hour,
		leaseTTL:    30 * time.Second,
		sendTimeout: 15 * time.Second,
		batch:       20,
	}
}

func notifyTestRunCycle(t *testing.T, w *Website, opts notifyWorkerOptions) {
	t.Helper()
	w.runNotifyCycle(context.Background(), opts)
}

func notifyTestDeliveryForTarget(t *testing.T, w *Website, eventId, ruleId, targetId uint) *model.NotificationDelivery {
	t.Helper()
	var delivery model.NotificationDelivery
	err := w.DB.Where(
		"site_id = ? AND event_id = ? AND rule_id = ? AND target_id = ?",
		w.Id, eventId, ruleId, targetId,
	).Take(&delivery).Error
	if err != nil {
		t.Fatalf("读取目标 %d 的投递失败：%v", targetId, err)
	}
	return &delivery
}

// notifyTestBackoff 是第 1..5 次失败后的退避（第 6 次直接 failed）。
var notifyTestBackoff = []time.Duration{
	10 * time.Second,
	30 * time.Second,
	2 * time.Minute,
	10 * time.Minute,
	30 * time.Minute,
}

// ============================================================
// 7. transient 重试、permanent 不重试、第 6 次失败
// ============================================================

func TestDeliveryRetriesTransientNotPermanent(t *testing.T) {
	w := notifyTestSite(t)
	ch := newManagerTestChannel(t, w, true)
	clock := newFakeChannelClock()
	// 投递在扇出时用真实时间写 next_attempt_at；把注入时钟拨快到其后，
	// 避免整秒边界导致「下一次尝试」尚未到期。
	clock.Advance(time.Minute)

	// 7a. transient 一次后成功：先 retry（+10s），推进时钟后 sent，平台调用 2 次。
	transientTarget := newNotifyTestGroupTarget(t, w, ch.Id, 1)
	transientRule := newNotifyTestRule(t, w, NotifyEventOrderCreated, 0, transientTarget.Id)
	transientEvent := notifyTestEmitReady(t, w, NotifyEventOrderCreated, "retry-"+notifyTestSuffix(), map[string]any{
		"order_id": "retry-order",
		"amount":   int64(1234),
		"status":   "pending",
	})
	transientAdapter := newNotifyFakeAdapter(
		notifyFakeResponse{err: NewSendError(ErrorClassTransient, "服务端限流")},
	)
	opts := notifyTestOptions(transientAdapter, clock.Now)

	notifyTestRunCycle(t, w, opts)
	delivery := notifyTestDeliveryForTarget(t, w, transientEvent.Id, transientRule.Id, transientTarget.Id)
	if delivery.Status != NotifyDeliveryStatusRetry {
		t.Fatalf("transient 失败后投递应为 retry，实际 %q", delivery.Status)
	}
	wantNext := clock.Now().Unix() + int64(notifyTestBackoff[0]/time.Second)
	if delta := delivery.NextAttemptAt - wantNext; delta < -1 || delta > 1 {
		t.Fatalf("transient 失败后下次尝试应约为 now+10s，期望 %d，实际 %d", wantNext, delivery.NextAttemptAt)
	}
	if got := transientAdapter.callsFor(transientTarget.PlatformTargetId); got != 1 {
		t.Fatalf("第一次失败后平台调用应为 1 次，实际 %d 次", got)
	}

	clock.Advance(notifyTestBackoff[0] + time.Second)
	notifyTestRunCycle(t, w, opts)
	delivery = notifyTestDeliveryForTarget(t, w, transientEvent.Id, transientRule.Id, transientTarget.Id)
	if delivery.Status != NotifyDeliveryStatusSent {
		t.Fatalf("重试成功后投递应为 sent，实际 %q（LastError=%q）", delivery.Status, delivery.LastError)
	}
	if delivery.PlatformMessageId == "" {
		t.Fatal("发送成功后必须记录平台消息 ID")
	}
	if delivery.LastError != "" {
		t.Fatalf("发送成功后必须清空 LastError，实际 %q", delivery.LastError)
	}
	if got := transientAdapter.callsFor(transientTarget.PlatformTargetId); got != 2 {
		t.Fatalf("transient 一次后成功应调用平台 2 次，实际 %d 次", got)
	}

	// 7b. permanent：立即 failed，只调用 1 次。
	permanentTarget := newNotifyTestGroupTarget(t, w, ch.Id, 1)
	permanentRule := newNotifyTestRule(t, w, NotifyEventOrderPaid, 0, permanentTarget.Id)
	permanentEvent := notifyTestEmitReady(t, w, NotifyEventOrderPaid, "permanent-"+notifyTestSuffix(), map[string]any{
		"order_id": "permanent-order",
		"status":   "paid",
	})
	permanentAdapter := newNotifyFakeAdapter(
		notifyFakeResponse{err: NewSendError(ErrorClassPermanent, "目标不存在")},
	)
	permanentOpts := notifyTestOptions(permanentAdapter, clock.Now)

	notifyTestRunCycle(t, w, permanentOpts)
	delivery = notifyTestDeliveryForTarget(t, w, permanentEvent.Id, permanentRule.Id, permanentTarget.Id)
	if delivery.Status != NotifyDeliveryStatusFailed {
		t.Fatalf("permanent 失败后投递应为 failed，实际 %q", delivery.Status)
	}
	if delivery.LastError == "" {
		t.Fatal("permanent 失败必须记录 LastError")
	}
	if got := permanentAdapter.callsFor(permanentTarget.PlatformTargetId); got != 1 {
		t.Fatalf("permanent 失败不得重试，平台调用应为 1 次，实际 %d 次", got)
	}
	clock.Advance(time.Hour)
	notifyTestRunCycle(t, w, permanentOpts)
	if got := permanentAdapter.callsFor(permanentTarget.PlatformTargetId); got != 1 {
		t.Fatalf("permanent 失败后再次跑轮不得再调用平台，实际 %d 次", got)
	}

	// 7c. 连续 transient：退避 10s/30s/2m/10m/30m，第 6 次 failed。
	exhaustTarget := newNotifyTestGroupTarget(t, w, ch.Id, 1)
	exhaustRule := newNotifyTestRule(t, w, NotifyEventGuestbookCreated, 0, exhaustTarget.Id)
	exhaustEvent := notifyTestEmitReady(t, w, NotifyEventGuestbookCreated, "exhaust-"+notifyTestSuffix(), map[string]any{
		"guestbook_id": 1,
		"user_name":    "重试测试",
		"content":      "重试测试正文",
	})
	exhaustAdapter := newNotifyFakeAdapter()
	for i := 0; i < 6; i++ {
		exhaustAdapter.responses = append(exhaustAdapter.responses, notifyFakeResponse{
			err: NewSendError(ErrorClassTransient, "服务端 503"),
		})
	}
	exhaustOpts := notifyTestOptions(exhaustAdapter, clock.Now)

	for attempt := 0; attempt < 6; attempt++ {
		notifyTestRunCycle(t, w, exhaustOpts)
		delivery = notifyTestDeliveryForTarget(t, w, exhaustEvent.Id, exhaustRule.Id, exhaustTarget.Id)
		if delivery.Attempts != attempt+1 {
			t.Fatalf("第 %d 次尝试后 attempts 应为 %d，实际 %d", attempt+1, attempt+1, delivery.Attempts)
		}
		if attempt < 5 {
			if delivery.Status != NotifyDeliveryStatusRetry {
				t.Fatalf("第 %d 次 transient 失败后应为 retry，实际 %q", attempt+1, delivery.Status)
			}
			want := clock.Now().Unix() + int64(notifyTestBackoff[attempt]/time.Second)
			if delta := delivery.NextAttemptAt - want; delta < -1 || delta > 1 {
				t.Fatalf("第 %d 次失败后的退避不符：期望 %d，实际 %d", attempt+1, want, delivery.NextAttemptAt)
			}
			clock.Advance(notifyTestBackoff[attempt] + time.Second)
			continue
		}
		if delivery.Status != NotifyDeliveryStatusFailed {
			t.Fatalf("第 6 次 transient 失败后应为 failed，实际 %q", delivery.Status)
		}
	}
	if got := exhaustAdapter.callsFor(exhaustTarget.PlatformTargetId); got != 6 {
		t.Fatalf("连续 transient 应调用平台 6 次，实际 %d 次", got)
	}
}

// ============================================================
// 8. 同一 (event, rule, target) 只允许一行
// ============================================================

func TestDeliveryIsIdempotentPerTarget(t *testing.T) {
	w := notifyTestSite(t)
	ch := newManagerTestChannel(t, w, true)
	targetA := newNotifyTestGroupTarget(t, w, ch.Id, 1)
	targetB := newNotifyTestGroupTarget(t, w, ch.Id, 1)
	rule := newNotifyTestRule(t, w, NotifyEventOrderCreated, 0, targetA.Id, targetB.Id)
	event := notifyTestEmitReady(t, w, NotifyEventOrderCreated, "idempotent-"+notifyTestSuffix(), map[string]any{
		"order_id": "idempotent-order",
		"status":   "pending",
	})

	if got := len(notifyTestDeliveries(t, w, event.Id, rule.Id)); got != 2 {
		t.Fatalf("两个目标应产生 2 条投递，实际 %d 条", got)
	}

	// 同一事件再扇出一次：uk_delivery_hash 必须把重复行顶掉。
	tx := w.DB.Begin()
	if tx.Error != nil {
		t.Fatalf("开启事务失败：%v", tx.Error)
	}
	if err := w.fanOutNotificationEvent(tx, event); err != nil {
		tx.Rollback()
		t.Fatalf("重复扇出不应报错：%v", err)
	}
	if err := tx.Commit().Error; err != nil {
		t.Fatalf("提交重复扇出失败：%v", err)
	}

	deliveries := notifyTestDeliveries(t, w, event.Id, rule.Id)
	if len(deliveries) != 2 {
		t.Fatalf("重复扇出不得增加投递，实际 %d 条", len(deliveries))
	}
	seen := map[uint]int{}
	for _, delivery := range deliveries {
		seen[delivery.TargetId]++
	}
	if seen[targetA.Id] != 1 || seen[targetB.Id] != 1 {
		t.Fatalf("每个目标应恰好 1 条投递，实际 %v", seen)
	}
}

// ============================================================
// 9. 目标失效：取消投递，不发平台请求
// ============================================================

func TestRevokedTargetCancelsPendingDelivery(t *testing.T) {
	w := notifyTestSite(t)
	ch := newManagerTestChannel(t, w, true)
	admin := newPermTestAdmin(t, w, nil, 1)
	binding := newPermTestBinding(t, w, ch.Id, admin.Id)
	target := newNotifyTestPrivateTarget(t, w, ch.Id, binding.Id, 1)
	rule := newNotifyTestRule(t, w, NotifyEventOrderPaid, 0, target.Id)
	event := notifyTestEmitReady(t, w, NotifyEventOrderPaid, "revoke-"+notifyTestSuffix(), map[string]any{
		"order_id": "revoke-order",
		"status":   "paid",
	})

	delivery := notifyTestDeliveryForTarget(t, w, event.Id, rule.Id, target.Id)
	if delivery.Status != NotifyDeliveryStatusPending {
		t.Fatalf("扇出后投递应为 pending，实际 %q", delivery.Status)
	}

	// 撤销绑定：私聊目标不再可用。
	if err := w.DB.Model(&model.ChannelBinding{}).
		Where("id = ? AND site_id = ?", binding.Id, w.Id).
		Update("revoked_at", time.Now().Unix()).Error; err != nil {
		t.Fatalf("撤销绑定失败：%v", err)
	}

	adapter := newNotifyFakeAdapter()
	notifyTestRunCycle(t, w, notifyTestOptions(adapter, nil))

	delivery = notifyTestDeliveryForTarget(t, w, event.Id, rule.Id, target.Id)
	if delivery.Status != NotifyDeliveryStatusCancelled {
		t.Fatalf("绑定撤销后投递应为 cancelled，实际 %q", delivery.Status)
	}
	if got := adapter.callsFor(target.PlatformTargetId); got != 0 {
		t.Fatalf("目标失效时不得发送平台请求，实际调用 %d 次", got)
	}
}

// ============================================================
// 10. 投递失败不得反向外发 agent.failed
// ============================================================

func TestFailedDeliveryDoesNotEmitAgentFailedRecursively(t *testing.T) {
	w := notifyTestSite(t)
	ch := newManagerTestChannel(t, w, true)
	target := newNotifyTestGroupTarget(t, w, ch.Id, 1)
	rule := newNotifyTestRule(t, w, NotifyEventGuestbookCreated, 0, target.Id)

	// 先空跑一轮，让工作者的「按审计派生 agent.failed」的存量部分收敛，
	// 避免它与本用例要统计的投递失败混淆。
	warmup := newNotifyFakeAdapter()
	notifyTestRunCycle(t, w, notifyTestOptions(warmup, nil))

	var baseline int64
	if err := w.DB.Model(&model.NotificationEvent{}).
		Where("site_id = ? AND event_type = ?", w.Id, NotifyEventAgentFailed).
		Count(&baseline).Error; err != nil {
		t.Fatalf("统计 agent.failed 基线失败：%v", err)
	}

	event := notifyTestEmitReady(t, w, NotifyEventGuestbookCreated, "recursive-"+notifyTestSuffix(), map[string]any{
		"guestbook_id": 2,
		"user_name":    "递归测试",
		"content":      "递归测试正文",
	})
	adapter := newNotifyFakeAdapter(
		notifyFakeResponse{err: NewSendError(ErrorClassPermanent, "目标不存在")},
	)
	notifyTestRunCycle(t, w, notifyTestOptions(adapter, nil))

	delivery := notifyTestDeliveryForTarget(t, w, event.Id, rule.Id, target.Id)
	if delivery.Status != NotifyDeliveryStatusFailed {
		t.Fatalf("permanent 失败应让投递进入 failed，实际 %q", delivery.Status)
	}

	var after int64
	if err := w.DB.Model(&model.NotificationEvent{}).
		Where("site_id = ? AND event_type = ?", w.Id, NotifyEventAgentFailed).
		Count(&after).Error; err != nil {
		t.Fatalf("统计 agent.failed 失败：%v", err)
	}
	if after != baseline {
		t.Fatalf("投递失败不得创建 agent.failed 事件，基线 %d，实际 %d", baseline, after)
	}

	notifyTestRunCycle(t, w, notifyTestOptions(adapter, nil))
	var again int64
	if err := w.DB.Model(&model.NotificationEvent{}).
		Where("site_id = ? AND event_type = ?", w.Id, NotifyEventAgentFailed).
		Count(&again).Error; err != nil {
		t.Fatalf("再次统计 agent.failed 失败：%v", err)
	}
	if again != baseline {
		t.Fatalf("后续轮次也不得因投递失败创建 agent.failed，基线 %d，实际 %d", baseline, again)
	}

	// 派生口径：只认最近 10 分钟失败的审计，且重复派生不计数（R75）。
	if err := w.DB.AutoMigrate(&model.ChannelAudit{}); err != nil {
		t.Fatalf("迁移渠道审计表失败：%v", err)
	}
	staleAudit := newNotifyTestFailedAudit(t, w, time.Now().Unix()-3600)
	freshAudit := newNotifyTestFailedAudit(t, w, time.Now().Unix())
	staleKey := fmt.Sprintf("%s:audit:%d", NotifyEventAgentFailed, staleAudit.Id)
	freshKey := fmt.Sprintf("%s:audit:%d", NotifyEventAgentFailed, freshAudit.Id)
	cleanupNotifyBusinessKeys(t, w, staleKey, freshKey)

	w.deriveAgentFailedNotificationEvents()
	if got := notifyTestCountEventsByKey(t, w, staleKey); got != 0 {
		t.Fatalf("1 小时前失败的审计不得派生 agent.failed 事件（时间窗外），实际 %d 条", got)
	}
	if got := notifyTestCountEventsByKey(t, w, freshKey); got != 1 {
		t.Fatalf("最近失败的审计应派生 1 条 agent.failed 事件，实际 %d 条", got)
	}

	// 再派生一轮：唯一键命中属于 no-op 插入，Derived 必须为 0。
	if derived := w.deriveAgentFailedNotificationEvents(); derived != 0 {
		t.Fatalf("已派生过的审计重复扫描不得计入 Derived，实际 %d", derived)
	}
	if got := notifyTestCountEventsByKey(t, w, freshKey); got != 1 {
		t.Fatalf("重复派生不得新增事件，实际 %d 条", got)
	}
}

// newNotifyTestFailedAudit 建一条 result=failed 的审计，created_time 由调用方给定。
// GORM 的 autoCreateTime 只在零值时生效，显式给值即可模拟历史失败。
func newNotifyTestFailedAudit(t *testing.T, w *Website, createdTime int64) *model.ChannelAudit {
	t.Helper()
	audit := &model.ChannelAudit{
		CreatedTime: createdTime,
		SiteId:      w.Id,
		Action:      "notify_test_audit",
		TargetHash:  "notify_test_target_hash",
		Result:      "failed",
	}
	if err := w.DB.Create(audit).Error; err != nil {
		t.Fatalf("创建失败审计失败：%v", err)
	}
	t.Cleanup(func() {
		if err := w.DB.Where("id = ? AND site_id = ?", audit.Id, w.Id).Delete(&model.ChannelAudit{}).Error; err != nil {
			t.Logf("清理失败审计 %d 失败：%v", audit.Id, err)
		}
	})
	return audit
}

// notifyTestCountEventsByKey 按事件键精确统计事件行数（独立算摘要，不复用被测函数）。
func notifyTestCountEventsByKey(t *testing.T, w *Website, eventKey string) int64 {
	t.Helper()
	var count int64
	if err := w.DB.Model(&model.NotificationEvent{}).
		Where("site_id = ? AND event_key_hash = ?", w.Id, notifyTestEventKeyHash(eventKey)).
		Count(&count).Error; err != nil {
		t.Fatalf("统计事件 %s 失败：%v", eventKey, err)
	}
	return count
}

// ============================================================
// 11. 投递时间语义：order.paid 优先 paid_time，其余事件仍以 created_time 为准
// ============================================================

func TestSuccessPaidOrderEmitsPaidTimeInTemplate(t *testing.T) {
	// 订单可能创建很久后才支付；order.paid 显示的时间必须是支付时间。
	createdStamp := int64(1700000000)
	paidStamp := int64(1700007200)
	payload, err := json.Marshal(map[string]any{
		"order_id":     "paid-time-order",
		"status":       "paid",
		"created_time": createdStamp,
		"paid_time":    paidStamp,
	})
	if err != nil {
		t.Fatalf("序列化载荷失败：%v", err)
	}
	tpl := &model.NotificationTemplate{Subject: "时间", Content: "{{ occurred_at }}"}

	_, paidText, err := RenderNotificationText(tpl, "feishu", NotifyEventOrderPaid, string(payload), TargetKindGroup, "站点", "https://example.com")
	if err != nil {
		t.Fatalf("order.paid 模板渲染失败：%v", err)
	}
	wantPaid := time.Unix(paidStamp, 0).Format("2006-01-02 15:04:05")
	if strings.TrimSpace(paidText) != wantPaid {
		t.Fatalf("order.paid 的时间应为支付时间 %q，实际 %q", wantPaid, paidText)
	}

	// 反向：其他事件不得被 paid_time 抢走 created_time（保持原有行为）。
	_, createdText, err := RenderNotificationText(tpl, "feishu", NotifyEventOrderCreated, string(payload), TargetKindGroup, "站点", "https://example.com")
	if err != nil {
		t.Fatalf("order.created 模板渲染失败：%v", err)
	}
	wantCreated := time.Unix(createdStamp, 0).Format("2006-01-02 15:04:05")
	if strings.TrimSpace(createdText) != wantCreated {
		t.Fatalf("order.created 的时间应为下单时间 %q，实际 %q", wantCreated, createdText)
	}

	// 支付时间缺失时回落 created_time，不得渲染成空。
	fallbackPayload, err := json.Marshal(map[string]any{
		"order_id":     "paid-time-fallback",
		"created_time": createdStamp,
	})
	if err != nil {
		t.Fatalf("序列化回退载荷失败：%v", err)
	}
	_, fallbackText, err := RenderNotificationText(tpl, "feishu", NotifyEventOrderPaid, string(fallbackPayload), TargetKindGroup, "站点", "https://example.com")
	if err != nil {
		t.Fatalf("order.paid 回退渲染失败：%v", err)
	}
	if strings.TrimSpace(fallbackText) != wantCreated {
		t.Fatalf("paid_time 缺失时应回落到 created_time %q，实际 %q", wantCreated, fallbackText)
	}
}

package provider

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"strconv"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/flosch/pongo2/v6"

	"kandaoni.com/anqicms/config"
	"kandaoni.com/anqicms/model"
	"kandaoni.com/anqicms/request"
)

// ============================================================
// 夹具：真实库（站点 1）。
//
// 通知四表在当前开发库上尚未建立（AutoMigrate 只在版本变化时跑），
// 与渠道相关任务的处理方式一致：本任务用例按需补建。
// 所有数据带随机后缀，清理一律按自己创建的 ID / 业务键精确删除。
// ============================================================

func notifyTestSite(t *testing.T) *Website {
	t.Helper()
	w := channelManagerTestSite(t)
	notifyTestRegisterEmailTagShim(t)
	// initTestSite 会走生产初始化流程；通知工作者默认关闭（只有进程入口打开开关），
	// 这里再显式停一次，避免用例之间留下后台 goroutine。
	w.StopNotifyWorker()
	if err := w.DB.AutoMigrate(
		&model.NotificationRule{},
		&model.NotificationTemplate{},
		&model.NotificationEvent{},
		&model.NotificationDelivery{},
	); err != nil {
		t.Fatalf("迁移通知规则/模板/事件/投递表失败：%v", err)
	}
	t.Cleanup(func() { w.StopNotifyWorker() })
	return w
}

func notifyTestSuffix() string {
	return fmt.Sprintf("%d", time.Now().UnixNano())
}

// notifyTestRegisterEmailTagShim 在测试二进制里注册一个输出空串的 system 标签。
//
// 生产环境的 bootstrap.Serve 会通过 pongo2.RegisterTag 注册真实标签；provider 包测试
// 不经过 bootstrap，而真实 CreateOrder/SuccessPaidOrder 会先渲染默认邮件模板
// （其中含 {% system %}），pongo2.Must 会因此 panic。这个 shim 只让「与本任务无关的
// 邮件渲染」不再中断业务用例，不替代任何被测业务路径；通知模板侧仍在词法白名单处拒绝 system。
func notifyTestRegisterEmailTagShim(t *testing.T) {
	t.Helper()
	parser := func(doc *pongo2.Parser, start *pongo2.Token, arguments *pongo2.Parser) (pongo2.INodeTag, *pongo2.Error) {
		return notifyTestNoopTag{}, nil
	}
	// 已注册（生产或其它用例先注册）时忽略错误。
	_ = pongo2.RegisterTag("system", parser)
	// 默认邮件模板还用到项目自定义过滤器；测试二进制未导入 tags 包，补两个空值 shim。
	emptyFilter := func(in *pongo2.Value, param *pongo2.Value) (*pongo2.Value, *pongo2.Error) {
		return pongo2.AsValue(""), nil
	}
	_ = pongo2.RegisterFilter("dateFormat", emptyFilter)
	_ = pongo2.RegisterFilter("priceFormat", emptyFilter)
}

type notifyTestNoopTag struct{}

func (notifyTestNoopTag) Execute(ctx *pongo2.ExecutionContext, writer pongo2.TemplateWriter) *pongo2.Error {
	return nil
}

// notifyTestEventKeyHash 是测试侧的事件键摘要（独立实现，不复用被测函数）：
// 被测函数算错时清理就找不到行，问题会立刻暴露成残留数据。
func notifyTestEventKeyHash(eventKey string) string {
	sum := sha256.Sum256([]byte(eventKey))
	return hex.EncodeToString(sum[:])
}

func newNotifyTestRule(t *testing.T, w *Website, eventType string, templateId uint, targetIds ...uint) *model.NotificationRule {
	t.Helper()
	if targetIds == nil {
		targetIds = []uint{}
	}
	buf, err := json.Marshal(targetIds)
	if err != nil {
		t.Fatalf("序列化规则目标失败：%v", err)
	}
	rule := &model.NotificationRule{
		SiteId:     w.Id,
		Name:       "通知规则-" + notifyTestSuffix(),
		EventType:  eventType,
		Enabled:    1,
		TargetIds:  string(buf),
		TemplateId: templateId,
	}
	if err := w.DB.Create(rule).Error; err != nil {
		t.Fatalf("创建通知规则失败：%v", err)
	}
	t.Cleanup(func() {
		if err := w.DB.Where("id = ? AND site_id = ?", rule.Id, w.Id).Delete(&model.NotificationRule{}).Error; err != nil {
			t.Logf("清理通知规则 %d 失败：%v", rule.Id, err)
		}
	})
	return rule
}

func newNotifyTestGroupTarget(t *testing.T, w *Website, channelId uint, approved int) *model.ChannelTarget {
	t.Helper()
	suffix := notifyTestSuffix()
	platformTargetId := "notify_group_" + suffix
	target := &model.ChannelTarget{
		SiteId:           w.Id,
		ChannelId:        channelId,
		Kind:             TargetKindGroup,
		PlatformTargetId: platformTargetId,
		Approved:         approved,
		TargetHash:       bindingTargetDigest(w.Id, channelId, TargetKindGroup, platformTargetId+"|"+suffix),
	}
	if err := w.DB.Create(target).Error; err != nil {
		t.Fatalf("创建群目标失败：%v", err)
	}
	t.Cleanup(func() {
		if err := w.DB.Where("id = ? AND site_id = ?", target.Id, w.Id).Delete(&model.ChannelTarget{}).Error; err != nil {
			t.Logf("清理群目标 %d 失败：%v", target.Id, err)
		}
	})
	return target
}

func newNotifyTestPrivateTarget(t *testing.T, w *Website, channelId, bindingId uint, approved int) *model.ChannelTarget {
	t.Helper()
	suffix := notifyTestSuffix()
	platformTargetId := "notify_private_" + suffix
	target := &model.ChannelTarget{
		SiteId:           w.Id,
		ChannelId:        channelId,
		Kind:             TargetKindPrivate,
		PlatformTargetId: platformTargetId,
		BindingId:        bindingId,
		Approved:         approved,
		TargetHash:       bindingTargetDigest(w.Id, channelId, TargetKindPrivate, platformTargetId+"|"+suffix),
	}
	if err := w.DB.Create(target).Error; err != nil {
		t.Fatalf("创建私聊目标失败：%v", err)
	}
	t.Cleanup(func() {
		if err := w.DB.Where("id = ? AND site_id = ?", target.Id, w.Id).Delete(&model.ChannelTarget{}).Error; err != nil {
			t.Logf("清理私聊目标 %d 失败：%v", target.Id, err)
		}
	})
	return target
}

func newNotifyTestTemplate(t *testing.T, w *Website, key, subject, content, overrides string) *model.NotificationTemplate {
	t.Helper()
	tpl := &model.NotificationTemplate{
		SiteId:                w.Id,
		Key:                   key,
		Name:                  "通知模板-" + notifyTestSuffix(),
		Subject:               subject,
		Content:               content,
		PlatformOverridesJson: overrides,
	}
	if err := w.DB.Create(tpl).Error; err != nil {
		t.Fatalf("创建通知模板失败：%v", err)
	}
	t.Cleanup(func() {
		if err := w.DB.Where("id = ? AND site_id = ?", tpl.Id, w.Id).Delete(&model.NotificationTemplate{}).Error; err != nil {
			t.Logf("清理通知模板 %d 失败：%v", tpl.Id, err)
		}
	})
	return tpl
}

func newNotifyTestProduct(t *testing.T, w *Website) *model.Archive {
	t.Helper()
	suffix := notifyTestSuffix()
	archive := &model.Archive{
		Title:      "通知测试商品-" + suffix,
		ModuleId:   2,
		CategoryId: 1,
		UrlToken:   "notify-product-" + suffix,
		Price:      1234,
		Stock:      10,
	}
	if err := w.DB.Create(archive).Error; err != nil {
		t.Fatalf("创建测试商品失败：%v", err)
	}
	t.Cleanup(func() {
		if err := w.DB.Unscoped().Where("archive_id = ?", archive.Id).Delete(&model.ArchiveProduct{}).Error; err != nil {
			t.Logf("清理商品侧表 %d 失败：%v", archive.Id, err)
		}
		if err := w.DB.Unscoped().Where("id = ?", archive.Id).Delete(&model.Archive{}).Error; err != nil {
			t.Logf("清理测试商品 %d 失败：%v", archive.Id, err)
		}
	})
	return archive
}

func newNotifyTestUser(t *testing.T, w *Website) *model.User {
	t.Helper()
	suffix := notifyTestSuffix()
	user := &model.User{
		UserName: "notify_user_" + suffix,
		Email:    "notify_" + suffix + "@example.com",
		Password: "notify_test_password",
		Status:   1,
	}
	if err := w.DB.Create(user).Error; err != nil {
		t.Fatalf("创建测试用户失败：%v", err)
	}
	t.Cleanup(func() {
		if err := w.DB.Unscoped().Where("id = ?", user.Id).Delete(&model.User{}).Error; err != nil {
			t.Logf("清理测试用户 %d 失败：%v", user.Id, err)
		}
	})
	return user
}

// notifyTestWithNoNeedLogin 临时打开「免登录下单」，清理时还原原值。
func notifyTestWithNoNeedLogin(t *testing.T, w *Website, enabled bool) {
	t.Helper()
	if w.PluginOrder == nil {
		t.Fatal("站点缺少订单插件配置")
	}
	origin := w.PluginOrder.NoNeedLogin
	w.PluginOrder.NoNeedLogin = enabled
	t.Cleanup(func() { w.PluginOrder.NoNeedLogin = origin })
}

func notifyTestCleanupOrder(t *testing.T, w *Website, order *model.Order) {
	t.Helper()
	t.Cleanup(func() {
		if err := w.DB.Unscoped().Where("order_id = ?", order.OrderId).Delete(&model.OrderDetail{}).Error; err != nil {
			t.Logf("清理订单明细 %s 失败：%v", order.OrderId, err)
		}
		if err := w.DB.Unscoped().Where("id = ?", order.Id).Delete(&model.Order{}).Error; err != nil {
			t.Logf("清理订单 %d 失败：%v", order.Id, err)
		}
	})
}

// cleanupNotifyBusinessKeys 按业务事件键精确清理事件及其投递。
func cleanupNotifyBusinessKeys(t *testing.T, w *Website, eventKeys ...string) {
	t.Helper()
	t.Cleanup(func() {
		for _, key := range eventKeys {
			hash := notifyTestEventKeyHash(key)
			var ids []uint
			if err := w.DB.Model(&model.NotificationEvent{}).
				Where("site_id = ? AND event_key_hash = ?", w.Id, hash).Pluck("id", &ids).Error; err != nil {
				t.Logf("清理事件 %s 时读取 ID 失败：%v", key, err)
				continue
			}
			for _, id := range ids {
				if err := w.DB.Unscoped().Where("event_id = ?", id).Delete(&model.NotificationDelivery{}).Error; err != nil {
					t.Logf("清理事件 %d 的投递失败：%v", id, err)
				}
				if err := w.DB.Unscoped().Where("id = ?", id).Delete(&model.NotificationEvent{}).Error; err != nil {
					t.Logf("清理事件 %d 失败：%v", id, err)
				}
			}
		}
	})
}

func notifyTestLoadEvent(t *testing.T, w *Website, eventType, businessKey string) *model.NotificationEvent {
	t.Helper()
	var event model.NotificationEvent
	hash := notifyTestEventKeyHash(eventType + ":" + businessKey)
	if err := w.DB.Where("site_id = ? AND event_key_hash = ?", w.Id, hash).Take(&event).Error; err != nil {
		t.Fatalf("读取事件 %s:%s 失败：%v", eventType, businessKey, err)
	}
	return &event
}

func notifyTestDeliveries(t *testing.T, w *Website, eventId, ruleId uint) []*model.NotificationDelivery {
	t.Helper()
	var deliveries []*model.NotificationDelivery
	err := w.DB.Where("site_id = ? AND event_id = ? AND rule_id = ?", w.Id, eventId, ruleId).
		Order("id asc").Find(&deliveries).Error
	if err != nil {
		t.Fatalf("读取投递失败：%v", err)
	}
	return deliveries
}

// notifyTestEmitReady 走真实入口写一条 ready 事件（并在同一事务内扇出）。
func notifyTestEmitReady(t *testing.T, w *Website, eventType, businessKey string, payload map[string]any) *model.NotificationEvent {
	t.Helper()
	tx := w.DB.Begin()
	if tx.Error != nil {
		t.Fatalf("开启事务失败：%v", tx.Error)
	}
	event, err := w.emitNotificationEvent(tx, eventType, businessKey, time.Now().Unix(), payload, NotifyEventStatusReady)
	if err != nil {
		tx.Rollback()
		t.Fatalf("写入通知事件 %s:%s 失败：%v", eventType, businessKey, err)
	}
	if err := tx.Commit().Error; err != nil {
		t.Fatalf("提交通知事件失败：%v", err)
	}
	cleanupNotifyBusinessKeys(t, w, eventType+":"+businessKey)
	return event
}

// ============================================================
// 留言事件：分类之前不得外发
// ============================================================

func TestGuestbookEventWaitsForClassification(t *testing.T) {
	w := notifyTestSite(t)
	ch := newManagerTestChannel(t, w, true)
	target := newNotifyTestGroupTarget(t, w, ch.Id, 1)
	rule := newNotifyTestRule(t, w, NotifyEventGuestbookCreated, 0, target.Id)

	guestbook := &model.Guestbook{
		UserName: "留言人-" + notifyTestSuffix(),
		Contact:  "13800000000",
		Content:  "想咨询一下产品报价",
		Ip:       "127.0.0.1",
		Refer:    "https://referrer.example.com/notify",
		// 自定义字段必须一起给：否则「载荷不含 extra_data」的断言在实现回归后仍然通过。
		ExtraData: model.ExtraData{"company": "哨兵公司QZ9"},
	}
	if err := w.SaveGuestbookWithNotification(guestbook); err != nil {
		t.Fatalf("保存留言失败：%v", err)
	}
	t.Cleanup(func() {
		if err := w.DB.Unscoped().Where("id = ?", guestbook.Id).Delete(&model.Guestbook{}).Error; err != nil {
			t.Logf("清理留言 %d 失败：%v", guestbook.Id, err)
		}
	})
	businessKey := strconv.FormatUint(uint64(guestbook.Id), 10)
	cleanupNotifyBusinessKeys(t, w, NotifyEventGuestbookCreated+":"+businessKey)

	event := notifyTestLoadEvent(t, w, NotifyEventGuestbookCreated, businessKey)
	if event.Status != NotifyEventStatusWaitingClassification {
		t.Fatalf("留言保存后事件应为 waiting_classification，实际 %q", event.Status)
	}
	if got := len(notifyTestDeliveries(t, w, event.Id, rule.Id)); got != 0 {
		t.Fatalf("分类完成前不得创建投递，实际 %d 条", got)
	}
	var payload map[string]any
	if err := json.Unmarshal([]byte(event.PayloadJson), &payload); err != nil {
		t.Fatalf("事件载荷不是合法 JSON：%v", err)
	}
	if payload["user_name"] == "" || payload["content"] == "" {
		t.Fatalf("事件载荷缺少白名单字段：%v", payload)
	}
	// 载荷白名单只有 5 个字段：网络元数据（ip/refer）与任意结构的自定义字段不得进事件表。
	for _, forbidden := range []string{"ip", "refer", "extra_data"} {
		if value, exists := payload[forbidden]; exists {
			t.Fatalf("留言事件载荷不得包含 %s（实际 %v），只允许 guestbook_id/user_name/contact/content/created_time", forbidden, value)
		}
	}
	if got := len(payload); got != 5 {
		t.Fatalf("留言事件载荷应恰好 5 个白名单字段，实际 %d 个：%v", got, payload)
	}

	w.ProcessGuestbookNotification(guestbook, 1, AkismetOutcomeChecked)
	event = notifyTestLoadEvent(t, w, NotifyEventGuestbookCreated, businessKey)
	if event.Status != NotifyEventStatusReady {
		t.Fatalf("正常留言分类后事件应为 ready，实际 %q", event.Status)
	}
	if got := len(notifyTestDeliveries(t, w, event.Id, rule.Id)); got != 1 {
		t.Fatalf("正常留言应产生 1 条投递，实际 %d 条", got)
	}

	// 反向：同一留言重复处理一次，投递数仍为 1（幂等）。
	w.ProcessGuestbookNotification(guestbook, 1, AkismetOutcomeChecked)
	if got := len(notifyTestDeliveries(t, w, event.Id, rule.Id)); got != 1 {
		t.Fatalf("重复处理不得增加投递，实际 %d 条", got)
	}

	spam := &model.Guestbook{
		UserName: "垃圾留言人-" + notifyTestSuffix(),
		Content:  "买量加微信",
	}
	if err := w.SaveGuestbookWithNotification(spam); err != nil {
		t.Fatalf("保存垃圾留言失败：%v", err)
	}
	t.Cleanup(func() {
		if err := w.DB.Unscoped().Where("id = ?", spam.Id).Delete(&model.Guestbook{}).Error; err != nil {
			t.Logf("清理垃圾留言 %d 失败：%v", spam.Id, err)
		}
	})
	spamKey := strconv.FormatUint(uint64(spam.Id), 10)
	cleanupNotifyBusinessKeys(t, w, NotifyEventGuestbookCreated+":"+spamKey)

	w.ProcessGuestbookNotification(spam, 2, AkismetOutcomeChecked)
	spamEvent := notifyTestLoadEvent(t, w, NotifyEventGuestbookCreated, spamKey)
	if spamEvent.Status != NotifyEventStatusCancelled {
		t.Fatalf("垃圾留言事件应为 cancelled，实际 %q", spamEvent.Status)
	}
	if spamEvent.LastError == "" {
		t.Fatal("垃圾留言事件应记录静态原因")
	}
	if got := len(notifyTestDeliveries(t, w, spamEvent.Id, rule.Id)); got != 0 {
		t.Fatalf("垃圾留言不得产生投递，实际 %d 条", got)
	}

	// 检测未完成（AkismetOutcomeFailed，且留言非垃圾）：必须停在待分类、留下静态原因、零投递。
	// 与「垃圾留言」分支的区别是状态不同：这里不得被当成 ready 或 cancelled。
	pending := &model.Guestbook{
		UserName: "检测失败留言人-" + notifyTestSuffix(),
		Content:  "检测服务不可用时的正常留言",
	}
	if err := w.SaveGuestbookWithNotification(pending); err != nil {
		t.Fatalf("保存检测失败留言失败：%v", err)
	}
	t.Cleanup(func() {
		if err := w.DB.Unscoped().Where("id = ?", pending.Id).Delete(&model.Guestbook{}).Error; err != nil {
			t.Logf("清理检测失败留言 %d 失败：%v", pending.Id, err)
		}
	})
	pendingKey := strconv.FormatUint(uint64(pending.Id), 10)
	cleanupNotifyBusinessKeys(t, w, NotifyEventGuestbookCreated+":"+pendingKey)

	w.ProcessGuestbookNotification(pending, 1, AkismetOutcomeFailed)
	pendingEvent := notifyTestLoadEvent(t, w, NotifyEventGuestbookCreated, pendingKey)
	if pendingEvent.Status != NotifyEventStatusWaitingClassification {
		t.Fatalf("垃圾检测未完成时事件必须停在 waiting_classification，实际 %q", pendingEvent.Status)
	}
	if pendingEvent.LastError == "" {
		t.Fatal("垃圾检测未完成时必须记录静态原因，便于后台排查")
	}
	if got := len(notifyTestDeliveries(t, w, pendingEvent.Id, rule.Id)); got != 0 {
		t.Fatalf("垃圾检测未完成时不得产生投递，实际 %d 条", got)
	}
}

// ============================================================
// 通知侧失败不得回滚已完成业务
// ============================================================

func TestNotificationFailureDoesNotBlockCreateOrder(t *testing.T) {
	w := notifyTestSite(t)
	ch := newManagerTestChannel(t, w, true)
	target := newNotifyTestGroupTarget(t, w, ch.Id, 1)
	tpl := newNotifyTestTemplate(t, w, NotifyEventOrderCreated, "新订单", `{% system with name="SiteName" %}`, "")
	rule := newNotifyTestRule(t, w, NotifyEventOrderCreated, tpl.Id, target.Id)

	notifyTestWithNoNeedLogin(t, w, true)
	archive := newNotifyTestProduct(t, w)

	order, err := w.CreateOrder(0, &request.OrderRequest{
		Type:    config.OrderTypeGoods,
		Details: []request.OrderDetail{{GoodsId: archive.Id, Quantity: 1}},
	})
	if err != nil {
		t.Fatalf("通知模板渲染失败不得让下单失败：%v", err)
	}
	notifyTestCleanupOrder(t, w, order)
	cleanupNotifyBusinessKeys(t, w, NotifyEventOrderCreated+":"+order.OrderId)

	var saved model.Order
	if err := w.DB.Where("id = ?", order.Id).Take(&saved).Error; err != nil {
		t.Fatalf("通知侧失败后订单行不存在（业务被回滚）：%v", err)
	}

	event := notifyTestLoadEvent(t, w, NotifyEventOrderCreated, order.OrderId)
	deliveries := notifyTestDeliveries(t, w, event.Id, rule.Id)
	if len(deliveries) != 1 {
		t.Fatalf("渲染失败也必须留下 1 条可观测投递，实际 %d 条", len(deliveries))
	}
	if deliveries[0].Status != NotifyDeliveryStatusFailed {
		t.Fatalf("渲染失败的投递应为 failed，实际 %q", deliveries[0].Status)
	}
	if deliveries[0].LastError == "" {
		t.Fatal("渲染失败的投递必须记录静态原因")
	}
	if deliveries[0].Attempts != 0 {
		t.Fatalf("渲染失败的投递不得计入发送尝试，实际 %d 次", deliveries[0].Attempts)
	}
}

// ============================================================
// 支付成功事件：并发只有赢家产生一次
// ============================================================

func TestSuccessPaidOrderEmitsEventOnce(t *testing.T) {
	w := notifyTestSite(t)
	user := newNotifyTestUser(t, w)
	archive := newNotifyTestProduct(t, w)

	order, err := w.CreateOrder(user.Id, &request.OrderRequest{
		Type:    config.OrderTypeGoods,
		Details: []request.OrderDetail{{GoodsId: archive.Id, Quantity: 1}},
	})
	if err != nil {
		t.Fatalf("创建订单失败：%v", err)
	}
	notifyTestCleanupOrder(t, w, order)
	cleanupNotifyBusinessKeys(t, w,
		NotifyEventOrderCreated+":"+order.OrderId,
		NotifyEventOrderPaid+":"+order.OrderId,
	)

	payment := &model.Payment{OrderId: order.OrderId, Amount: order.Amount}
	first := *order
	second := *order

	// order_count 只在状态迁移赢家分支之后累加（SuccessPaidOrder 里 RowsAffected == 1 之后），
	// 是「赢家独占副作用」的可观测证据：事件行数被 uk_event_key_hash 兜住，删掉赢家判断也不会变红。
	var beforeOrderCount int64
	if err := w.DB.Model(&model.User{}).Where("id = ?", user.Id).Pluck("order_count", &beforeOrderCount).Error; err != nil {
		t.Fatalf("读取用户下单数失败：%v", err)
	}

	start := make(chan struct{})
	errs := make([]error, 2)
	var wg sync.WaitGroup
	wg.Add(2)
	go func() {
		defer wg.Done()
		<-start
		errs[0] = w.SuccessPaidOrder(&first, payment)
	}()
	go func() {
		defer wg.Done()
		<-start
		errs[1] = w.SuccessPaidOrder(&second, payment)
	}()
	close(start)
	wg.Wait()

	for i, callErr := range errs {
		if callErr != nil {
			t.Fatalf("第 %d 次支付成功回调返回错误（重复回调应直接返回 nil）：%v", i+1, callErr)
		}
	}

	var count int64
	err = w.DB.Model(&model.NotificationEvent{}).
		Where("site_id = ? AND event_type = ? AND event_key LIKE ?", w.Id, NotifyEventOrderPaid, "%"+order.OrderId+"%").
		Count(&count).Error
	if err != nil {
		t.Fatalf("统计支付事件失败：%v", err)
	}
	if count != 1 {
		t.Fatalf("并发支付只允许产生 1 条 order.paid 事件，实际 %d 条", count)
	}

	// 赢家独占的副作用：只有 RowsAffected == 1 的那次调用会执行到 order_count 累加。
	// 这里断言「恰好 +1」而不是「>= 1」：删掉赢家判断后两次调用都会累加，delta 变 2。
	var afterOrderCount int64
	if err := w.DB.Model(&model.User{}).Where("id = ?", user.Id).Pluck("order_count", &afterOrderCount).Error; err != nil {
		t.Fatalf("读取用户下单数失败：%v", err)
	}
	if delta := afterOrderCount - beforeOrderCount; delta != 1 {
		t.Fatalf("并发支付只允许赢家累加用户下单数：期望 +1，实际 +%d（之前 %d，之后 %d）",
			delta, beforeOrderCount, afterOrderCount)
	}
}

// ============================================================
// 匿名订单同样产生 order.created（不依赖邮箱 / Brevo / 登录态）
// ============================================================

func TestAnonymousOrderEmitsCreatedEvent(t *testing.T) {
	w := notifyTestSite(t)
	notifyTestWithNoNeedLogin(t, w, true)
	archive := newNotifyTestProduct(t, w)

	order, err := w.CreateOrder(0, &request.OrderRequest{
		Type:    config.OrderTypeGoods,
		Details: []request.OrderDetail{{GoodsId: archive.Id, Quantity: 1}},
	})
	if err != nil {
		t.Fatalf("匿名下单失败：%v", err)
	}
	notifyTestCleanupOrder(t, w, order)
	cleanupNotifyBusinessKeys(t, w, NotifyEventOrderCreated+":"+order.OrderId)

	if order.UserId != 0 {
		t.Fatalf("匿名订单的 UserId 应为 0，实际 %d", order.UserId)
	}
	event := notifyTestLoadEvent(t, w, NotifyEventOrderCreated, order.OrderId)
	var payload map[string]any
	if err := json.Unmarshal([]byte(event.PayloadJson), &payload); err != nil {
		t.Fatalf("事件载荷不是合法 JSON：%v", err)
	}
	if got, _ := payload["order_id"].(string); got != order.OrderId {
		t.Fatalf("载荷 order_id 应为 %q，实际 %v", order.OrderId, payload["order_id"])
	}
	if _, ok := payload["items"]; ok {
		t.Fatal("载荷不得包含商品明细")
	}
}

// ============================================================
// 群模板不得看到私聊字段
// ============================================================

func TestGroupTemplateCannotRenderForbiddenFields(t *testing.T) {
	payload, err := json.Marshal(map[string]any{
		"guestbook_id":  1,
		"user_name":     "姓名哨兵QZ1",
		"contact":       "电话哨兵QZ2",
		"content":       "正文哨兵QZ3",
		"buyer_name":    "买家哨兵QZ4",
		"buyer_phone":   "手机哨兵QZ5",
		"buyer_email":   "邮箱哨兵QZ6",
		"buyer_address": "地址哨兵QZ7",
	})
	if err != nil {
		t.Fatalf("序列化载荷失败：%v", err)
	}
	tpl := &model.NotificationTemplate{
		Subject: "[{{ site_name }}] {{ event_label }}",
		Content: "留言人：{{ guestbook.user_name }} 联系方式：{{ guestbook.contact }} " +
			"正文：{{ guestbook.content }} 地址：{{ order.buyer_address }}",
	}
	sentinels := []string{"QZ1", "QZ2", "QZ3", "QZ4", "QZ5", "QZ6", "QZ7"}

	groupSubject, groupText, err := RenderNotificationText(tpl, "feishu", NotifyEventGuestbookCreated, string(payload), TargetKindGroup, "测试站点", "https://example.com")
	if err != nil {
		t.Fatalf("群模板渲染失败：%v", err)
	}
	for _, sentinel := range sentinels {
		if strings.Contains(groupSubject, sentinel) || strings.Contains(groupText, sentinel) {
			t.Fatalf("群聊模板渲染出了私聊字段 %s：%q", sentinel, groupSubject+groupText)
		}
	}

	_, privateText, err := RenderNotificationText(tpl, "feishu", NotifyEventGuestbookCreated, string(payload), TargetKindPrivate, "测试站点", "https://example.com")
	if err != nil {
		t.Fatalf("私聊模板渲染失败：%v", err)
	}
	for _, sentinel := range []string{"QZ1", "QZ2", "QZ3", "QZ7"} {
		if !strings.Contains(privateText, sentinel) {
			t.Fatalf("私聊模板应能渲染 %s，实际输出 %q", sentinel, privateText)
		}
	}
}

// ============================================================
// 模板词法白名单
// ============================================================

func TestTemplateRejectsSystemAndIncludeTags(t *testing.T) {
	rejected := []string{
		`{% system with name="SiteName" %}`,
		`{% include "x" %}`,
		`{% extends "x" %}`,
		`{% import "x" as y %}`,
		`{{ x|safe }}`,
		`{{ x|striptags }}`,
	}
	for _, content := range rejected {
		tpl := &model.NotificationTemplate{Subject: "标题", Content: content}
		_, _, err := RenderNotificationText(tpl, "feishu", NotifyEventOrderCreated, `{}`, TargetKindGroup, "站点", "https://example.com")
		if err == nil {
			t.Errorf("模板 %q 应被拒绝", content)
			continue
		}
		if strings.Contains(err.Error(), content) {
			t.Errorf("错误文案不得回显模板内容：%q", err.Error())
		}
	}

	accepted := `{% if status == "paid" %}已支付{% else %}未支付{% endif %}` +
		` {% for item in status %}x{% empty %}（不可枚举）{% endfor %}` +
		` {{ site_name|upper }} {{ occurred_time|date:"2006-01-02" }}`
	tpl := &model.NotificationTemplate{Subject: "通知", Content: accepted}
	_, text, err := RenderNotificationText(tpl, "feishu", NotifyEventOrderPaid, `{"status":"paid"}`, TargetKindGroup, "站点", "https://example.com")
	if err != nil {
		t.Fatalf("合法模板应能渲染：%v", err)
	}
	if !strings.Contains(text, "已支付") {
		t.Fatalf("合法模板渲染结果不对：%q", text)
	}
}

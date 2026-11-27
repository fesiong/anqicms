package provider

import (
	"encoding/json"
	"strings"
	"testing"
	"time"

	"github.com/go-pay/gopay/paypal"
	"kandaoni.com/anqicms/config"
	"kandaoni.com/anqicms/model"
)

// 这一组对应 v3.8.3 客户实测的 PayPal 通知问题：
//
//	① 验签接口路径写成 /v1/notify-webhook/…（官方是 /v1/notifications/…）→ 404
//	② 退款通知的 resource.id 是 Refund ID，不是 order id 也不是 capture id → 永远 record not found
//	③ 已扣款后重投 CHECKOUT.ORDER.APPROVED → ORDER_ALREADY_CAPTURED → 老实现回 500 被无限重投

// TestPaypalVerifyPathIsOfficial 锁死验签路径。
//
// 反向对照：把常量改回 "/v1/notify-webhook/verify-webhook-signature" 这条立刻红。
// 这个错误本地跑一切正常（HTTP 客户端只是收一个 404），只有真实通知才会暴露。
func TestPaypalVerifyPathIsOfficial(t *testing.T) {
	want := "/v1/notifications/verify-webhook-signature"
	if paypalVerifyPath != want {
		t.Fatalf("验签路径必须是 %q（PayPal 官方路由），实际 %q", want, paypalVerifyPath)
	}
	got := paypalVerifySignatureURL(true)
	if got != paypalProdBaseURL+want {
		t.Fatalf("生产环境验签地址拼错: %s", got)
	}
	if !strings.HasPrefix(paypalVerifySignatureURL(false), paypalSandboxBaseURL) {
		t.Fatal("沙盒环境必须走 api-m.sandbox.paypal.com")
	}
}

// TestPaypalUpLinkCaptureId 退款通知只能靠 links[rel=up] 找 capture。
func TestPaypalUpLinkCaptureId(t *testing.T) {
	cases := []struct {
		name string
		raw  string
		want string
	}{
		{
			name: "sandbox 绝对地址",
			raw:  `{"links":[{"href":"https://api-m.sandbox.paypal.com/v2/payments/captures/3C1234567890","rel":"up","method":"GET"}]}`,
			want: "3C1234567890",
		},
		{
			name: "带查询串",
			raw:  `{"links":[{"href":"https://api-m.paypal.com/v2/payments/captures/CAP-9?fields=all","rel":"up"}]}`,
			want: "CAP-9",
		},
		{
			name: "只有 self 链接时不能瞎猜",
			raw:  `{"links":[{"href":"https://api-m.paypal.com/v2/payments/refunds/R-1","rel":"self"}]}`,
			want: "",
		},
	}
	for _, c := range cases {
		var res PaypalWebhookResource
		if err := json.Unmarshal([]byte(c.raw), &res); err != nil {
			t.Fatalf("%s: 解析失败 %v", c.name, err)
		}
		if got := res.UpLinkCaptureId(); got != c.want {
			t.Fatalf("%s: capture id 解析错误，want %q got %q", c.name, c.want, got)
		}
	}
}

// TestGetPaymentInfoByPaypalRefundResource 退款事件必须能定位到支付单。
//
// 真实的 PAYMENT.CAPTURE.REFUNDED 长这样（refund 对象）：
//
//	{"id":"R-REFUND-1","status":"COMPLETED","amount":{"total":"0.50"},
//	 "links":[{"href":".../v2/payments/captures/CAP-REF-1","rel":"up"}]}
//
// 老实现按 capture/order id 匹配，三种查法全部落空 → 404 "record not found" → PayPal 重投。
func TestGetPaymentInfoByPaypalRefundResource(t *testing.T) {
	w := newZeroValueTestSite(t)
	orderId := testOrderId("pr")
	payment := &model.Payment{
		PaymentId: testPaymentId("pr"),
		OrderId:   orderId,
		Amount:    50,
		PayWay:    config.PayWayPaypal,
		TerraceId: "ORDER-PR-1", // 下单时存的是 order id
		CaptureId: "CAP-REF-1",  // 付款通知里补的 capture id
	}
	if err := w.DB.Create(payment).Error; err != nil {
		t.Fatalf("造支付单失败: %v", err)
	}
	t.Cleanup(func() {
		w.DB.Unscoped().Where("`order_id` = ?", orderId).Delete(&model.Payment{})
		w.DB.Unscoped().Where("`order_id` = ?", orderId).Delete(&model.OrderRefund{})
	})

	raw := `{"id":"R-REFUND-1","status":"COMPLETED","amount":{"total":"0.50","currency_code":"USD"},
		"links":[{"href":"https://api-m.sandbox.paypal.com/v2/payments/captures/CAP-REF-1","rel":"up","method":"GET"},
		{"href":"https://api-m.sandbox.paypal.com/v2/payments/refunds/R-REFUND-1","rel":"self","method":"GET"}]}`
	var res PaypalWebhookResource
	if err := json.Unmarshal([]byte(raw), &res); err != nil {
		t.Fatalf("解析退款 resource 失败: %v", err)
	}
	got, err := w.GetPaymentInfoByPaypalResource(&res)
	if err != nil {
		t.Fatalf("退款通知必须能定位到支付单（否则通知会被 404 无限重投）: %v", err)
	}
	if got.PaymentId != payment.PaymentId {
		t.Fatalf("定位到了错误的支付单: %s", got.PaymentId)
	}

	// 反向对照：去掉 links 后必须定位失败 —— 证明上面能查到是靠 capture 关联，
	// 不是碰巧 id 撞上了 terrace_id。
	res.Links = nil
	if _, err := w.GetPaymentInfoByPaypalResource(&res); err == nil {
		t.Fatal("反向对照失效：没有 links 时不该还能按 refund id 命中支付单")
	}
}

// TestGetPaymentInfoByRefundTerraceId 老数据没有 capture_id，只能靠退款单反查。
func TestGetPaymentInfoByRefundTerraceId(t *testing.T) {
	w := newZeroValueTestSite(t)
	orderId := testOrderId("rf")
	payment := &model.Payment{
		PaymentId: testPaymentId("rf"),
		OrderId:   orderId,
		Amount:    50,
		PayWay:    config.PayWayPaypal,
		TerraceId: "ORDER-RF-1",
	}
	if err := w.DB.Create(payment).Error; err != nil {
		t.Fatalf("造支付单失败: %v", err)
	}
	refund := &model.OrderRefund{OrderId: orderId, Amount: 50, Status: config.OrderRefundStatusWaiting, TerraceId: "R-REFUND-RF-1"}
	if err := w.DB.Create(refund).Error; err != nil {
		t.Fatalf("造退款单失败: %v", err)
	}
	t.Cleanup(func() {
		w.DB.Unscoped().Where("`order_id` = ?", orderId).Delete(&model.Payment{})
		w.DB.Unscoped().Where("`order_id` = ?", orderId).Delete(&model.OrderRefund{})
	})

	// 没有 links、也没有 capture_id 的退款资源
	res := &PaypalWebhookResource{Id: "R-REFUND-RF-1"}
	got, err := w.GetPaymentInfoByPaypalResource(res)
	if err != nil {
		t.Fatalf("应能按 order_refunds.terrace_id 反查到支付单: %v", err)
	}
	if got.PaymentId != payment.PaymentId {
		t.Fatalf("反查到错误的支付单: %s", got.PaymentId)
	}
}

// TestHandlePaypalCaptureRefundedUsesRefundAmount 退款金额取 refund.amount.total。
//
// 老实现只认 capture 的 seller_receivable_breakdown（refund 对象上没有）与
// status=="REFUNDED"（refund 的 status 是 COMPLETED/PENDING），最后都落到
// 「本地退款单金额」这一步，部分退款会把金额记错。
func TestHandlePaypalCaptureRefundedUsesRefundAmount(t *testing.T) {
	w := newZeroValueTestSite(t)
	orderId := testOrderId("ra")
	order := mustCreateTestOrder(t, w, orderId)
	payment := &model.Payment{
		PaymentId: testPaymentId("ra"),
		OrderId:   orderId,
		Amount:    order.Amount,
		PayWay:    config.PayWayPaypal,
		TerraceId: "ORDER-RA-1",
		CaptureId: "CAP-RA-1",
		PaidTime:  time.Now().Unix(),
	}
	if err := w.DB.Create(payment).Error; err != nil {
		t.Fatalf("造支付单失败: %v", err)
	}
	t.Cleanup(func() { w.DB.Unscoped().Where("`order_id` = ?", orderId).Delete(&model.Payment{}) })

	var res PaypalWebhookResource
	raw := `{"id":"R-RA-1","status":"COMPLETED","amount":{"total":"0.50","currency_code":"USD"},
		"links":[{"href":"https://api-m.sandbox.paypal.com/v2/payments/captures/CAP-RA-1","rel":"up"}]}`
	if err := json.Unmarshal([]byte(raw), &res); err != nil {
		t.Fatalf("解析失败: %v", err)
	}
	if err := w.handlePaypalCaptureRefunded(payment, &res); err != nil {
		t.Fatalf("退款通知处理失败: %v", err)
	}

	var saved model.OrderRefund
	if err := w.DB.Where("`order_id` = ?", orderId).Take(&saved).Error; err != nil {
		t.Fatalf("应生成退款单: %v", err)
	}
	if saved.Amount != 50 {
		t.Fatalf("退款金额应取 amount.total=0.50（50 分），实际 %d 分", saved.Amount)
	}
	if saved.TerraceId != "R-RA-1" {
		t.Fatalf("应记下 PayPal 退款单号用于回查，实际 %q", saved.TerraceId)
	}
	var savedOrder model.Order
	w.DB.Where("`id` = ?", order.Id).Take(&savedOrder)
	if savedOrder.Status != config.OrderStatusRefunded {
		t.Fatalf("订单应变为已退款，实际 %d", savedOrder.Status)
	}

	// PayPal 重投同一条退款通知：处理必须成功，且流水不能再多记一笔
	if err := w.handlePaypalCaptureRefunded(payment, &res); err != nil {
		t.Fatalf("重投退款通知应幂等成功: %v", err)
	}
	var refundCount int64
	w.DB.Model(&model.Finance{}).Where("`order_id` = ? AND `action` = ?", orderId, config.FinanceActionRefund).Count(&refundCount)
	if refundCount != 1 {
		t.Fatalf("重投后退款流水必须仍为 1 笔，实际 %d 笔", refundCount)
	}
	var refundCount2 int64
	w.DB.Model(&model.OrderRefund{}).Where("`order_id` = ?", orderId).Count(&refundCount2)
	if refundCount2 != 1 {
		t.Fatalf("重投后不能生成第二张退款单，实际 %d 张", refundCount2)
	}
}

// TestCheckoutOrderApprovedReplayIsIdempotent 已扣款后重投 APPROVED 必须返回成功。
//
// fakePaypal 的订单详情固定返回 status=COMPLETED + CAP-1，正好模拟「钱已经扣过」。
// 老实现会无条件再 capture 一次，拿到 ORDER_ALREADY_CAPTURED 后返回 error → HTTP 500 → 无限重投。
func TestCheckoutOrderApprovedReplayIsIdempotent(t *testing.T) {
	newFakePaypal(t) // 把 SDK 指到假服务
	w := newZeroValueTestSite(t)
	w.CachePath = t.TempDir() + "/"

	account := &model.PaymentAccount{
		PayWay:    config.PayWayPaypal,
		PayConfig: model.PayConfig{AppId: "client-id", AppSecret: "secret", Sandbox: true},
		Status:    1,
	}
	if err := w.DB.Create(account).Error; err != nil {
		t.Fatalf("造支付账号失败: %v", err)
	}
	t.Cleanup(func() { w.DB.Unscoped().Delete(&model.PaymentAccount{}, account.Id) })

	orderId := testOrderId("ap")
	order := &model.Order{OrderId: orderId, Amount: 5000, Status: config.OrderStatusWaiting}
	if err := w.DB.Create(order).Error; err != nil {
		t.Fatalf("造订单失败: %v", err)
	}
	payment := &model.Payment{
		PaymentId:        testPaymentId("ap"),
		OrderId:          orderId,
		Amount:           5000,
		PayWay:           config.PayWayPaypal,
		TerraceId:        "ORDER-1",
		PaymentAccountId: account.Id,
	}
	if err := w.DB.Create(payment).Error; err != nil {
		t.Fatalf("造支付单失败: %v", err)
	}
	t.Cleanup(func() {
		w.DB.Unscoped().Where("`order_id` = ?", orderId).Delete(&model.Payment{})
		w.DB.Unscoped().Where("`order_id` = ?", orderId).Delete(&model.Order{})
		w.DB.Unscoped().Where("`order_id` = ?", orderId).Delete(&model.Finance{})
	})

	// PayPal 重投同一条通知：事件 id 相同，但上一次因为 500 没有写去重标记。
	// 这里直接打 handler，绕开缓存去重，专门验证「已扣款」分支的幂等。
	ev := func() *paypal.WebhookEvent {
		return &paypal.WebhookEvent{
			Id:        "WH-AP-1",
			EventType: "CHECKOUT.ORDER.APPROVED",
			Resource:  json.RawMessage(`{"id":"ORDER-1","status":"APPROVED"}`),
		}
	}
	for i := 0; i < 2; i++ {
		if err := w.handlePaypalEvent(ev()); err != nil {
			t.Fatalf("第 %d 次 APPROVED（订单已扣款）必须成功应答，实际 %v", i+1, err)
		}
	}

	var saved model.Payment
	if err := w.DB.Where("`id` = ?", payment.Id).Take(&saved).Error; err != nil {
		t.Fatalf("读回支付单失败: %v", err)
	}
	if saved.CaptureId != "CAP-1" {
		t.Fatalf("已扣款时应把 capture id 补进库（退款要用），实际 %q", saved.CaptureId)
	}
	var buyCount int64
	w.DB.Model(&model.Finance{}).Where("`order_id` = ? AND `action` = ?", orderId, config.FinanceActionBuy).Count(&buyCount)
	if buyCount > 1 {
		t.Fatalf("重投 APPROVED 不能重复记账，付款流水 %d 笔", buyCount)
	}
}

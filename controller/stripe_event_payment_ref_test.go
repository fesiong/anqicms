package controller

import (
	"encoding/json"
	"os"
	"strings"
	"testing"
)

// 锁住 2026-11-04 排查出的 Stripe 回调 bug：
// notify 里用 `stripe.Event.ID`（evt_ 开头的事件 ID）去查 payment_id，
// 必然查不到 → 静默 return → 返回 200 空响应 → Stripe 认为投递成功，
// 钱收了、订单永远停在未支付。
//
// 正确的映射：payment.terrace_id = PaymentIntent id（pi_...），metadata.payment_id 兜底。

func TestStripeEventPaymentRefUsesObjectNotEventId(t *testing.T) {
	// 真实 payment_intent.succeeded 事件的 data.object 片段
	raw := json.RawMessage(`{
		"id": "pi_3Q0abc",
		"object": "payment_intent",
		"amount": 19900,
		"status": "succeeded",
		"metadata": {"payment_id": "202611041104240420"}
	}`)

	piId, paymentId := stripeEventPaymentRef(raw)
	if piId != "pi_3Q0abc" {
		t.Fatalf("PaymentIntent id 取错：%q", piId)
	}
	if paymentId != "202611041104240420" {
		t.Fatalf("metadata.payment_id 取错：%q", paymentId)
	}
	// 事件 ID 根本不在这个对象里，取出来的值不可能是 evt_
	if piId[:3] == "evt" {
		t.Fatal("不能拿事件 ID 当 PaymentIntent id 用")
	}
}

// 反向对照：没有 metadata 的对象（如 charge.* 事件）不能凭空编出 payment_id，
// 否则会查到别的单子上
func TestStripeEventPaymentRefWithoutMetadata(t *testing.T) {
	raw := json.RawMessage(`{"id":"ch_3Q0xyz","object":"charge"}`)
	piId, paymentId := stripeEventPaymentRef(raw)
	if piId != "ch_3Q0xyz" {
		t.Fatalf("id 取错：%q", piId)
	}
	if paymentId != "" {
		t.Fatalf("没有 metadata 时 payment_id 必须为空，实际 %q", paymentId)
	}
}

func TestStripeEventPaymentRefEmptyObject(t *testing.T) {
	if piId, paymentId := stripeEventPaymentRef(nil); piId != "" || paymentId != "" {
		t.Fatalf("空对象应返回两个空串，实际 %q %q", piId, paymentId)
	}
	if piId, paymentId := stripeEventPaymentRef(json.RawMessage(`not-json`)); piId != "" || paymentId != "" {
		t.Fatalf("非法 JSON 应返回两个空串，实际 %q %q", piId, paymentId)
	}
}

// 调用点守卫：helper 正确不代表调用点正确。
// 这条直接盯源码，禁止 NotifyStripe 再出现「按事件 ID 查 payment_id」的写法。
func TestNotifyStripeNeverLooksUpByEventId(t *testing.T) {
	src, err := os.ReadFile("apiNotify.go")
	if err != nil {
		t.Fatalf("读取 apiNotify.go 失败: %v", err)
	}
	text := string(src)
	if strings.Contains(text, "GetPaymentInfoByPaymentId(e.ID)") {
		t.Fatal("NotifyStripe 不能拿 stripe.Event.ID（evt_ 开头）去查 payment_id，永远查不到")
	}
	// 反向断言，防上面那条变成空转
	if !strings.Contains(text, "stripeEventPaymentRef(e.Data.Raw)") {
		t.Fatal("NotifyStripe 应通过 stripeEventPaymentRef 取 PaymentIntent id / metadata.payment_id")
	}
	if !strings.Contains(text, "currentSite.GetPaymentInfoByTerraceId(piId)") {
		t.Fatal("NotifyStripe 应优先按 payment.terrace_id 定位支付单")
	}
}

// 支付宝：非终态（WAIT_BUYER_PAY 等）不能算支付成功
func TestAlipayTradePaid(t *testing.T) {
	paid := []string{"TRADE_SUCCESS", "TRADE_FINISHED"}
	for _, s := range paid {
		if !alipayTradePaid(s) {
			t.Fatalf("%s 应判定为已支付", s)
		}
	}
	// 反向对照：这些状态绝对不能标记已支付
	for _, s := range []string{"WAIT_BUYER_PAY", "TRADE_CLOSED", "", "SUCCESS"} {
		if alipayTradePaid(s) {
			t.Fatalf("%q 不能判定为已支付", s)
		}
	}
}

package provider

import (
	"encoding/json"
	"testing"
)

// 锁住 PayPal 回调的订单定位：
// CHECKOUT.ORDER.* 的 resource.id 就是 order id（= payment.terrace_id）；
// 但 PAYMENT.CAPTURE.* 的 resource.id 是 capture id，必须用
// supplementary_data.related_ids.order_id 换算，否则这类通知查不到 payment，
// 处理函数直接 return，返回 200，PayPal 不再重试 → 钱收了订单还是未支付。

func TestPaypalWebhookResourceCaptureOrderId(t *testing.T) {
	// 真实的 PAYMENT.CAPTURE.COMPLETED resource 片段
	raw := []byte(`{
		"id": "3C679366HH908993F",
		"status": "COMPLETED",
		"amount": {"value": "199.00", "currency_code": "USD"},
		"supplementary_data": {"related_ids": {"order_id": "5O190127TN364715T"}}
	}`)
	var res PaypalWebhookResource
	if err := json.Unmarshal(raw, &res); err != nil {
		t.Fatalf("解析失败: %v", err)
	}
	if res.Id != "3C679366HH908993F" {
		t.Fatalf("capture id 取错: %q", res.Id)
	}
	if got := res.SupplementaryData.RelatedIds.OrderId; got != "5O190127TN364715T" {
		t.Fatalf("related_ids.order_id 取错（json tag 写错就会走到这里）: %q", got)
	}
}

// 反向对照：CHECKOUT.ORDER.* 没有 supplementary_data，不能凭空空转出 order id
func TestPaypalWebhookResourceOrderEventHasNoRelatedId(t *testing.T) {
	raw := []byte(`{"id":"5O190127TN364715T","status":"APPROVED"}`)
	var res PaypalWebhookResource
	if err := json.Unmarshal(raw, &res); err != nil {
		t.Fatalf("解析失败: %v", err)
	}
	if got := res.SupplementaryData.RelatedIds.OrderId; got != "" {
		t.Fatalf("order 事件不该有 related order id，实际 %q", got)
	}
}

// nil 资源要给错误而不是 panic
func TestGetPaymentInfoByPaypalResourceNil(t *testing.T) {
	w := &Website{}
	if _, err := w.GetPaymentInfoByPaypalResource(nil); err == nil {
		t.Fatal("nil 资源应返回错误")
	}
}

package controller

import (
	"os"
	"strings"
	"testing"
)

// 锁住 v3.8.4 客户实测的「已取消订单还能生成付款链接」：
//
// 取消 US$0.50 沙盒订单后连续两次调 POST /api/order/payment 都返回成功，
// 还生成了两个不同的 PayPal 订单（payment.terrace_id 被后写的覆盖）。
// 原实现只按 order_id 取单，既不校验订单归属，也不看订单状态。
//
// 现在必须：归属校验 + 只允许「待支付且未支付」，且都要排在 GeneratePayment 之前。
func TestCreateOrderPaymentGuardsStatusAndOwner(t *testing.T) {
	src, err := os.ReadFile("apiOrder.go")
	if err != nil {
		t.Fatalf("读取 apiOrder.go 失败: %v", err)
	}
	text := string(src)

	// 反向对照：确认读到的确实是这个 handler，断言不会空转
	handlerIdx := strings.Index(text, "func ApiCreateOrderPayment(ctx iris.Context) {")
	if handlerIdx < 0 {
		t.Fatal("断言空转：apiOrder.go 里找不到 ApiCreateOrderPayment")
	}
	handler := text[handlerIdx:]

	payIdx := strings.Index(handler, "currentSite.GeneratePayment(")
	if payIdx < 0 {
		t.Fatal("ApiCreateOrderPayment 里找不到 GeneratePayment 调用")
	}
	guard := handler[:payIdx]

	if !strings.Contains(guard, "order.UserId != userId") {
		t.Fatal("生成付款链接前必须校验订单归属，否则任何登录用户都能给别人的订单发起支付")
	}
	if !strings.Contains(guard, "order.Status != config.OrderStatusWaiting") {
		t.Fatal("只有待支付订单能生成付款链接：已取消/已退款订单一律拒绝")
	}
	if !strings.Contains(guard, "order.PaidTime > 0") {
		t.Fatal("已支付的订单不能再生成付款链接")
	}
}

// 同一笔 payment 不能被反复新开 PayPal 订单：已有 terrace_id 时必须先尝试复用。
func TestCreatePaypalPaymentReusesExistingOrder(t *testing.T) {
	src, err := os.ReadFile("apiOrder.go")
	if err != nil {
		t.Fatalf("读取 apiOrder.go 失败: %v", err)
	}
	text := string(src)

	idx := strings.Index(text, "func createPaypalPayment(ctx iris.Context, payment *model.Payment, order *model.Order) {")
	if idx < 0 {
		t.Fatal("断言空转：apiOrder.go 里找不到 createPaypalPayment")
	}
	body := text[idx:]

	createIdx := strings.Index(body, "client.CreateOrder(")
	if createIdx < 0 {
		t.Fatal("createPaypalPayment 里找不到 CreateOrder 调用")
	}
	before := body[:createIdx]
	if !strings.Contains(before, "payment.TerraceId != \"\"") {
		t.Fatal("已有 PayPal 订单时必须先尝试复用它的付款链接，否则每调一次就多一个 PayPal 订单")
	}
	if !strings.Contains(before, "client.OrderDetail(") {
		t.Fatal("复用前要用 OrderDetail 确认旧订单还可以付款（未 COMPLETED/VOIDED）")
	}
	if !strings.Contains(body, "paypalApproveLink(") {
		t.Fatal("必须从 links 里取 payer-action/approve 链接，不能硬编码 Links[1]")
	}
}

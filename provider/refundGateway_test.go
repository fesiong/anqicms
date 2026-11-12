package provider

import (
	"crypto/aes"
	"crypto/md5"
	"encoding/base64"
	"encoding/hex"
	"os"
	"strings"
	"testing"

	"github.com/stripe/stripe-go/v82"
	"kandaoni.com/anqicms/config"
	"kandaoni.com/anqicms/model"
)

// TestRefundGatewayStates 各渠道的「退款状态字符串 → 三态」映射。
// 这一层是「网关还在退，本地已经算退完」的唯一防线，错一个就等于凭空记账。
func TestRefundGatewayStates(t *testing.T) {
	cases := []struct {
		name string
		got  refundSettleState
		want refundSettleState
	}{
		// 微信：只有 SUCCESS 算完成，PROCESSING/CHANGE 都不能当完成
		{"wechat SUCCESS", wechatRefundState("SUCCESS"), refundSettleDone},
		{"wechat PROCESSING", wechatRefundState("PROCESSING"), refundSettlePending},
		{"wechat CHANGE", wechatRefundState("CHANGE"), refundSettlePending},
		{"wechat REFUNDCLOSE", wechatRefundState("REFUNDCLOSE"), refundSettleFailed},
		{"wechat empty", wechatRefundState(""), refundSettlePending},
		// 支付宝：refund_status 的值是 REFUND_SUCCESS，不是 SUCCESS
		{"alipay REFUND_SUCCESS", alipayRefundState("REFUND_SUCCESS"), refundSettleDone},
		{"alipay SUCCESS", alipayRefundState("SUCCESS"), refundSettleDone},
		{"alipay empty", alipayRefundState(""), refundSettlePending},
		// 虎皮椒：CD 完成 / RD 退款中 / UD 失败
		{"xunhupay CD", xunhupayRefundState(XunhupayStatusRefunded), refundSettleDone},
		{"xunhupay RD", xunhupayRefundState(XunhupayStatusRefunding), refundSettlePending},
		{"xunhupay UD", xunhupayRefundState(XunhupayStatusRefundFailed), refundSettleFailed},
		// Stripe
		{"stripe succeeded", stripeRefundState(stripe.RefundStatusSucceeded), refundSettleDone},
		{"stripe pending", stripeRefundState(stripe.RefundStatusPending), refundSettlePending},
		{"stripe requires_action", stripeRefundState(stripe.RefundStatusRequiresAction), refundSettlePending},
		{"stripe failed", stripeRefundState(stripe.RefundStatusFailed), refundSettleFailed},
		{"stripe canceled", stripeRefundState(stripe.RefundStatusCanceled), refundSettleFailed},
	}
	for _, c := range cases {
		if c.got != c.want {
			t.Errorf("%s: got %v, want %v", c.name, c.got, c.want)
		}
	}
}

// TestApplyRefundGatewayStatePendingKeepsOrderPaid 网关还在处理时，
// 订单必须还是「已付款」、退款单还是「待处理」，且不能有退款流水。
func TestApplyRefundGatewayStatePendingKeepsOrderPaid(t *testing.T) {
	w := newZeroValueTestSite(t)
	orderId := testOrderId("gp")
	order := mustCreateTestOrder(t, w, orderId)
	refund := &model.OrderRefund{
		OrderId:   orderId,
		UserId:    order.UserId,
		Amount:    5000,
		Status:    config.OrderRefundStatusWaiting,
		TerraceId: "REFUND-P",
	}
	if err := w.DB.Create(refund).Error; err != nil {
		t.Fatalf("造退款单失败: %v", err)
	}

	if err := w.ApplyRefundGatewayState(refund, order, refundSettlePending, "REFUND-P", "处理中"); err != nil {
		t.Fatalf("pending 不应报错: %v", err)
	}
	var saved model.Order
	w.DB.Where("`id` = ?", order.Id).Take(&saved)
	if saved.Status == config.OrderStatusRefunded {
		t.Fatal("网关只是受理，订单不能变已退款")
	}
	var savedRefund model.OrderRefund
	w.DB.Where("`id` = ?", refund.Id).Take(&savedRefund)
	if savedRefund.Status != config.OrderRefundStatusWaiting {
		t.Fatalf("退款单应保持待处理，实际 %d", savedRefund.Status)
	}
	var count int64
	w.DB.Model(&model.Finance{}).Where("`order_id` = ? AND `action` = ?", orderId, config.FinanceActionRefund).Count(&count)
	if count != 0 {
		t.Fatalf("还没退成就记了 %d 笔退款流水", count)
	}
}

// TestApplyRefundGatewayStateDoneRecordsOnce 变成 done 之后，
// 重复调用（通知重投）只能记一笔流水。
func TestApplyRefundGatewayStateDoneRecordsOnce(t *testing.T) {
	w := newZeroValueTestSite(t)
	orderId := testOrderId("gd")
	order := mustCreateTestOrder(t, w, orderId)
	refund := &model.OrderRefund{OrderId: orderId, Amount: 5000, Status: config.OrderRefundStatusWaiting}
	if err := w.DB.Create(refund).Error; err != nil {
		t.Fatalf("造退款单失败: %v", err)
	}
	for i := 0; i < 3; i++ {
		if err := w.ApplyRefundGatewayState(refund, nil, refundSettleDone, "REFUND-D", "成功"); err != nil {
			t.Fatalf("第 %d 次收尾失败: %v", i+1, err)
		}
	}
	var saved model.Order
	w.DB.Where("`id` = ?", order.Id).Take(&saved)
	if saved.Status != config.OrderStatusRefunded {
		t.Fatalf("订单应已退款，status=%d", saved.Status)
	}
	var count int64
	w.DB.Model(&model.Finance{}).Where("`order_id` = ? AND `action` = ?", orderId, config.FinanceActionRefund).Count(&count)
	if count != 1 {
		t.Fatalf("重复收尾只能记 1 笔退款流水，实际 %d", count)
	}
	var savedRefund model.OrderRefund
	w.DB.Where("`id` = ?", refund.Id).Take(&savedRefund)
	if savedRefund.TerraceId != "REFUND-D" {
		t.Fatalf("退款单号没落库: %q", savedRefund.TerraceId)
	}
}

// TestSuccessRefundOrderRejectsFailedRefund 网关明确失败时，
// SuccessRefundOrder 必须拒绝推进订单（虎皮椒 UD 曾经就这么把订单算成已退款）。
func TestSuccessRefundOrderRejectsFailedRefund(t *testing.T) {
	w := newZeroValueTestSite(t)
	orderId := testOrderId("gf")
	order := mustCreateTestOrder(t, w, orderId)
	refund := &model.OrderRefund{
		OrderId: orderId,
		Amount:  5000,
		Status:  config.OrderRefundStatusFailed,
	}
	if err := w.DB.Create(refund).Error; err != nil {
		t.Fatalf("造退款单失败: %v", err)
	}
	if err := w.SuccessRefundOrder(refund, order); err == nil {
		t.Fatal("退款失败状态不能推进订单到已退款")
	}
	var saved model.Order
	w.DB.Where("`id` = ?", order.Id).Take(&saved)
	if saved.Status == config.OrderStatusRefunded {
		t.Fatal("订单被失败退款改成了已退款")
	}
}

// TestDecryptWechatRefundNotify 微信退款通知的 req_info 是
// MD5(apiKey) 作 key 的 AES-256-ECB。这里用同一套规则加密再解密，
// 确保按微信协议能拿到 out_refund_no / refund_status。
func TestDecryptWechatRefundNotify(t *testing.T) {
	apiKey := "192006250b4c09247ec02edce69f6a2d"
	payload := `<root><out_trade_no>pay-1</out_trade_no><out_refund_no>r-1</out_refund_no>` +
		`<refund_id>5030000001</refund_id><refund_status>SUCCESS</refund_status>` +
		`<success_time>2026-11-12 10:00:00</success_time><refund_fee>5000</refund_fee><total_fee>5000</total_fee></root>`
	cipherText := wechatECBEncryptForTest(t, []byte(payload), apiKey)

	info, err := DecryptWechatRefundNotify(cipherText, apiKey)
	if err != nil {
		t.Fatalf("解密失败: %v", err)
	}
	if info["out_trade_no"] != "pay-1" || info["out_refund_no"] != "r-1" {
		t.Fatalf("单号解析错误: %#v", info)
	}
	if info["refund_status"] != "SUCCESS" || info["refund_id"] != "5030000001" {
		t.Fatalf("退款状态解析错误: %#v", info)
	}
	if wechatRefundState(info["refund_status"]) != refundSettleDone {
		t.Fatal("SUCCESS 应判为退款完成")
	}

	// 换一个密钥必须解不开（PKCS#7 校验会失败），避免「配错密钥也能过」
	if _, err = DecryptWechatRefundNotify(cipherText, "another-key"); err == nil {
		t.Fatal("错误的 api key 竟然解密成功")
	}
	if _, err = DecryptWechatRefundNotify("", apiKey); err == nil {
		t.Fatal("空 req_info 应报错")
	}
}

// wechatECBEncryptForTest 按微信的规则加密：MD5(apiKey) 的十六进制字符串作 AES-256 密钥，
// ECB 模式，PKCS#7 填充，base64 输出。
func wechatECBEncryptForTest(t *testing.T, plain []byte, apiKey string) string {
	t.Helper()
	sum := md5.Sum([]byte(apiKey))
	key := []byte(hex.EncodeToString(sum[:]))
	block, err := aes.NewCipher(key)
	if err != nil {
		t.Fatalf("new cipher: %v", err)
	}
	pad := aes.BlockSize - len(plain)%aes.BlockSize
	buf := append([]byte{}, plain...)
	buf = append(buf, []byte(strings.Repeat(string(rune(pad)), pad))...)
	out := make([]byte, len(buf))
	for i := 0; i < len(buf); i += aes.BlockSize {
		block.Encrypt(out[i:i+aes.BlockSize], buf[i:i+aes.BlockSize])
	}
	return base64.StdEncoding.EncodeToString(out)
}

// TestNotifyHandlersCheckRefundBeforePaidShortCircuit 源码级护栏。
//
// 微信/支付宝的退款通知对应的支付单一定 paid_time > 0，只要「已支付就 return success」
// 排在退款分支前面，退款结果就永远落不下来 —— 这正是本仓原来的样子。
func TestNotifyHandlersCheckRefundBeforePaidShortCircuit(t *testing.T) {
	raw, err := os.ReadFile("../controller/apiNotify.go")
	if err != nil {
		t.Fatalf("读 apiNotify.go 失败: %v", err)
	}
	src := string(raw)
	for _, fn := range []string{"func NotifyWechatPay(", "func NotifyAlipay("} {
		start := strings.Index(src, fn)
		if start < 0 {
			t.Fatalf("找不到 %s", fn)
		}
		body := src[start:]
		if end := strings.Index(body[10:], "\nfunc "); end > 0 {
			body = body[:end+10]
		}
		refundAt := strings.Index(body, "req_info")
		if refundAt < 0 {
			refundAt = strings.Index(body, "refund_status")
		}
		if refundAt < 0 {
			t.Fatalf("%s 里找不到退款通知分支", fn)
		}
		for _, short := range []string{"if payment.PaidTime > 0 {", "if order.PaidTime > 0 {"} {
			if i := strings.Index(body, short); i >= 0 && i < refundAt {
				t.Fatalf("%s: 退款分支之前出现了 %q，退款通知会被短路掉", fn, short)
			}
		}
	}
}

// TestNotifyHandlersUseMarkPaymentPaid 微信/支付宝的支付分支不能再自己写
// 「Save(payment) + Create(finance)」：那不是原子闸门，通知重发与前台轮询并发会记两笔。
func TestNotifyHandlersUseMarkPaymentPaid(t *testing.T) {
	raw, err := os.ReadFile("../controller/apiNotify.go")
	if err != nil {
		t.Fatalf("读 apiNotify.go 失败: %v", err)
	}
	src := string(raw)
	for _, fn := range []string{"func NotifyWechatPay(", "func NotifyAlipay("} {
		start := strings.Index(src, fn)
		body := src[start:]
		if end := strings.Index(body[10:], "\nfunc "); end > 0 {
			body = body[:end+10]
		}
		if !strings.Contains(body, "MarkPaymentPaid(") {
			t.Fatalf("%s 必须走 MarkPaymentPaid 的记账闸门", fn)
		}
		if strings.Contains(body, "FinanceActionBuy") {
			t.Fatalf("%s 里还有手写 finance，会绕过 paid_time 闸门", fn)
		}
	}
}

// TestStripeRefundSendsAmountAndIdempotencyKey 源码级护栏：
// 部分退款必须显式传 amount，且必须带幂等键，否则重试会退第二笔。
func TestStripeRefundSendsAmountAndIdempotencyKey(t *testing.T) {
	raw, err := os.ReadFile("refundGateway.go")
	if err != nil {
		t.Fatalf("读 refundGateway.go 失败: %v", err)
	}
	src := string(raw)
	idx := strings.Index(src, "func (w *Website) RefundStripePayment(")
	if idx < 0 {
		t.Fatal("找不到 RefundStripePayment")
	}
	body := src[idx:]
	if end := strings.Index(body[10:], "\nfunc "); end > 0 {
		body = body[:end+10]
	}
	if !strings.Contains(body, "params.Amount = stripe.Int64(refund.Amount)") {
		t.Fatal("Stripe 部分退款没有传 amount（会按全额退）")
	}
	if !strings.Contains(body, "SetIdempotencyKey(") {
		t.Fatal("Stripe 退款没有幂等键（重试会重复出金）")
	}
	if !strings.Contains(body, "refund.TerraceId = r.ID") {
		t.Fatal("Stripe 退款单号没有回写（事后无法对账/回查）")
	}
}

// TestWechatRefundUsesPaidAmount 微信退款的 total_fee 必须是实付金额。
// 用 order.Amount 时，改价/用券的订单会直接报「退款金额大于订单金额」。
func TestWechatRefundUsesPaidAmount(t *testing.T) {
	raw, err := os.ReadFile("refundGateway.go")
	if err != nil {
		t.Fatalf("读 refundGateway.go 失败: %v", err)
	}
	src := string(raw)
	idx := strings.Index(src, "func (w *Website) RefundWechatPayment(")
	if idx < 0 {
		t.Fatal("找不到 RefundWechatPayment")
	}
	body := src[idx:]
	if end := strings.Index(body[10:], "\nfunc "); end > 0 {
		body = body[:end+10]
	}
	if !strings.Contains(body, `Set("total_fee", payment.Amount)`) {
		t.Fatal("微信退款的 total_fee 应该用 payment.Amount（实付），不是 order.Amount")
	}
	if !strings.Contains(body, `Set("notify_url"`) {
		t.Fatal("微信退款必须显式传 notify_url，否则收不到退款结果通知")
	}
}

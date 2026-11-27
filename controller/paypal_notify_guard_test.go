package controller

import (
	"os"
	"strings"
	"testing"
)

// 锁住 2026-11-12 排查出的 PayPal 回调验签 bug：
//
// 老写法把通知 JSON 反序列化进 gopay.BodyMap，再让 SDK 序列化发去 verify-webhook-signature。
// PayPal 的签名里带 crc32(原始正文)，重新序列化（字段顺序/空白/转义都变）后 CRC 对不上，
// 真实通知 100% 验签失败 → HTTP 400 → 付款永远落不到订单上。
//
// 现在必须：原始 body + 验签头 + 该环境的 webhook_id，交给 provider.VerifyPaypalWebhook。

func TestNotifyPayPalVerifiesRawBody(t *testing.T) {
	src, err := os.ReadFile("apiNotify.go")
	if err != nil {
		t.Fatalf("读取 apiNotify.go 失败: %v", err)
	}
	text := string(src)

	if strings.Contains(text, "VerifyWebhookSignature(") {
		t.Fatal("不能用 SDK 的 VerifyWebhookSignature(BodyMap)：它会把 body 重新序列化，CRC 对不上")
	}
	if strings.Contains(text, `SetBodyMap("webhook_event"`) {
		t.Fatal("webhook_event 不能由反序列化后的 Map 重新拼出来")
	}
	if !strings.Contains(text, "currentSite.VerifyPaypalWebhook(account, headers, body)") {
		t.Fatal("NotifyPayPal 必须用原始 body 调 provider.VerifyPaypalWebhook")
	}
	if !strings.Contains(text, "provider.PaypalSignatureHeaders{") {
		t.Fatal("验签头必须从 PAYPAL-TRANSMISSION-* 请求头取")
	}

	// 反向对照：确认读到的确实是回调处理函数，断言没有空转
	if !strings.Contains(text, "func NotifyPayPal(ctx iris.Context)") {
		t.Fatal("断言空转：apiNotify.go 里找不到 NotifyPayPal")
	}
	if !strings.Contains(text, "currentSite.ProcessPaypalEvent(&event)") {
		t.Fatal("验签通过后必须同步处理事件")
	}
}

// 处理失败必须回非 2xx，否则 PayPal 认为投递成功、不再重投。
func TestNotifyPayPalReturnsErrorOnProcessFailure(t *testing.T) {
	src, err := os.ReadFile("apiNotify.go")
	if err != nil {
		t.Fatalf("读取 apiNotify.go 失败: %v", err)
	}
	text := string(src)

	if strings.Contains(text, "go currentSite.ProcessPaypalEvent") {
		t.Fatal("不能改用协程：先回 200 再异步处理，处理失败就永远补不回来")
	}
	idx := strings.Index(text, "if err = currentSite.ProcessPaypalEvent(&event); err != nil {")
	if idx < 0 {
		t.Fatal("处理失败必须显式判断并返回非 2xx")
	}
	// 失败分支里要出现 500
	block := text[idx:]
	if end := strings.Index(block, "\n}"); end > 0 {
		block = block[:end]
	}
	if !strings.Contains(block, "http.StatusInternalServerError") {
		t.Fatalf("处理失败分支必须返回 500 让 PayPal 重投，实际:\n%s", block)
	}
}

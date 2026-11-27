package provider

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"os"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/go-pay/gopay/paypal"
	"kandaoni.com/anqicms/config"
	"kandaoni.com/anqicms/library"
	"kandaoni.com/anqicms/model"
)

// ── PayPal 退款 / 回调验签 的护栏 ─────────────────────────────────────────
//
// 这三条都只在「真钱 + 真通知」的环境里才暴露，本地跑起来一切正常：
//
//	① 退款传的是 order id 而不是 capture id  → PayPal 404 "The specified resource does not exist"
//	② 验签前重新序列化通知 JSON              → CRC 变了，真实通知 100% 验签失败（HTTP 400）
//	③ 退款完成没有状态闸门                    → 后台退款 + REFUNDED 通知并发时记两笔流水

// TestBuildPaypalVerifyBodyPreservesRawEvent 锁住「原始正文原样进 webhook_event」。
func TestBuildPaypalVerifyBodyPreservesRawEvent(t *testing.T) {
	// 故意用非标准格式：字段顺序不是字母序、冒号后有空格、带换行
	raw := []byte("{\n  \"id\": \"WH-1\",\n  \"event_type\": \"PAYMENT.CAPTURE.COMPLETED\",\n  \"z_field\": 1,   \n  \"a_field\": \"x\"\n}")
	headers := PaypalSignatureHeaders{
		AuthAlgo:         "SHA256withRSA",
		CertUrl:          "https://api.paypal.com/cert.pem",
		TransmissionId:   "tid-1",
		TransmissionSig:  "sig-1",
		TransmissionTime: "2026-11-12T08:00:00Z",
	}
	body, err := BuildPaypalVerifyBody(headers, "WH-WEBHOOK-1", raw)
	if err != nil {
		t.Fatalf("拼验签请求体失败: %v", err)
	}
	if !strings.Contains(string(body), string(raw)) {
		t.Fatalf("webhook_event 必须原样保留收到的字节，实际请求体:\n%s", body)
	}
	if !json.Valid(body) {
		t.Fatalf("拼出来的请求体不是合法 JSON:\n%s", body)
	}

	// 反向对照：把同样的正文按老写法「反序列化再序列化」一遍，字节必然不同。
	// 这条断言证明上面的 Contains 不是空转 —— 两种写法真的会产生不同的 body。
	var m map[string]interface{}
	if err := json.Unmarshal(raw, &m); err != nil {
		t.Fatalf("测试数据不合法: %v", err)
	}
	reserialized, err := json.Marshal(m)
	if err != nil {
		t.Fatalf("重新序列化失败: %v", err)
	}
	if string(reserialized) == string(raw) {
		t.Fatal("反向对照失效：这份测试数据重新序列化后居然和原文一致，换一组带缩进/空格的数据")
	}
	if strings.Contains(string(body), string(reserialized)) {
		t.Fatal("请求体里出现的是重新序列化后的字节，不是原文")
	}
}

func TestBuildPaypalVerifyBodyRejectsBadInput(t *testing.T) {
	h := PaypalSignatureHeaders{AuthAlgo: "SHA256withRSA", CertUrl: "u", TransmissionId: "i", TransmissionSig: "s", TransmissionTime: "t"}
	if _, err := BuildPaypalVerifyBody(h, "", []byte(`{"id":"1"}`)); err == nil {
		t.Fatal("webhook_id 为空必须报错，否则 PayPal 只会回一个含糊的 FAILURE")
	}
	if _, err := BuildPaypalVerifyBody(h, "WH-1", nil); err == nil {
		t.Fatal("空正文必须报错")
	}
	if _, err := BuildPaypalVerifyBody(h, "WH-1", []byte("not-json")); err == nil {
		t.Fatal("非法 JSON 必须报错")
	}
	if !(PaypalSignatureHeaders{}).Empty() {
		t.Fatal("空 header 必须被判定为缺失")
	}
}

// TestVerifyPaypalWebhookSendsRawBody 用假 PayPal 服务端验证真实发出的请求。
func TestVerifyPaypalWebhookSendsRawBody(t *testing.T) {
	raw := []byte(`{"id":"WH-1","event_type":"PAYMENT.CAPTURE.COMPLETED","resource":{"id":"CAP-1"}}`)
	var gotBody, gotAuth string
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		buf := make([]byte, r.ContentLength)
		_, _ = r.Body.Read(buf)
		gotBody = string(buf)
		gotAuth = r.Header.Get("Authorization")
		_, _ = w.Write([]byte(`{"verification_status":"SUCCESS"}`))
	}))
	defer srv.Close()

	old := paypalVerifySignatureURL
	paypalVerifySignatureURL = func(bool) string { return srv.URL }
	defer func() { paypalVerifySignatureURL = old }()

	res, err := callPaypalVerifySignature(context.Background(), srv.URL, "token-1", []byte(`{"webhook_id":"WH","webhook_event":`+string(raw)+`}`))
	if err != nil {
		t.Fatalf("验签请求失败: %v", err)
	}
	if res.VerificationStatus != "SUCCESS" {
		t.Fatalf("状态取错: %q", res.VerificationStatus)
	}
	if !strings.Contains(gotBody, string(raw)) {
		t.Fatalf("服务端收到的 body 里没有原始事件字节: %s", gotBody)
	}
	if gotAuth != "Bearer token-1" {
		t.Fatalf("Authorization 头错误: %q", gotAuth)
	}

	// 反向对照：PayPal 回 FAILURE 时必须报错，否则伪造通知会被放行
	srv2 := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_, _ = w.Write([]byte(`{"verification_status":"FAILURE"}`))
	}))
	defer srv2.Close()
	res2, err := callPaypalVerifySignature(context.Background(), srv2.URL, "token-1", []byte(`{}`))
	if err != nil {
		t.Fatalf("HTTP 200 但状态 FAILURE 不应报传输错误: %v", err)
	}
	if res2.VerificationStatus == "SUCCESS" {
		t.Fatal("FAILURE 不能当成成功")
	}
}

// fakePaypal 模拟 PayPal 的 token / 订单详情 / 退款 三个接口。
type fakePaypal struct {
	srv        *httptest.Server
	refundHits int
	lastReqId  string
	lastPath   string
	refundRsp  string
	mu         sync.Mutex
}

func newFakePaypal(t *testing.T) *fakePaypal {
	t.Helper()
	f := &fakePaypal{refundRsp: `{"id":"REFUND-1","status":"COMPLETED"}`}
	f.srv = httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch {
		case strings.HasSuffix(r.URL.Path, "/v1/oauth2/token"):
			_, _ = w.Write([]byte(`{"access_token":"fake-token","token_type":"Bearer","expires_in":3600}`))
		case strings.Contains(r.URL.Path, "/v2/checkout/orders/"):
			// 订单详情：order id 是 ORDER-1，扣款凭证是 CAP-1
			_, _ = w.Write([]byte(`{"id":"ORDER-1","status":"COMPLETED","payer":{"payer_id":"PAYER-1"},
				"purchase_units":[{"payments":{"captures":[{"id":"CAP-1","status":"COMPLETED",
				"amount":{"value":"50.00","currency_code":"USD"}}]}}]}`))
		case strings.Contains(r.URL.Path, "/v2/payments/refunds/"):
			_, _ = w.Write([]byte(f.refundRsp))
		case strings.Contains(r.URL.Path, "/v2/payments/captures/"):
			f.mu.Lock()
			f.refundHits++
			f.lastReqId = r.Header.Get("PayPal-Request-Id")
			f.lastPath = r.URL.Path
			f.mu.Unlock()
			// 真实 PayPal 的行为：capture id 不存在就是 404 RESOURCE_NOT_FOUND。
			// 测试据此形成反向对照 —— 传 order id 进来必然 404。
			if !strings.Contains(r.URL.Path, "/captures/CAP-1/refund") {
				w.WriteHeader(http.StatusNotFound)
				_, _ = w.Write([]byte(`{"name":"RESOURCE_NOT_FOUND","message":"The specified resource does not exist."}`))
				return
			}
			_, _ = w.Write([]byte(f.refundRsp))
		default:
			w.WriteHeader(http.StatusNotFound)
			_, _ = w.Write([]byte(`{"name":"NOT_FOUND"}`))
		}
	}))
	t.Cleanup(f.srv.Close)
	old := paypalProxyURL
	paypalProxyURL = func() (string, string) { return f.srv.URL, f.srv.URL }
	t.Cleanup(func() { paypalProxyURL = old })
	return f
}

func paypalTestAccount() *model.PaymentAccount {
	return &model.PaymentAccount{
		Id:        1,
		PayWay:    config.PayWayPaypal,
		PayConfig: model.PayConfig{AppId: "client-id", AppSecret: "secret", Sandbox: true},
	}
}

func paypalTestClient(t *testing.T) *paypal.Client {
	t.Helper()
	c, err := newPaypalClient(paypalTestAccount())
	if err != nil {
		t.Fatalf("初始化 paypal client 失败: %v", err)
	}
	return c
}

// TestRefundPaypalUsesCaptureId 是这次修复的主断言：
// 退款必须打到 /v2/payments/captures/CAP-1/refund，而不是拿 terrace_id(ORDER-1) 去调。
func TestRefundPaypalUsesCaptureId(t *testing.T) {
	f := newFakePaypal(t)
	w := &Website{CachePath: t.TempDir() + "/"}
	payment := &model.Payment{
		Id:               1,
		PaymentId:        "PAY-1",
		TerraceId:        "ORDER-1", // 下单时写入的是 order id
		PayWay:           config.PayWayPaypal,
		Amount:           5000,
		PaidTime:         time.Now().Unix(),
		PaymentAccountId: 1,
	}
	refund := &model.OrderRefund{Id: 1, RefundId: "202611110340000065", OrderId: "ORD-1", Amount: 5000}

	completed, err := w.refundPaypalWithClient(context.Background(), paypalTestClient(t), payment, refund)
	if err != nil {
		t.Fatalf("退款失败: %v", err)
	}
	if !completed {
		t.Fatal("PayPal 回 COMPLETED 时应判定为已完成")
	}
	if !strings.Contains(f.lastPath, "/captures/CAP-1/refund") {
		t.Fatalf("退款打到了错误的路径（说明传的是 order id 不是 capture id）: %s", f.lastPath)
	}
	if refund.TerraceId != "REFUND-1" {
		t.Fatalf("应记下 PayPal 退款单号，实际 %q", refund.TerraceId)
	}
	if f.lastReqId != refund.RefundId {
		t.Fatalf("PayPal-Request-Id 必须复用退款单号（重试幂等），实际 %q", f.lastReqId)
	}
}

// TestRefundPaypalPendingIsNotCompleted PENDING 不能当成已完成，否则本地先标「已退款」，
// PayPal 那边还在处理，最后失败就变成「本地说退了、钱没退」。
func TestRefundPaypalPendingIsNotCompleted(t *testing.T) {
	f := newFakePaypal(t)
	f.refundRsp = `{"id":"REFUND-2","status":"PENDING"}`
	w := &Website{CachePath: t.TempDir() + "/"}
	payment := &model.Payment{Id: 2, PaymentId: "PAY-2", TerraceId: "ORDER-1", PayWay: config.PayWayPaypal,
		Amount: 5000, PaidTime: time.Now().Unix(), PaymentAccountId: 1}
	refund := &model.OrderRefund{Id: 2, RefundId: "R2", Amount: 5000}

	completed, err := w.refundPaypalWithClient(context.Background(), paypalTestClient(t), payment, refund)
	if err != nil {
		t.Fatalf("退款请求本身没失败: %v", err)
	}
	if completed {
		t.Fatal("PENDING 不能判定为已完成")
	}
	if refund.TerraceId != "REFUND-2" {
		t.Fatalf("PENDING 也要记下退款单号（后面靠它回查），实际 %q", refund.TerraceId)
	}
}

// TestRefundPaypalRetryDoesNotRefundTwice 重试复用同一个退款单：
// 已有退款单号时先查状态，绝不重复调退款接口。
func TestRefundPaypalRetryDoesNotRefundTwice(t *testing.T) {
	f := newFakePaypal(t)
	f.refundRsp = `{"id":"REFUND-3","status":"COMPLETED"}`
	w := &Website{CachePath: t.TempDir() + "/"}
	payment := &model.Payment{Id: 3, PaymentId: "PAY-3", TerraceId: "ORDER-1", PayWay: config.PayWayPaypal,
		Amount: 5000, PaidTime: time.Now().Unix(), PaymentAccountId: 1}
	refund := &model.OrderRefund{Id: 3, RefundId: "R3", Amount: 5000}

	client := paypalTestClient(t)
	if _, err := w.refundPaypalWithClient(context.Background(), client, payment, refund); err != nil {
		t.Fatalf("第一次退款失败: %v", err)
	}
	hits := f.refundHits
	if hits != 1 {
		t.Fatalf("第一次退款应只调用一次退款接口，实际 %d", hits)
	}
	// 第二次：refund.TerraceId 已有值 → 走状态回查
	completed, err := w.refundPaypalWithClient(context.Background(), paypalTestClient(t), payment, refund)
	if err != nil {
		t.Fatalf("重试失败: %v", err)
	}
	if !completed {
		t.Fatal("回查显示 COMPLETED 应判定为已完成")
	}
	if f.refundHits != hits {
		t.Fatalf("重试不能再次调用退款接口（会重复出金），实际累计 %d 次", f.refundHits)
	}
}

// TestRefundPaypalRejectsUnpaidAndOverAmount 参数闸门。
func TestRefundPaypalRejectsBadAmount(t *testing.T) {
	w := &Website{}
	payment := &model.Payment{Id: 4, PayWay: config.PayWayPaypal, Amount: 5000, PaidTime: time.Now().Unix()}
	if _, err := w.RefundPaypalPayment(context.Background(), payment, &model.OrderRefund{Amount: 6000}); err == nil {
		t.Fatal("退款金额大于支付金额必须拒绝")
	}
	payment.PaidTime = 0
	if _, err := w.RefundPaypalPayment(context.Background(), payment, &model.OrderRefund{Amount: 100}); err == nil {
		t.Fatal("未支付的支付单必须拒绝退款")
	}
}

func TestYuanToFen(t *testing.T) {
	cases := map[string]int64{"50.00": 5000, "50": 5000, "0.01": 1, "1234.56": 123456}
	for in, want := range cases {
		if got, ok := yuanToFen(in); !ok || got != want {
			t.Fatalf("%q 应解析为 %d 分，实际 %d (ok=%v)", in, want, got, ok)
		}
	}
	for _, bad := range []string{"", "abc", " "} {
		if _, ok := yuanToFen(bad); ok {
			t.Fatalf("%q 不该解析成功", bad)
		}
	}
}

// ── 回调去重 ────────────────────────────────────────────────────────────

func TestIsDuplicateEvent(t *testing.T) {
	w := &Website{Cache: library.InitMemoryCache()}
	if w.isDuplicateEvent("") {
		t.Fatal("空事件 ID 不能算重复（否则所有无 ID 的通知都被吞掉）")
	}
	if w.isDuplicateEvent("WH-EVT-1") {
		t.Fatal("第一次出现不算重复")
	}
	// 只有「处理成功」才写标记：处理失败就标上，PayPal 重投会被当成重复直接吞掉
	w.markEventProcessed("WH-EVT-1")
	if !w.isDuplicateEvent("WH-EVT-1") {
		t.Fatal("处理成功后再来必须判为重复 —— 否则这个函数就是永远返回 false")
	}
}

// TestFailedEventCanBeRetried 处理失败不能把事件标成已处理，否则重投永远补不回来。
func TestFailedEventCanBeRetried(t *testing.T) {
	w := &Website{Cache: library.InitMemoryCache()}
	// 一条定位不到支付单的事件 → handlePaypalEvent 返回 error
	ev := &paypal.WebhookEvent{
		Id:        "WH-EVT-FAIL",
		EventType: "PAYMENT.CAPTURE.COMPLETED",
		Resource:  json.RawMessage(`{"id":"CAP-NOT-EXIST","supplementary_data":{"related_ids":{"order_id":"ORDER-NOT-EXIST"}}}`),
	}
	if w.DB == nil {
		// 没有库时连查询都做不了，直接跳过（这条用例只在有库的环境下有意义）
		realW := newZeroValueTestSite(t)
		realW.Cache = w.Cache
		realW.CachePath = t.TempDir() + "/"
		w = realW
	}
	if err := w.ProcessPaypalEvent(ev); err == nil {
		t.Fatal("定位不到支付单必须返回 error（调用方据此回非 2xx）")
	}
	if w.isDuplicateEvent(ev.Id) {
		t.Fatal("处理失败不能写去重标记，否则 PayPal 重投会被吞掉")
	}
}

// ── 需要真实库的幂等断言 ─────────────────────────────────────────────────

// testOrderId 生成 <=32 字符的订单号：finances.order_id 只有 varchar(32)，
// 用纳秒时间戳拼长前缀会直接 Data too long。
func testOrderId(prefix string) string {
	return fmt.Sprintf("%s%d", prefix, time.Now().UnixNano()%1e12)
}

func testPaymentId(prefix string) string {
	return fmt.Sprintf("%s%d", prefix, time.Now().UnixNano()%1e12)
}

func mustCreateTestOrder(t *testing.T, w *Website, orderId string) *model.Order {
	t.Helper()
	order := &model.Order{
		OrderId:  orderId,
		Amount:   5000,
		Status:   config.OrderStatusPaid,
		PaidTime: time.Now().Unix(),
	}
	if err := w.DB.Create(order).Error; err != nil {
		t.Fatalf("造订单失败: %v", err)
	}
	t.Cleanup(func() {
		w.DB.Unscoped().Where("`order_id` = ?", orderId).Delete(&model.Order{})
		w.DB.Unscoped().Where("`order_id` = ?", orderId).Delete(&model.OrderRefund{})
		w.DB.Unscoped().Where("`order_id` = ?", orderId).Delete(&model.Finance{})
	})
	return order
}

// TestSuccessRefundOrderIsIdempotent 后台退款与 REFUNDED 通知同时到达时，
// 只能推进一次订单、只能记一笔退款流水。
func TestSuccessRefundOrderIsIdempotent(t *testing.T) {
	w := newZeroValueTestSite(t)
	orderId := testOrderId("ri")
	order := mustCreateTestOrder(t, w, orderId)

	refund := &model.OrderRefund{
		OrderId: orderId,
		Amount:  order.Amount,
		Status:  config.OrderRefundStatusWaiting,
	}
	if err := w.DB.Create(refund).Error; err != nil {
		t.Fatalf("造退款单失败: %v", err)
	}

	var wg sync.WaitGroup
	for i := 0; i < 2; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			// 两个并发调用各自持有一份对象副本，模拟后台退款 + 通知两条独立路径
			r := &model.OrderRefund{Id: refund.Id, OrderId: orderId, Amount: order.Amount, Status: config.OrderRefundStatusWaiting}
			o := &model.Order{Id: order.Id, OrderId: orderId, Amount: order.Amount, Status: config.OrderStatusPaid, UserId: order.UserId}
			if err := w.SuccessRefundOrder(r, o); err != nil {
				t.Errorf("退款收尾失败: %v", err)
			}
		}()
	}
	wg.Wait()

	var saved model.Order
	if err := w.DB.Where("`order_id` = ?", orderId).Take(&saved).Error; err != nil {
		t.Fatalf("读回订单失败: %v", err)
	}
	if saved.Status != config.OrderStatusRefunded {
		t.Fatalf("订单应变为已退款，实际 status=%d", saved.Status)
	}

	var financeCount int64
	w.DB.Model(&model.Finance{}).Where("`order_id` = ? AND `action` = ?", orderId, config.FinanceActionRefund).Count(&financeCount)
	if financeCount != 1 {
		t.Fatalf("退款流水必须只有 1 笔，实际 %d 笔（状态闸门失效）", financeCount)
	}

	var savedRefund model.OrderRefund
	if err := w.DB.Where("`id` = ?", refund.Id).Take(&savedRefund).Error; err != nil {
		t.Fatalf("读回退款单失败: %v", err)
	}
	if savedRefund.Status != config.OrderRefundStatusDone {
		t.Fatalf("退款单应已完成，实际 status=%d", savedRefund.Status)
	}

	// 第三次（串行）再调一次也不能多记
	if err := w.SuccessRefundOrder(&model.OrderRefund{Id: refund.Id, OrderId: orderId, Amount: order.Amount}, &model.Order{Id: order.Id, OrderId: orderId}); err != nil {
		t.Fatalf("重复调用应幂等返回 nil: %v", err)
	}
	w.DB.Model(&model.Finance{}).Where("`order_id` = ? AND `action` = ?", orderId, config.FinanceActionRefund).Count(&financeCount)
	if financeCount != 1 {
		t.Fatalf("串行重试后又多了流水，实际 %d 笔", financeCount)
	}
}

// TestMarkPaymentPaidSkipsRefundedOrder 迟到的付款通知不能把已退款订单改回已付款。
func TestMarkPaymentPaidSkipsRefundedOrder(t *testing.T) {
	w := newZeroValueTestSite(t)
	orderId := testOrderId("lp")
	order := mustCreateTestOrder(t, w, orderId)
	w.DB.Model(&model.Order{}).Where("id = ?", order.Id).Updates(map[string]interface{}{"status": config.OrderStatusRefunded})

	payment := &model.Payment{
		PaymentId: testPaymentId("p"),
		OrderId:   orderId,
		Amount:    order.Amount,
		PayWay:    config.PayWayPaypal,
	}
	if err := w.DB.Create(payment).Error; err != nil {
		t.Fatalf("造支付单失败: %v", err)
	}
	t.Cleanup(func() { w.DB.Unscoped().Where("`order_id` = ?", orderId).Delete(&model.Payment{}) })

	if err := w.MarkPaymentPaid(payment, "ORDER-1", "PAYER-1", ""); err != nil {
		t.Fatalf("迟到的通知应被安静地忽略（不能报错让 PayPal 无限重投）: %v", err)
	}

	var saved model.Payment
	if err := w.DB.Where("`id` = ?", payment.Id).Take(&saved).Error; err != nil {
		t.Fatalf("读回支付单失败: %v", err)
	}
	if saved.PaidTime != 0 {
		t.Fatal("已退款订单的支付单不能被标记为已支付")
	}
	var savedOrder model.Order
	w.DB.Where("`id` = ?", order.Id).Take(&savedOrder)
	if savedOrder.Status != config.OrderStatusRefunded {
		t.Fatalf("已退款订单状态被改回 %d", savedOrder.Status)
	}
	var financeCount int64
	w.DB.Model(&model.Finance{}).Where("`order_id` = ? AND `action` = ?", orderId, config.FinanceActionBuy).Count(&financeCount)
	if financeCount != 0 {
		t.Fatalf("不能为迟到通知记付款流水，实际 %d 笔", financeCount)
	}
}

// ── 源码级护栏：调用点正确才算真修好 ──────────────────────────────────────

// TestSetOrderRefundUsesPaypalHelper 禁止 SetOrderRefund 再直接拿 payment.TerraceId 调退款接口。
func TestSetOrderRefundUsesPaypalHelper(t *testing.T) {
	src, err := os.ReadFile("order.go")
	if err != nil {
		t.Fatalf("读取 order.go 失败: %v", err)
	}
	text := string(src)
	if strings.Contains(text, "PaymentCaptureRefund(") && !strings.Contains(text, "captureId") {
		t.Fatal("order.go 里不能直接调 PaymentCaptureRefund，必须走 RefundPaypalPayment（内部解析 capture id）")
	}
	if !strings.Contains(text, "w.RefundPaypalPayment(") {
		t.Fatal("SetOrderRefund 的 PayPal 分支必须调用 RefundPaypalPayment")
	}
	// 反向对照：确认这两条断言盯的文件确实包含退款主流程（而不是读错文件后的空转）
	if !strings.Contains(text, "func (w *Website) SetOrderRefund(") {
		t.Fatal("断言空转：order.go 里找不到 SetOrderRefund")
	}
}

// TestPaypalRefundSuccessUsesSdkCode 成功判定必须用 paypal.Success(0)，
// 用 HTTP 200/201 判定会永远走失败分支（SDK 在 2xx 时把 Code 置为 Success）。
func TestPaypalRefundSuccessUsesSdkCode(t *testing.T) {
	src, err := os.ReadFile("paypalPayment.go")
	if err != nil {
		t.Fatalf("读取 paypalPayment.go 失败: %v", err)
	}
	text := string(src)
	if !strings.Contains(text, "ppRsp.Code != paypal.Success") {
		t.Fatal("退款成功判定必须比对 paypal.Success")
	}
	if strings.Contains(text, "http.StatusCreated") {
		t.Fatal("不能再用 HTTP 201/200 判断退款成功")
	}
	if !strings.Contains(text, "paypalRequestIdHeader") {
		t.Fatal("退款请求必须带 PayPal-Request-Id（重试幂等）")
	}
}

// TestSetOrderRefundPaypalEndToEnd 走一遍「后台点退款」的真实路径：
// 建支付账号（指向假 PayPal）→ SetOrderRefund(order, 1) → 订单已退款 + 一笔退款流水。
func TestSetOrderRefundPaypalEndToEnd(t *testing.T) {
	newFakePaypal(t)
	w := newZeroValueTestSite(t)
	w.CachePath = t.TempDir() + "/"

	account := &model.PaymentAccount{
		PayWay:    config.PayWayPaypal,
		PayConfig: model.PayConfig{AppId: "client-id", AppSecret: "secret", Sandbox: true, WebhookId: "WH-1"},
		Status:    1,
	}
	if err := w.DB.Create(account).Error; err != nil {
		t.Fatalf("造支付账号失败: %v", err)
	}
	t.Cleanup(func() { w.DB.Unscoped().Delete(&model.PaymentAccount{}, account.Id) })

	orderId := testOrderId("ar")
	order := mustCreateTestOrder(t, w, orderId)
	payment := &model.Payment{
		PaymentId:        testPaymentId("p"),
		OrderId:          orderId,
		TerraceId:        "ORDER-1",
		Amount:           order.Amount,
		PayWay:           config.PayWayPaypal,
		PaidTime:         time.Now().Unix(),
		PaymentAccountId: account.Id,
	}
	if err := w.DB.Create(payment).Error; err != nil {
		t.Fatalf("造支付单失败: %v", err)
	}
	t.Cleanup(func() { w.DB.Unscoped().Where("`order_id` = ?", orderId).Delete(&model.Payment{}) })
	w.DB.Model(&model.Order{}).Where("id = ?", order.Id).Update("payment_id", payment.PaymentId)
	order.PaymentId = payment.PaymentId

	if err := w.SetOrderRefund(order, 1); err != nil {
		t.Fatalf("后台退款失败: %v", err)
	}

	var saved model.Order
	w.DB.Where("`id` = ?", order.Id).Take(&saved)
	if saved.Status != config.OrderStatusRefunded {
		t.Fatalf("订单未变为已退款，status=%d", saved.Status)
	}
	var refund model.OrderRefund
	if err := w.DB.Where("`order_id` = ?", orderId).Take(&refund).Error; err != nil {
		t.Fatalf("没有生成退款单: %v", err)
	}
	if refund.Status != config.OrderRefundStatusDone {
		t.Fatalf("退款单未完成，status=%d", refund.Status)
	}
	if refund.TerraceId != "REFUND-1" {
		t.Fatalf("退款单号没落库: %q", refund.TerraceId)
	}
	var financeCount int64
	w.DB.Model(&model.Finance{}).Where("`order_id` = ? AND `action` = ?", orderId, config.FinanceActionRefund).Count(&financeCount)
	if financeCount != 1 {
		t.Fatalf("退款流水应为 1 笔，实际 %d", financeCount)
	}
}

// TestHandlePaypalCaptureRefundedCompletesOnce PayPal 的 REFUNDED 通知（没有本地申请单）
// 要把订单/退款单/流水补齐，重复投递不能多记。
func TestHandlePaypalCaptureRefundedCompletesOnce(t *testing.T) {
	w := newZeroValueTestSite(t)
	w.CachePath = t.TempDir() + "/"

	orderId := testOrderId("nr")
	order := mustCreateTestOrder(t, w, orderId)
	payment := &model.Payment{
		PaymentId: testPaymentId("p"),
		OrderId:   orderId,
		TerraceId: "ORDER-1",
		Amount:    order.Amount,
		PayWay:    config.PayWayPaypal,
		PaidTime:  time.Now().Unix(),
	}
	if err := w.DB.Create(payment).Error; err != nil {
		t.Fatalf("造支付单失败: %v", err)
	}
	t.Cleanup(func() { w.DB.Unscoped().Where("`order_id` = ?", orderId).Delete(&model.Payment{}) })

	// 真实的 PAYMENT.CAPTURE.REFUNDED resource 片段（capture id + 已退金额 + 关联 order id）
	raw := []byte(`{"id":"CAP-1","status":"REFUNDED",
		"amount":{"total":"50.00","currency":"USD"},
		"seller_receivable_breakdown":{"total_refunded_amount":{"value":"50.00","currency_code":"USD"}},
		"supplementary_data":{"related_ids":{"order_id":"ORDER-1"}}}`)
	var res PaypalWebhookResource
	if err := json.Unmarshal(raw, &res); err != nil {
		t.Fatalf("解析 resource 失败: %v", err)
	}
	if p, err := w.GetPaymentInfoByPaypalResource(&res); err != nil || p.PaymentId != payment.PaymentId {
		t.Fatalf("PAYMENT.CAPTURE.* 事件没定位到支付单: %v", err)
	}

	for i := 0; i < 2; i++ { // 第二次 = PayPal 重投
		if err := w.handlePaypalCaptureRefunded(payment, &res); err != nil {
			t.Fatalf("第 %d 次处理退款通知失败: %v", i+1, err)
		}
	}

	var saved model.Order
	w.DB.Where("`id` = ?", order.Id).Take(&saved)
	if saved.Status != config.OrderStatusRefunded {
		t.Fatalf("订单未变为已退款，status=%d", saved.Status)
	}
	var refund model.OrderRefund
	if err := w.DB.Where("`order_id` = ?", orderId).Take(&refund).Error; err != nil {
		t.Fatalf("退款通知应自动补出退款单: %v", err)
	}
	if refund.Amount != 5000 {
		t.Fatalf("退款金额应取自 total_refunded_amount(50.00)，实际 %d", refund.Amount)
	}
	var financeCount int64
	w.DB.Model(&model.Finance{}).Where("`order_id` = ? AND `action` = ?", orderId, config.FinanceActionRefund).Count(&financeCount)
	if financeCount != 1 {
		t.Fatalf("重复通知不能多记流水，实际 %d 笔", financeCount)
	}
}

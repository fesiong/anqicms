package provider

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"strings"
	"time"

	"github.com/go-pay/gopay"
	"github.com/go-pay/gopay/paypal"
	"github.com/go-pay/xlog"
	"gorm.io/gorm"
	"kandaoni.com/anqicms/config"
	"kandaoni.com/anqicms/library"
	"kandaoni.com/anqicms/model"
)

// PayPal 支付链路的三个坑（都只在真钱真通知的环境里才暴露）：
//
//	① 退款必须用 capture id：payment.terrace_id 存的是下单时的 order id，
//	   /v2/payments/captures/{capture_id}/refund 只认 capture id，
//	   传 order id 会得到 404 "The specified resource does not exist"；
//	② 回调验签必须用 PayPal 发来的**原始请求体**：PayPal 的签名是对
//	   transmissionId|transmissionTime|webhookId|crc32(rawBody) 做的，把 JSON 反序列化
//	   再序列化一次（字段顺序/空白/转义都会变）CRC 就对不上，验签永远失败；
//	③ 退款完成要幂等：后台退款和 PAYMENT.CAPTURE.REFUNDED 通知会同时到达，
//	   只有拿到「状态迁移」的那一次才允许写退款流水。

const (
	paypalProdBaseURL    = "https://api-m.paypal.com"
	paypalSandboxBaseURL = "https://api-m.sandbox.paypal.com"
	// PayPal-Request-Id 是 PayPal 的幂等键：同一笔退款重试必须复用同一个值，
	// 否则网络超时后的重试会被当成一笔新退款，重复出金。
	paypalRequestIdHeader = "PayPal-Request-Id"
	paypalVerifyPath      = "/v1/notify-webhook/verify-webhook-signature"
)

func paypalBaseURL(isProd bool) string {
	if isProd {
		return paypalProdBaseURL
	}
	return paypalSandboxBaseURL
}

// paypalVerifySignatureURL 单独抽成变量，测试里替换成 httptest 地址。
var paypalVerifySignatureURL = func(isProd bool) string {
	return paypalBaseURL(isProd) + paypalVerifyPath
}

// paypalProxyURL 让测试把 SDK 指到本地 httptest 服务（生产为 nil，走官方域名）。
var paypalProxyURL func() (prod, sandbox string)

func newPaypalClient(account *model.PaymentAccount) (*paypal.Client, error) {
	if account == nil {
		return nil, errors.New("payment account is nil")
	}
	if account.PayConfig.AppId == "" || account.PayConfig.AppSecret == "" {
		return nil, errors.New("paypal client_id / client_secret 未配置")
	}
	// WithoutAutoRefreshToken：这里的 client 是一次性的，自动刷新会留下一个永不退出的 goroutine。
	opts := []paypal.Option{paypal.WithoutAutoRefreshToken()}
	if paypalProxyURL != nil {
		if prod, sandbox := paypalProxyURL(); prod != "" || sandbox != "" {
			opts = append(opts, paypal.WithProxyUrl(prod, sandbox))
		}
	}
	return paypal.NewClient(account.PayConfig.AppId, account.PayConfig.AppSecret,
		!account.PayConfig.Sandbox, opts...)
}

// ensurePaypalColumns 给老库补 capture_id / order_refunds.terrace_id。
//
// 不能只靠 AutoMigrateDB：它只在 last_run_version != config.Version 时执行（升级才跑），
// 而这两个字段在退款链路上是「缺了就报 Unknown column」的硬依赖，所以放 InitModelData 里
// 每次初始化都幂等补一次。
func (w *Website) ensurePaypalColumns() error {
	if w.DB == nil {
		return nil
	}
	if w.DB.Migrator().HasTable(&model.Payment{}) && !w.DB.Migrator().HasColumn(&model.Payment{}, "capture_id") {
		if err := w.DB.Exec("ALTER TABLE `payments` ADD COLUMN `capture_id` varchar(64) NOT NULL DEFAULT ''").Error; err != nil {
			return err
		}
	}
	if w.DB.Migrator().HasTable(&model.OrderRefund{}) && !w.DB.Migrator().HasColumn(&model.OrderRefund{}, "terrace_id") {
		if err := w.DB.Exec("ALTER TABLE `order_refunds` ADD COLUMN `terrace_id` varchar(64) NOT NULL DEFAULT ''").Error; err != nil {
			return err
		}
	}
	return nil
}

// ── 回调验签 ────────────────────────────────────────────────────────────

// PaypalSignatureHeaders PayPal 回调的验签头（注意是单 n 的 Paypal，与网关头名一致）。
type PaypalSignatureHeaders struct {
	AuthAlgo         string
	CertUrl          string
	TransmissionId   string
	TransmissionSig  string
	TransmissionTime string
}

// Empty 判断验签头是否齐全。缺任何一个都不要发请求——PayPal 只会回一个含糊的 FAILURE。
func (h PaypalSignatureHeaders) Empty() bool {
	return h.AuthAlgo == "" || h.CertUrl == "" || h.TransmissionId == "" ||
		h.TransmissionSig == "" || h.TransmissionTime == ""
}

// BuildPaypalVerifyBody 拼 /v1/notify-webhook/verify-webhook-signature 的请求体。
//
// webhook_event 必须是**原始字节**：PayPal 的签名里带着 crc32(原始通知正文)，
// 这里如果塞反序列化后的结构体/Map，JSON 会被重新编码（map 还会随机化字段顺序），
// CRC 立刻对不上，验签 100% 失败。所以整个 body 手拼，事件部分原样拷贝。
func BuildPaypalVerifyBody(h PaypalSignatureHeaders, webhookId string, rawEvent []byte) ([]byte, error) {
	if webhookId == "" {
		return nil, errors.New("webhook_id 为空：请在支付账号里填写当前环境（沙盒/正式）的 PayPal Webhook ID")
	}
	if len(rawEvent) == 0 {
		return nil, errors.New("原始通知正文为空，无法验签")
	}
	if !json.Valid(rawEvent) {
		return nil, errors.New("原始通知正文不是合法 JSON")
	}
	// 去掉前后空白，避免把 PayPal 的换行拼进 webhook_event 造成字节偏移。
	rawEvent = bytes.TrimSpace(rawEvent)

	var buf bytes.Buffer
	buf.WriteString(`{"auth_algo":`)
	buf.Write(mustJSONString(h.AuthAlgo))
	buf.WriteString(`,"cert_url":`)
	buf.Write(mustJSONString(h.CertUrl))
	buf.WriteString(`,"transmission_id":`)
	buf.Write(mustJSONString(h.TransmissionId))
	buf.WriteString(`,"transmission_sig":`)
	buf.Write(mustJSONString(h.TransmissionSig))
	buf.WriteString(`,"transmission_time":`)
	buf.Write(mustJSONString(h.TransmissionTime))
	buf.WriteString(`,"webhook_id":`)
	buf.Write(mustJSONString(webhookId))
	buf.WriteString(`,"webhook_event":`)
	buf.Write(rawEvent)
	buf.WriteByte('}')
	return buf.Bytes(), nil
}

func mustJSONString(s string) []byte {
	b, err := json.Marshal(s)
	if err != nil {
		return []byte(`""`)
	}
	return b
}

type paypalVerifyResponse struct {
	VerificationStatus string `json:"verification_status"`
}

// callPaypalVerifySignature 直接发 HTTP，不用 SDK 的 BodyMap（见 BuildPaypalVerifyBody 的注释）。
func callPaypalVerifySignature(ctx context.Context, verifyURL, accessToken string, body []byte) (*paypalVerifyResponse, error) {
	if accessToken == "" {
		return nil, errors.New("paypal access token 为空")
	}
	reqCtx, cancel := context.WithTimeout(ctx, 20*time.Second)
	defer cancel()

	req, err := http.NewRequestWithContext(reqCtx, http.MethodPost, verifyURL, bytes.NewReader(body))
	if err != nil {
		return nil, err
	}
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Accept", "application/json")
	req.Header.Set("Authorization", "Bearer "+accessToken)

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	raw, err := io.ReadAll(io.LimitReader(resp.Body, 1<<20))
	if err != nil {
		return nil, err
	}
	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("paypal verify signature http %d: %s", resp.StatusCode, strings.TrimSpace(string(raw)))
	}
	var out paypalVerifyResponse
	if err := json.Unmarshal(raw, &out); err != nil {
		return nil, fmt.Errorf("paypal verify signature 响应解析失败: %w, body: %s", err, string(raw))
	}
	return &out, nil
}

// VerifyPaypalWebhook 用原始正文 + 验签头 + 该环境的 webhook_id 验签。
func (w *Website) VerifyPaypalWebhook(account *model.PaymentAccount, headers PaypalSignatureHeaders, rawEvent []byte) error {
	if headers.Empty() {
		return errors.New("缺少 PayPal 验签头（PAYPAL-TRANSMISSION-*）")
	}
	if account == nil {
		return errors.New("payment account is nil")
	}
	webhookId := account.PayConfig.WebhookId
	body, err := BuildPaypalVerifyBody(headers, webhookId, rawEvent)
	if err != nil {
		return err
	}
	client, err := newPaypalClient(account)
	if err != nil {
		return err
	}
	res, err := callPaypalVerifySignature(context.Background(), paypalVerifySignatureURL(!account.PayConfig.Sandbox), client.AccessToken, body)
	if err != nil {
		return err
	}
	if res.VerificationStatus != "SUCCESS" {
		return fmt.Errorf("paypal webhook verify failed: status=%q (sandbox=%v, webhook_id=%s)",
			res.VerificationStatus, account.PayConfig.Sandbox, webhookId)
	}
	return nil
}

// ── 退款 ────────────────────────────────────────────────────────────────

// paypalRefundRequestId 同一笔退款（同一 order_refunds 行）重试复用同一个值。
func paypalRefundRequestId(refund *model.OrderRefund) string {
	if refund.RefundId != "" {
		return refund.RefundId
	}
	return fmt.Sprintf("anqicms-refund-%d", refund.Id)
}

// firstPaypalCaptureId 从订单详情里取 capture id：优先已完成的，其次第一个非空的。
func firstPaypalCaptureId(detail *paypal.OrderDetail) string {
	if detail == nil {
		return ""
	}
	var fallback string
	for _, pu := range detail.PurchaseUnits {
		if pu == nil || pu.Payments == nil {
			continue
		}
		for _, c := range pu.Payments.Captures {
			if c == nil || c.Id == "" {
				continue
			}
			if c.Status == "COMPLETED" || c.Status == "PARTIALLY_REFUNDED" {
				return c.Id
			}
			if fallback == "" {
				fallback = c.Id
			}
		}
	}
	return fallback
}

func (w *Website) savePaypalCaptureId(payment *model.Payment, captureId string) {
	if payment == nil {
		return
	}
	if captureId == "" || payment.CaptureId == captureId {
		return
	}
	payment.CaptureId = captureId
	if payment.Id > 0 && w.DB != nil {
		w.DB.Model(&model.Payment{}).Where("id = ?", payment.Id).UpdateColumn("capture_id", captureId)
	}
}

// ResolvePaypalCaptureId 拿到退款要用的 capture id：
// 先看库里存的，再回查订单详情（必要时先 capture 一次），最后落库。
func (w *Website) ResolvePaypalCaptureId(ctx context.Context, client *paypal.Client, payment *model.Payment) (string, error) {
	if payment == nil {
		return "", errors.New("payment is nil")
	}
	if payment.CaptureId != "" {
		return payment.CaptureId, nil
	}
	if payment.TerraceId == "" {
		return "", errors.New("支付单没有 terrace_id（PayPal order id），无法定位扣款")
	}
	ppRsp, err := client.OrderDetail(ctx, payment.TerraceId, make(gopay.BodyMap))
	if err != nil {
		return "", fmt.Errorf("查询 PayPal 订单失败（order_id=%s）：%w", payment.TerraceId, err)
	}
	if ppRsp.Code != paypal.Success || ppRsp.Response == nil {
		return "", fmt.Errorf("查询 PayPal 订单失败（order_id=%s）：%s", payment.TerraceId, ppRsp.Error)
	}
	detail := ppRsp.Response
	if detail.Status == "APPROVED" {
		// 用户已授权但还没扣款：先 capture，钱到账后才能退
		captureRsp, cErr := client.OrderCapture(ctx, payment.TerraceId, nil)
		if cErr != nil {
			return "", fmt.Errorf("capture PayPal 订单失败（order_id=%s）：%w", payment.TerraceId, cErr)
		}
		if captureRsp.Code == paypal.Success && captureRsp.Response != nil {
			detail = captureRsp.Response
		}
	}
	captureId := firstPaypalCaptureId(detail)
	if captureId == "" {
		return "", fmt.Errorf("PayPal 订单 %s 里没有 capture（status=%s），可能尚未扣款", payment.TerraceId, detail.Status)
	}
	w.savePaypalCaptureId(payment, captureId)
	return captureId, nil
}

// syncPaypalRefund 用退款单号回查状态。
// 返回 done=是否已完成，retry=是否应该重新发起退款。
func (w *Website) syncPaypalRefund(ctx context.Context, client *paypal.Client, refund *model.OrderRefund) (done bool, retry bool, err error) {
	ppRsp, err := client.PaymentRefundDetail(ctx, refund.TerraceId)
	if err != nil {
		return false, false, err
	}
	if ppRsp.Code == http.StatusNotFound {
		// 查不到这笔退款单：说明上次其实没建成，可以重新发起
		return false, true, nil
	}
	if ppRsp.Code != paypal.Success || ppRsp.Response == nil {
		// 401/500 这类不能当成「没建成」去重发退款，否则会重复出金
		return false, false, fmt.Errorf("查询 PayPal 退款单失败（refund_id=%s）：%s", refund.TerraceId, ppRsp.Error)
	}
	switch ppRsp.Response.Status {
	case "COMPLETED":
		return true, false, nil
	case "PENDING":
		return false, false, nil
	default: // CANCELLED / FAILED
		return false, true, nil
	}
}

// RefundPaypalPayment 用 capture id 向 PayPal 发起退款。
//
// 返回 completed=false 表示 PayPal 已受理但还在 PENDING，此时**不能**把本地退款标成已完成，
// 等 PAYMENT.CAPTURE.REFUNDED 通知或下次重试收尾。
func (w *Website) RefundPaypalPayment(ctx context.Context, payment *model.Payment, refund *model.OrderRefund) (completed bool, err error) {
	if payment == nil || refund == nil {
		return false, errors.New("payment or refund is nil")
	}
	if payment.PayWay != config.PayWayPaypal {
		return false, fmt.Errorf("payment %s 不是 PayPal 支付", payment.PaymentId)
	}
	if payment.PaidTime == 0 {
		return false, fmt.Errorf("支付单 %s 未支付，无法退款", payment.PaymentId)
	}
	if refund.Amount <= 0 || refund.Amount > payment.Amount {
		return false, fmt.Errorf("退款金额 %d 非法（支付金额 %d）", refund.Amount, payment.Amount)
	}
	account, err := w.GetPaymentAccountById(payment.PaymentAccountId)
	if err != nil {
		return false, err
	}
	client, err := newPaypalClient(account)
	if err != nil {
		return false, err
	}
	return w.refundPaypalWithClient(ctx, client, payment, refund)
}

// refundPaypalWithClient 退款主体（拆出来是为了让单测能直接喂一个指向假 PayPal 的 client）。
func (w *Website) refundPaypalWithClient(ctx context.Context, client *paypal.Client, payment *model.Payment, refund *model.OrderRefund) (completed bool, err error) {
	// 已经向 PayPal 发起过（有退款单号）：先查状态，别重复调退款接口
	if refund.TerraceId != "" {
		done, retry, sErr := w.syncPaypalRefund(ctx, client, refund)
		if sErr != nil {
			return false, sErr
		}
		if !retry {
			return done, nil
		}
	}

	captureId, err := w.ResolvePaypalCaptureId(ctx, client, payment)
	if err != nil {
		return false, err
	}

	bm := make(gopay.BodyMap)
	if refund.Amount < payment.Amount {
		// 部分退款必须显式带金额；全额退款不带 amount，由 PayPal 按 capture 金额退
		bm.SetBodyMap("amount", func(b gopay.BodyMap) {
			b.Set("currency_code", "USD").
				Set("value", fmt.Sprintf("%.2f", float64(refund.Amount)/100))
		})
	}
	if note := truncateRunes(strings.TrimSpace(strings.ReplaceAll(refund.Remark, "%", "")), 120); note != "" {
		bm.Set("note_to_payer", note)
	}

	client.SetRequestHeader(paypalRequestIdHeader, paypalRefundRequestId(refund))
	ppRsp, err := client.PaymentCaptureRefund(ctx, captureId, bm)
	if err != nil {
		return false, fmt.Errorf("PayPal 退款请求失败（capture_id=%s）：%w", captureId, err)
	}
	// ⚠️ 成功返回码是 paypal.Success(0)，不是 HTTP 200/201。SDK 在 2xx 时把 Code 设成 Success，
	// 非 2xx 才把 Code 覆盖成真实状态码，用 200/201 判断会永远走失败分支。
	if ppRsp == nil || ppRsp.Code != paypal.Success || ppRsp.Response == nil {
		msg := ""
		if ppRsp != nil {
			msg = ppRsp.Error
			if ppRsp.ErrorResponse != nil && ppRsp.ErrorResponse.Message != "" {
				msg = ppRsp.ErrorResponse.Message
			}
		}
		return false, fmt.Errorf("PayPal 退款失败（capture_id=%s）：%s", captureId, msg)
	}

	if refund.TerraceId != ppRsp.Response.Id && ppRsp.Response.Id != "" {
		refund.TerraceId = ppRsp.Response.Id
		if refund.Id > 0 && w.DB != nil {
			w.DB.Model(&model.OrderRefund{}).Where("id = ?", refund.Id).UpdateColumn("terrace_id", refund.TerraceId)
		}
	}
	library.DebugLog(w.CachePath, "paypal_refund", ppRsp.Response.Id, ppRsp.Response.Status)

	if ppRsp.Response.Status == "PENDING" {
		// 让后台看到「PayPal 还在处理」，而不是一条莫名的待处理记录
		refund.Remark = truncateRunes("PayPal refund "+ppRsp.Response.Id+" pending", 255)
		if refund.Id > 0 && w.DB != nil {
			w.DB.Model(&model.OrderRefund{}).Where("id = ?", refund.Id).UpdateColumn("remark", refund.Remark)
		}
		return false, nil
	}
	return true, nil
}

// ensureOrderRefund 取退款单，没有就补一张（后台直接退款、PayPal 后台发起退款都可能没有申请单）。
func (w *Website) ensureOrderRefund(order *model.Order, amount int64) (*model.OrderRefund, error) {
	refund, err := w.GetOrderRefundByOrderId(order.OrderId)
	if err == nil {
		// 已有退款单：金额只增不减，别把用户申请的部分退款覆盖成全额判断依据
		if refund.Amount <= 0 {
			refund.Amount = amount
			w.DB.Model(&model.OrderRefund{}).Where("id = ?", refund.Id).UpdateColumn("amount", refund.Amount)
		}
		return refund, nil
	}
	if !errors.Is(err, gorm.ErrRecordNotFound) {
		return nil, err
	}
	if amount <= 0 {
		amount = order.Amount
	}
	refund = &model.OrderRefund{
		OrderId: order.OrderId,
		UserId:  order.UserId,
		Amount:  amount,
		Status:  config.OrderRefundStatusWaiting,
	}
	if err = w.DB.Save(refund).Error; err != nil {
		return nil, err
	}
	return refund, nil
}

// CompleteGatewayRefund 把「网关已退款」落到本地（PayPal / Stripe 的退款通知都走这里）。
//
// 后台退款（SetOrderRefund）和 PAYMENT.CAPTURE.REFUNDED 通知都会走到这里，
// 幂等由 SuccessRefundOrder 的条件更新保证：只有把订单推进到「已退款」的那一次
// 才会写退款流水，另一次直接空转。
func (w *Website) CompleteGatewayRefund(payment *model.Payment, amount int64, terraceRefundId string) error {
	if payment == nil {
		return errors.New("payment is nil")
	}
	order, err := w.GetOrderInfoByOrderId(payment.OrderId)
	if err != nil {
		return err
	}
	refund, err := w.ensureOrderRefund(order, amount)
	if err != nil {
		return err
	}
	if refund.Status == config.OrderRefundStatusFailed {
		// 网关改口说这笔退成了（之前 Stripe 报 failed 后来又 succeeded、或通知晚于人工处理）：
		// 以网关为准，否则 SuccessRefundOrder 会拒绝推进，通知被无限重投。
		refund.Status = config.OrderRefundStatusWaiting
		if refund.Id > 0 && w.DB != nil {
			w.DB.Model(&model.OrderRefund{}).Where("id = ?", refund.Id).UpdateColumn("status", refund.Status)
		}
	}
	if terraceRefundId != "" && refund.TerraceId == "" {
		refund.TerraceId = terraceRefundId
		w.DB.Model(&model.OrderRefund{}).Where("id = ?", refund.Id).UpdateColumn("terrace_id", terraceRefundId)
	}
	return w.SuccessRefundOrder(refund, order)
}

// handlePaypalCaptureRefunded 处理 PAYMENT.CAPTURE.REFUNDED。
//
// 这类事件的 resource 是 capture（id 是 capture id，不是退款单 id），金额取
// seller_receivable_breakdown.total_refunded_amount，拿不到就退回本地退款单金额。
func (w *Website) handlePaypalCaptureRefunded(payment *model.Payment, res *PaypalWebhookResource) error {
	if res == nil {
		return errors.New("paypal refund resource is nil")
	}
	amount := int64(0)
	if res.SellerReceivableBreakdown.TotalRefundedAmount.Value != "" {
		if fen, ok := yuanToFen(res.SellerReceivableBreakdown.TotalRefundedAmount.Value); ok {
			amount = fen
		}
	}
	if amount <= 0 && res.Status == "REFUNDED" {
		if fen, ok := yuanToFen(res.Amount.Total); ok {
			amount = fen
		}
	}
	if amount <= 0 {
		// 部分退款时 capture 金额不等于退款金额，宁可回落到本地退款单金额
		if refund, err := w.GetOrderRefundByOrderId(payment.OrderId); err == nil {
			amount = refund.Amount
		}
	}
	return w.CompleteGatewayRefund(payment, amount, "")
}

// yuanToFen "50.00" -> 5000，解析失败返回 ok=false。
func yuanToFen(v string) (int64, bool) {
	v = strings.TrimSpace(v)
	if v == "" {
		return 0, false
	}
	var f float64
	if _, err := fmt.Sscanf(v, "%f", &f); err != nil {
		return 0, false
	}
	// 用四舍五入而不是截断，避免 49.999999 这类浮点表示误差少一分钱
	return int64(f*100 + 0.5), true
}

// logPaymentEvent 统一记录回调处理结果，方便运营在 cache 日志里对账。
func logPaymentEvent(cachePath, eventId, eventType, msg string) {
	library.DebugLog(cachePath, "payment_event", eventId, eventType, msg)
	xlog.Infof("payment event %s(%s): %s", eventType, eventId, msg)
}

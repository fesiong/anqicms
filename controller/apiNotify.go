package controller

import (
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"log"
	"net/http"
	"os"
	"strconv"

	"github.com/go-pay/gopay"
	"github.com/go-pay/gopay/paypal"
	"github.com/go-pay/gopay/wechat"
	"github.com/go-pay/xlog"
	"github.com/kataras/iris/v12"
	"github.com/medivhzhan/weapp/v3/server"
	"github.com/stripe/stripe-go/v82"
	"github.com/stripe/stripe-go/v82/webhook"
	"kandaoni.com/anqicms/config"
	"kandaoni.com/anqicms/library"
	"kandaoni.com/anqicms/model"
	"kandaoni.com/anqicms/provider"
)

func NotifyWeappMsg(ctx iris.Context) {
	currentSite := provider.CurrentSite(ctx)
	srv, err := server.NewServer(currentSite.PluginWeapp.AppID, currentSite.PluginWeapp.Token, currentSite.PluginWeapp.EncodingAESKey, "", "", true, nil)
	if err != nil {
		log.Printf("init server error: %s", err)
	}

	srv.OnSubscribeMsgPopup(func(popupEvent *server.SubscribeMsgPopupEvent) {
		SubscribeMsgPopup(ctx, popupEvent)
	})
	srv.OnSubscribeMsgChange(func(popupEvent *server.SubscribeMsgChangeEvent) {
		SubscribeMsgChange(ctx, popupEvent)
	})

	err = srv.Serve(ctx.ResponseWriter(), ctx.Request())
	if err != nil {
		log.Printf("serving error: %s", err)
	}
}

// NotifyWechatPay 公众号和小程序共用支付回调通知
func NotifyWechatPay(ctx iris.Context) {
	currentSite := provider.CurrentSite(ctx)
	body, err := ctx.GetBody()
	library.DebugLog(currentSite.CachePath, "wechat.log", string(body))
	rsp := new(wechat.NotifyResponse) // 回复微信的数据
	// 注意：这一步的 err 必须判，否则解析失败会一路走到「payment-not found」，
	// 看起来像是订单不存在，实际是报文没解开。
	notifyReq, err := wechat.ParseNotifyToBodyMap(ctx.Request())
	if err != nil {
		library.DebugLog(currentSite.CachePath, "wechat.log", "err", err, "notify-parse-failed")
		rsp.ReturnCode = gopay.FAIL
		rsp.ReturnMsg = ctx.Tr("PaymentFailed")
		ctx.WriteString(rsp.ToXmlString())
		return
	}

	// ⚠️ 退款结果通知必须最先判：它的支付单一定有 paid_time，后面任何一句
	// 「已支付就 return success」都会让退款结果永远落不下来（原来就是这么丢的）。
	// 而且退款通知的明文里**没有 out_trade_no**（只有加密的 req_info），
	// 所以不能用商户订单号去找支付单，必须先用各微信账号的 API 密钥试解密。
	if reqInfo := notifyReq.GetString("req_info"); reqInfo != "" {
		info, account, dErr := currentSite.DecryptWechatRefundNotifyWithAccount(reqInfo)
		if dErr == nil {
			var ok bool
			if ok, dErr = verifyWechatNotifySign(account, notifyReq); !ok && dErr == nil {
				dErr = errors.New("refund notify sign verify failed")
			}
		}
		if dErr != nil {
			library.DebugLog(currentSite.CachePath, "wechat.log", "refund-notify-decrypt-failed", dErr)
			rsp.ReturnCode = gopay.FAIL
			rsp.ReturnMsg = ctx.Tr("PaymentFailed")
			ctx.WriteString(rsp.ToXmlString())
			return
		}
		library.DebugLog(currentSite.CachePath, "wechat.log", "refund-notify", info)
		if err = currentSite.HandleWechatRefundNotify(info["out_trade_no"], info["refund_status"], info["refund_id"], info["success_time"]); err != nil {
			library.DebugLog(currentSite.CachePath, "wechat.log", "refund-notify-failed", err)
			rsp.ReturnCode = gopay.FAIL
			rsp.ReturnMsg = ctx.Tr("PaymentFailed")
			ctx.WriteString(rsp.ToXmlString())
			return
		}
		rsp.ReturnCode = gopay.SUCCESS
		rsp.ReturnMsg = gopay.OK
		ctx.WriteString(rsp.ToXmlString())
		return
	}

	//检查payment订单
	payment, err := currentSite.GetPaymentInfoByPaymentId(notifyReq.GetString("out_trade_no"))
	if err != nil {
		library.DebugLog(currentSite.CachePath, "wechat.log", "err", "payment-not found")
		rsp.ReturnCode = gopay.FAIL
		rsp.ReturnMsg = ctx.Tr("PaymentFailed")
		ctx.WriteString(rsp.ToXmlString())
		return
	}
	account, err := currentSite.GetPaymentAccountById(payment.PaymentAccountId)
	if err != nil {
		library.DebugLog(currentSite.CachePath, "wechat.log", "err", "account-not found")
		rsp.ReturnCode = gopay.FAIL
		rsp.ReturnMsg = ctx.Tr("PaymentFailed")
		ctx.WriteString(rsp.ToXmlString())
		return
	}
	ok, err := verifyWechatNotifySign(account, notifyReq)
	if !ok {
		library.DebugLog(currentSite.CachePath, "wechat.log", "err", err, "sign-verify-failed", fmt.Sprintf("%#v", notifyReq))
		rsp.ReturnCode = gopay.FAIL
		rsp.ReturnMsg = ctx.Tr("PaymentFailed")
		ctx.WriteString(rsp.ToXmlString())
		return
	}

	// result_code=FAIL 的支付失败通知：确认收到即可，绝不能据此把订单标记为已支付
	if notifyReq.GetString("result_code") != gopay.SUCCESS {
		library.DebugLog(currentSite.CachePath, "wechat.log", "result_code", notifyReq.GetString("result_code"),
			notifyReq.GetString("err_code"), notifyReq.GetString("err_code_des"))
		rsp.ReturnCode = gopay.SUCCESS
		rsp.ReturnMsg = gopay.OK
		ctx.WriteString(rsp.ToXmlString())
		return
	}
	if payment.PaidTime > 0 {
		library.DebugLog(currentSite.CachePath, "wechat.log", "err", "already-paid")
		rsp.ReturnCode = gopay.SUCCESS
		rsp.ReturnMsg = gopay.OK
		ctx.WriteString(rsp.ToXmlString())
		return
	}
	order, err := currentSite.GetOrderInfoByOrderId(payment.OrderId)
	if err != nil {
		library.DebugLog(currentSite.CachePath, "wechat.log", "err", "order not found")
		rsp.ReturnCode = gopay.FAIL
		rsp.ReturnMsg = ctx.Tr("PaymentFailed")
		ctx.WriteString(rsp.ToXmlString())
		return
	}
	if order.PaidTime > 0 {
		// 订单已被另一张支付单付掉：这张单得人工退款，别再往下记账
		library.DebugLog(currentSite.CachePath, "wechat.log", "err", "order need refund")
		rsp.ReturnCode = gopay.SUCCESS
		rsp.ReturnMsg = gopay.OK
		ctx.WriteString(rsp.ToXmlString())
		return
	}
	// 金额必须与下单时一致：通知的 total_fee 单位是分，与 payment.Amount 同单位。
	// 不一致说明 payment/order 映射错了，宁可让微信重试也不要记成已支付。
	if totalFee := notifyReq.GetString("total_fee"); totalFee != strconv.FormatInt(payment.Amount, 10) {
		library.DebugLog(currentSite.CachePath, "wechat.log", "err", "amount-mismatch",
			"notify:"+totalFee, "payment:"+strconv.FormatInt(payment.Amount, 10))
		rsp.ReturnCode = gopay.FAIL
		rsp.ReturnMsg = ctx.Tr("PaymentFailed")
		ctx.WriteString(rsp.ToXmlString())
		return
	}
	// 保留下单时的支付方式（公众号 wechat / 小程序 weapp），不要一律改写成 wechat
	if payment.PayWay == "" {
		payment.PayWay = config.PayWayWechat
	}
	// 统一走 MarkPaymentPaid：paid_time = 0 的条件更新是记账闸门。
	// 原来这里是「先判 PaidTime > 0，再无条件 Save + Create(finance)」，
	// 微信 15 秒重试与前台轮询并发时两条都能过判断，同一笔支付就记了两条流水。
	if err = currentSite.MarkPaymentPaid(payment, notifyReq.GetString("transaction_id"),
		notifyReq.GetString("openid"), ""); err != nil {
		library.DebugLog(currentSite.CachePath, "wechat.log", "err", "order pay failed", err)
		rsp.ReturnCode = gopay.FAIL
		rsp.ReturnMsg = ctx.Tr("PaymentFailed")
		ctx.WriteString(rsp.ToXmlString())
		return
	}
	//通知成功
	rsp.ReturnCode = gopay.SUCCESS
	rsp.ReturnMsg = gopay.OK
	ctx.WriteString(rsp.ToXmlString())
	return
}

// verifyWechatNotifySign 校验微信支付结果通知的签名。
//
// 这里有两个坑，都表现为「ok=false 且 err=nil」（静默验签失败，日志里只看到 err <nil>）：
//  1. gopay 的 wechat.VerifySign 第一个参数是**商户 API 密钥**（apiKey），不是 appId。
//     传 appId 时算出来的签名永远对不上，历史上这里传的就是 account.PayConfig.AppId。
//  2. 微信通知报文里不带 sign_type 字段，而签名算法必须与**下单时**一致。
//     本项目的下单（createWechatPayment / createWeappPayment / 退款）统一传 sign_type=HMAC-SHA256，
//     所以通知的 sign 是 64 位 HMAC-SHA256；gopay 在 sign_type 为空时默认 MD5，不能吃它的默认值。
//
// 历史有按 MD5 下单的订单，其通知 sign 是 32 位，按长度回落 MD5，既兼容旧订单
// 又不会让 64 位的 HMAC 通知被降级成 MD5。
func verifyWechatNotifySign(account *model.PaymentAccount, notifyReq gopay.BodyMap) (bool, error) {
	if account == nil {
		return false, errors.New("payment account is nil")
	}
	if account.PayConfig.ApiKey == "" {
		return false, errors.New("wechat api key is empty")
	}
	sign := notifyReq.GetString("sign")
	if sign == "" {
		return false, errors.New("notify sign is empty")
	}
	signType := wechat.SignType_HMAC_SHA256
	if len(sign) == 32 {
		signType = wechat.SignType_MD5
	}
	// VerifySign 会就地 Remove("sign")，传副本，避免污染调用方后续要用的 notifyReq
	bm := make(gopay.BodyMap, len(notifyReq))
	for k, v := range notifyReq {
		bm[k] = v
	}
	return wechat.VerifySign(account.PayConfig.ApiKey, signType, bm)
}

// alipayTradePaid 判断支付宝异步通知的 trade_status 是否代表「钱已到账」。
// WAIT_BUYER_PAY / TRADE_CLOSED 等非终态也会发通知，直接当成功会把未付款订单标记成已支付。
func alipayTradePaid(status string) bool {
	return status == "TRADE_SUCCESS" || status == "TRADE_FINISHED"
}

func NotifyAlipay(ctx iris.Context) {
	currentSite := provider.CurrentSite(ctx)
	if err := ctx.Request().ParseForm(); err != nil {
		return
	}
	var form map[string][]string = ctx.Request().Form
	library.DebugLog(currentSite.CachePath, "alipay.log", form)
	bm := make(gopay.BodyMap, len(form)+1)
	for k, v := range form {
		if len(v) == 1 {
			bm.Set(k, v[0])
		}
	}
	//检查payment订单
	payment, err := currentSite.GetPaymentInfoByPaymentId(bm.GetString("out_trade_no"))
	if err != nil {
		return
	}
	// ⚠️ 这里原来有「payment.PaidTime > 0 就 return success」。退款到账通知对应的
	// 支付单一定有 paid_time，那句话会把退款通知整条吞掉。幂等改由 MarkPaymentPaid
	// 的 paid_time = 0 条件更新负责。
	account, err := currentSite.GetPaymentAccountById(payment.PaymentAccountId)
	if err != nil {
		library.DebugLog(currentSite.CachePath, "alipay.log", "account-notfound", err)
		ctx.WriteString("fail")
		return
	}
	file, err := os.Open(currentSite.DataPath + "cert/" + account.PayConfig.PublicCertPath)
	if err != nil {
		library.DebugLog(currentSite.CachePath, "alipay.log", "cert-open-failed", err)
		return
	}
	defer file.Close()
	fileContent, err := io.ReadAll(file)
	if err != nil {
		library.DebugLog(currentSite.CachePath, "alipay.log", "cert-read-failed", err)
		return
	}

	// 证书模式走 VerifySignWithCert（VerifySign 的入参约定是裸 base64 公钥，
	// 带 PEM 头的证书内容会被重新包一层头，换一份证书文件就可能解析失败）。
	// 具体分流见 provider.VerifyAlipayNotifySign。
	ok, err := provider.VerifyAlipayNotifySign(fileContent, bm)
	if ok == false || err != nil {
		library.DebugLog(currentSite.CachePath, "alipay.log", "err", err, "sign-verify-failed", bm.GetString("out_trade_no"))
		ctx.WriteString("fail")
		return
	}

	// 退款到账通知（refund_status = REFUND_SUCCESS）必须排在所有「已支付就返回」之前，
	// 否则这类通知会被当成重复的支付通知直接 ack 掉，退款单永远停在待处理。
	if refundStatus := bm.GetString("refund_status"); refundStatus != "" {
		if err = currentSite.HandleAlipayRefundNotify(bm.GetString("out_trade_no"), refundStatus,
			bm.GetString("out_biz_no"), bm.GetString("gmt_refund")); err != nil {
			library.DebugLog(currentSite.CachePath, "alipay.log", "refund-notify-failed", err)
			ctx.WriteString("fail")
			return
		}
		ctx.WriteString("success")
		return
	}

	// 支付宝的异步通知在非终态（WAIT_BUYER_PAY）也会发，trade_status 不校验就直接
	// 标记已支付 = 用户还没付钱就把订单算成功
	if !alipayTradePaid(bm.GetString("trade_status")) {
		library.DebugLog(currentSite.CachePath, "alipay.log", "trade-status", bm.GetString("trade_status"), bm.GetString("out_trade_no"))
		ctx.WriteString("success") // 确认收到，但不改订单状态
		return
	}
	// 通知必须来自本账号配置的应用
	if account.PayConfig.AppId != "" && bm.GetString("app_id") != account.PayConfig.AppId {
		library.DebugLog(currentSite.CachePath, "alipay.log", "appid-mismatch",
			bm.GetString("app_id"), account.PayConfig.AppId)
		ctx.WriteString("fail")
		return
	}
	// total_amount 单位是元，payment.Amount 单位是分
	if v, perr := strconv.ParseFloat(bm.GetString("total_amount"), 64); perr != nil || int64(v*100+0.5) != payment.Amount {
		library.DebugLog(currentSite.CachePath, "alipay.log", "amount-mismatch",
			bm.GetString("total_amount"), strconv.FormatInt(payment.Amount, 10))
		ctx.WriteString("fail")
		return
	}

	order, err := currentSite.GetOrderInfoByOrderId(payment.OrderId)
	if err != nil {
		ctx.WriteString("fail")
		return
	}
	if order.PaidTime > 0 {
		// 已被另一张支付单付掉：这笔要人工退款，不能重复记账
		ctx.WriteString("success")
		return
	}
	buyerId := bm.GetString("buyer_id")
	if bm.GetString("buyer_open_id") != "" {
		buyerId = bm.GetString("buyer_open_id")
	}
	// 统一走 MarkPaymentPaid：paid_time = 0 的条件更新是「只记一笔付款流水」的闸门，
	// 原来这里是先判 PaidTime 再无条件 Save + Create(finance)，支付宝重发通知与
	// 前台轮询并发时会记两条。
	if err = currentSite.MarkPaymentPaid(payment, bm.GetString("trade_no"), buyerId,
		bm.GetString("buyer_logon_id")); err != nil {
		library.DebugLog(currentSite.CachePath, "alipay.log", "mark-paid-failed", err)
		ctx.WriteString("fail")
		return
	}

	//通知成功
	ctx.WriteString("success")
}

func NotifyPayPal(ctx iris.Context) {
	currentSite := provider.CurrentSite(ctx)
	// ⚠️ 这里原本要求 URL 带 ?token=xxx，但 UpdatePaypalWebhook 向 PayPal 注册的是
	// {BaseUrl}/notify/paypal/pay（不带任何 query），于是所有回调都在第一行就被挡掉，
	// 付款成功永远落不到订单上；而且返回的是 200 提示页，PayPal 认为投递成功、不再重试。
	// 回调的真正鉴权是下面的 VerifyWebhookSignature（用账号的 webhook_id 验签），不需要 token。
	body, err := ctx.GetBody()
	if err != nil {
		xlog.Errorf("Read body error: %v", err)
		return
	}
	library.DebugLog(currentSite.CachePath, "paypal_notify", string(body))
	// 处理结果
	var event paypal.WebhookEvent
	if err := json.Unmarshal(body, &event); err != nil {
		xlog.Errorf("JSON unmarshal error: %v", err)
		ctx.StatusCode(http.StatusBadRequest)
		ctx.WriteString("Invalid event format")
		return
	}

	// 4. 处理事件
	xlog.Infof("Received %s event: %s", event.EventType, event.Summary)
	// 获取payment信息
	var resp provider.PaypalWebhookResource
	err = json.Unmarshal(event.Resource, &resp)
	if err != nil {
		xlog.Errorf("Failed to unmarshal resource: %v, %v", err, string(event.Resource))
		return
	}
	// PAYMENT.CAPTURE.* 通知里的 id 是 capture id，而 payment.terrace_id 存的是 order id，
	// 需要经 supplementary_data.related_ids.order_id 换算，否则这类事件会静默丢掉
	payment, err := currentSite.GetPaymentInfoByPaypalResource(&resp)
	if err != nil {
		xlog.Errorf("Failed to get payment info: %v", err)
		// 非 2xx 让 PayPal 重试并在后台标红，别像原来那样 200 静默丢单
		ctx.StatusCode(http.StatusNotFound)
		ctx.WriteString("payment not found")
		return
	}
	account, err := currentSite.GetPaymentAccountById(payment.PaymentAccountId)
	if err != nil {
		library.DebugLog(currentSite.CachePath, "paypal_notify", "account-notfound", err)
		ctx.StatusCode(http.StatusNotFound)
		ctx.WriteString("account not found")
		return
	}

	// ⚠️ 验签必须用**原始请求体**。曾经这里是把 body 反序列化进 gopay.BodyMap 再序列化发出去，
	// 字段顺序/空白/转义一变，PayPal 签名里带的 crc32 就对不上，真实通知 100% 验签失败（HTTP 400）。
	// 现在由 provider 手拼请求体，webhook_event 部分原样拷贝收到的字节。
	headers := provider.PaypalSignatureHeaders{
		AuthAlgo:         ctx.GetHeader("Paypal-Auth-Algo"),
		CertUrl:          ctx.GetHeader("Paypal-Cert-Url"),
		TransmissionId:   ctx.GetHeader("Paypal-Transmission-Id"),
		TransmissionSig:  ctx.GetHeader("Paypal-Transmission-Sig"),
		TransmissionTime: ctx.GetHeader("Paypal-Transmission-Time"),
	}
	if err = currentSite.VerifyPaypalWebhook(account, headers, body); err != nil {
		// 验签报错多半是账号里的 webhook_id 没配或与 PayPal 后台的环境不一致，
		// 返回非 2xx 才能让问题暴露出来（原来 200 空响应 = 回调静默消失）
		xlog.Errorf("paypal webhook verify error (event %s, sandbox=%v): %v", event.Id, account.PayConfig.Sandbox, err)
		ctx.StatusCode(http.StatusBadRequest)
		ctx.WriteString("verify failed")
		return
	}

	// 同步处理：处理失败要返回非 2xx 让 PayPal 重投。事件级去重与状态闸门都在 provider 里，
	// 重投不会重复记账；改用协程会变成「先回 200 再慢慢处理」，失败就永远补不回来。
	if err = currentSite.ProcessPaypalEvent(&event); err != nil {
		xlog.Errorf("paypal webhook process error (event %s, type %s): %v", event.Id, event.EventType, err)
		ctx.StatusCode(http.StatusInternalServerError)
		ctx.WriteString("process failed")
		return
	}

	ctx.StatusCode(http.StatusOK)
	ctx.WriteString("Webhook processed")
}

// stripeEventPaymentRef 从事件体（data.object）里取出定位支付单的线索：
// PaymentIntent id（= payment.terrace_id）与 metadata.payment_id（下单时写入，作兜底）。
//
// 注意不能拿 stripe.Event.ID（evt_ 开头）去查 payment_id —— 那是事件 ID，
// 历史上 notify 里就是这么查的，结果每一次回调都在查库失败后静默 return，
// 返回 200 空响应，Stripe 认为投递成功，订单永远停在未支付。
func stripeEventPaymentRef(raw json.RawMessage) (piId, paymentId string) {
	if len(raw) == 0 {
		return "", ""
	}
	var obj struct {
		Id            string            `json:"id"`
		PaymentIntent string            `json:"payment_intent"`
		Metadata      map[string]string `json:"metadata"`
	}
	if err := json.Unmarshal(raw, &obj); err != nil {
		return "", ""
	}
	// charge.* / refund.* 事件的 object 是 ch_/re_ 开头，payment.terrace_id 存的是 pi_，
	// 必须取 payment_intent 才能定位（直接拿 object.id 永远查不到）。
	if obj.PaymentIntent != "" {
		return obj.PaymentIntent, obj.Metadata["payment_id"]
	}
	return obj.Id, obj.Metadata["payment_id"]
}

func NotifyStripe(ctx iris.Context) {
	currentSite := provider.CurrentSite(ctx)
	body, err := ctx.GetBody()
	if err != nil {
		log.Printf("Error reading body: %v", err)
		ctx.StatusCode(http.StatusServiceUnavailable)
		return
	}
	library.DebugLog(currentSite.CachePath, "stripe_notify", string(body))
	// 验证Webhook签名
	e := stripe.Event{}
	if err = json.Unmarshal(body, &e); err != nil {
		log.Printf("Failed to parse webhook body json: %s", err.Error())
		ctx.StatusCode(http.StatusServiceUnavailable)
		return
	}
	// ⚠️ e.ID 是 evt_ 开头的事件 ID，既不是我们的 payment_id，也不是 PaymentIntent id。
	// 支付单的映射关系是：payment.terrace_id = PaymentIntent id（下单时写入），
	// metadata.payment_id 作为兜底。这里先用它定位支付账号（拿该账号配置的 webhook 签名密钥），
	// 真正的处理在验签成功之后，伪造这一段改不了任何数据。
	piId, metaPaymentId := stripeEventPaymentRef(e.Data.Raw)
	payment, err := currentSite.GetPaymentInfoByTerraceId(piId)
	if err != nil && metaPaymentId != "" {
		payment, err = currentSite.GetPaymentInfoByPaymentId(metaPaymentId)
	}
	handledType := e.Type == "payment_intent.succeeded" || e.Type == "payment_intent.payment_failed" ||
		e.Type == "charge.refunded"
	if err != nil {
		library.DebugLog(currentSite.CachePath, "stripe_notify", "err", err, "payment-not-found",
			"event:"+e.ID, "object:"+piId)
		if handledType {
			// 返回非 2xx，让 Stripe 后台里的投递记录变成失败，别再静默丢单
			ctx.StatusCode(http.StatusNotFound)
			ctx.WriteString("payment not found")
			return
		}
		ctx.StatusCode(http.StatusOK)
		return
	}
	account, err := currentSite.GetPaymentAccountById(payment.PaymentAccountId)
	if err != nil {
		library.DebugLog(currentSite.CachePath, "stripe.log", "account-notfound", err)
		ctx.WriteString("fail")
		return
	}

	endpointSecret := account.PayConfig.WebhookId
	signatureHeader := ctx.GetHeader("Stripe-Signature")

	event, err := webhook.ConstructEvent(body, signatureHeader, endpointSecret)
	if err != nil {
		log.Printf("Webhook signature verification failed: %v", err)
		ctx.StatusCode(http.StatusBadRequest)
		return
	}

	// 处理事件类型
	switch event.Type {
	case "payment_intent.succeeded":
		var pi stripe.PaymentIntent
		err = json.Unmarshal(event.Data.Raw, &pi)
		if err != nil {
			log.Printf("Error parsing payment intent: %v", err)
			ctx.StatusCode(http.StatusBadRequest)
			return
		}
		// payment 已在验签前按 PaymentIntent id 定位，不能再拿 pi.ID 当 payment_id 查（永远查不到）
		if pi.Amount != payment.Amount {
			// 金额不一致只告警不阻断，TraceQuery 以 Stripe 侧真实状态为准
			log.Printf("stripe webhook: amount mismatch, pi=%d payment=%d payment_id=%s",
				pi.Amount, payment.Amount, payment.PaymentId)
		}
		if payment.PaidTime > 0 {
			ctx.StatusCode(http.StatusOK)
			return
		}
		order, err := currentSite.GetOrderInfoByOrderId(payment.OrderId)
		if err != nil {
			//ctx.WriteString(bm.NotOK("支付失败"))
			return
		}
		if order.PaidTime > 0 {
			// todo 已支付了，这个payment需要退款
			ctx.StatusCode(http.StatusOK)
			return
		}

		payment.PayWay = config.PayWayStripe
		err = currentSite.TraceQuery(payment)
		if err != nil {
			//ctx.WriteString(bm.NotOK("支付失败"))
			return
		}
	case "payment_intent.payment_failed":
		var pi stripe.PaymentIntent
		err = json.Unmarshal(event.Data.Raw, &pi)
		if err != nil {
			log.Printf("Error parsing payment intent: %v", err)
			ctx.StatusCode(http.StatusBadRequest)
			return
		}
		// payment 同上，已在验签前定位，不按 pi.ID 重查。
		// 已支付/已退款的订单不能因为一条 failed 事件被取消掉（支付失败重试成功后就地改状态）。
		order, err := currentSite.GetOrderInfoByOrderId(payment.OrderId)
		if err == nil && order.PaidTime == 0 && order.Status != config.OrderStatusRefunded {
			_ = currentSite.SetOrderCanceled(order)
		}

	case "charge.refunded":
		// Stripe 的退款结果。原来这个分支根本没实现，退款完全靠后台那次同步返回，
		// 一旦返回 pending（银行处理中）就再没有任何东西会把它推成已完成。
		var ch stripe.Charge
		if err = json.Unmarshal(event.Data.Raw, &ch); err != nil {
			log.Printf("Error parsing charge: %v", err)
			ctx.StatusCode(http.StatusBadRequest)
			return
		}
		amount := ch.AmountRefunded
		if amount <= 0 {
			amount = payment.Amount
		}
		if err = currentSite.CompleteGatewayRefund(payment, amount, ""); err != nil {
			log.Printf("stripe refund complete failed: %v", err)
			ctx.StatusCode(http.StatusInternalServerError)
			return
		}

	default:
		log.Printf("Unhandled event type: %s", event.Type)
	}

	ctx.StatusCode(http.StatusOK)
}

// NotifyXunhupay 虎皮椒支付结果通知（form 提交）。
//
// 网关只认纯文本 success，返回其它内容会重试 6 次；反过来，**任何校验不过都必须返回非 success**。
// 虎皮椒官方公告里被刷单的站点，全都是这一步没验签 / 没验状态，任何人 POST 一下就自动发货。
func NotifyXunhupay(ctx iris.Context) {
	currentSite := provider.CurrentSite(ctx)
	_ = ctx.Request().ParseForm()
	params := provider.XunhupayFormParams(ctx.Request().Form)
	library.DebugLog(currentSite.CachePath, "xunhupay.log", params)

	// reject 统一处理失败：写清楚是哪一步不过，再让网关重试
	reject := func(reason string, err error) {
		library.DebugLog(currentSite.CachePath, "xunhupay.log", "reason", reason, "err", err,
			"trade_order_id", params["trade_order_id"])
		ctx.WriteString("fail")
	}

	if params["trade_order_id"] == "" {
		reject("trade-order-id-missing", nil)
		return
	}
	// 通知里的商户订单号就是下单时传的 payment.PaymentId
	payment, err := currentSite.GetPaymentInfoByPaymentId(params["trade_order_id"])
	if err != nil {
		reject("payment-not-found", err)
		return
	}
	account, err := currentSite.GetPaymentAccountById(payment.PaymentAccountId)
	if err != nil {
		reject("account-not-found", err)
		return
	}
	if err = provider.XunhupayNotifySignVerify(params, account.PayConfig.AppSecret, account.PayConfig.AppId); err != nil {
		reject("sign-verify-failed", err)
		return
	}

	switch params["status"] {
	case provider.XunhupayStatusPaid:
		// 继续走下面的支付成功逻辑
	case provider.XunhupayStatusRefunded, provider.XunhupayStatusRefunding, provider.XunhupayStatusRefundFailed:
		// 退款结果也发到同一个 notify_url。
		// 状态机在 provider 里：CD 才推进订单与退款流水，RD（退款中）/UD（退款失败）
		// 只更新退款单本身。原来三种状态都调 SuccessRefundOrder，
		// 退款还没到账就已经把订单算成已退款、还记了一笔退款流水。
		if err = currentSite.HandleXunhupayRefundNotify(params["trade_order_id"], params["status"]); err != nil {
			reject("refund-notify-failed", err)
			return
		}
		ctx.WriteString("success")
		return
	default:
		// 非终态（等待支付等）：确认收到但不改订单状态
		library.DebugLog(currentSite.CachePath, "xunhupay.log", "unhandled-status", params["status"])
		ctx.WriteString("success")
		return
	}

	// total_fee 单位是元，payment.Amount 单位是分
	amountFen, aErr := provider.XunhupayAmountFen(params["total_fee"])
	if aErr != nil || amountFen != payment.Amount {
		reject("amount-mismatch", fmt.Errorf("notify total_fee %q -> %d fen, payment %d fen (%v)",
			params["total_fee"], amountFen, payment.Amount, aErr))
		return
	}
	if payment.PaidTime > 0 {
		library.DebugLog(currentSite.CachePath, "xunhupay.log", "already-paid", payment.PaymentId)
		ctx.WriteString("success")
		return
	}

	buyerInfo := params["open_order_id"]
	if err = currentSite.MarkPaymentPaid(payment, params["transaction_id"], "", buyerInfo); err != nil {
		reject("mark-paid-failed", err)
		return
	}

	ctx.WriteString("success")
}

// NotifyNowpayment NOWPayments IPN 回调（JSON + x-nowpayments-sig 头）。
func NotifyNowpayment(ctx iris.Context) {
	currentSite := provider.CurrentSite(ctx)
	body, err := ctx.GetBody()
	signature := ctx.GetHeader("x-nowpayments-sig")
	library.DebugLog(currentSite.CachePath, "nowpayment.log", "sig:"+signature, string(body))

	if err != nil || len(body) == 0 {
		ctx.StatusCode(http.StatusBadRequest)
		return
	}

	notify, err := provider.ParseNowpaymentNotify(body)
	if err != nil {
		library.DebugLog(currentSite.CachePath, "nowpayment.log", "decode-failed", err)
		ctx.StatusCode(http.StatusBadRequest)
		return
	}

	// 下单时把 payment.PaymentId 当作 order_id 传过去，IPN 原样带回
	payment, err := currentSite.GetPaymentInfoByPaymentId(string(notify.OrderId))
	if err != nil {
		library.DebugLog(currentSite.CachePath, "nowpayment.log", "payment-not-found", notify.OrderId)
		ctx.StatusCode(http.StatusBadRequest)
		return
	}
	account, err := currentSite.GetPaymentAccountById(payment.PaymentAccountId)
	if err != nil {
		library.DebugLog(currentSite.CachePath, "nowpayment.log", "account-not-found", err)
		ctx.StatusCode(http.StatusBadRequest)
		return
	}
	// 验签失败返回 400：NOWPayments 后台会把这次投递标红，比静默 200 好排查
	if err = provider.VerifyNowpaymentIPN(body, signature, account.PayConfig.WebhookId); err != nil {
		library.DebugLog(currentSite.CachePath, "nowpayment.log", "sign-verify-failed", err)
		ctx.StatusCode(http.StatusBadRequest)
		return
	}

	// 未到账的状态（waiting/confirming/partially_paid/failed/expired…）一律只记日志：
	// partially_paid 是用户少付，必须人工判断，绝不能当成支付成功
	if !provider.NowpaymentIsPaid(notify.PaymentStatus) {
		library.DebugLog(currentSite.CachePath, "nowpayment.log", "status", notify.PaymentStatus, notify.OrderId)
		ctx.StatusCode(http.StatusOK)
		return
	}
	if payment.PaidTime > 0 {
		ctx.StatusCode(http.StatusOK)
		return
	}

	// price_amount 是法币金额（payment.Amount 是分）
	amountFen, aErr := notify.AmountFen()
	if aErr != nil || amountFen != payment.Amount {
		library.DebugLog(currentSite.CachePath, "nowpayment.log", "amount-mismatch",
			string(notify.PriceAmount), strconv.FormatInt(payment.Amount, 10), aErr)
		ctx.StatusCode(http.StatusBadRequest)
		return
	}

	terraceId := string(notify.PaymentId)
	if err = currentSite.MarkPaymentPaid(payment, terraceId, notify.PayAddress, notify.PayCurrency); err != nil {
		library.DebugLog(currentSite.CachePath, "nowpayment.log", "mark-paid-failed", err)
		// 订单已被另一张支付单付掉等情况：单据本身没问题，返回 200 停止重试，靠日志人工跟进
		ctx.StatusCode(http.StatusOK)
		return
	}

	ctx.StatusCode(http.StatusOK)
}

// SubscribeMsgPopup 订阅消息弹框事件
func SubscribeMsgPopup(ctx iris.Context, popupEvent *server.SubscribeMsgPopupEvent) {
	currentSite := provider.CurrentSite(ctx)
	for _, v := range popupEvent.SubscribeMsgPopupEvent {
		subscribedUser := model.SubscribedUser{
			Openid:     popupEvent.FromUserName,
			TemplateId: v.TemplateId,
		}
		if v.SubscribeStatusString == server.SubscribeResultAccept {
			//订阅
			currentSite.DB.Where("openid = ? AND template_id = ?", subscribedUser.Openid, subscribedUser.TemplateId).FirstOrCreate(&subscribedUser)
		} else {
			//拒绝
			currentSite.DB.Unscoped().Where("openid = ? AND template_id = ?", subscribedUser.Openid, subscribedUser.TemplateId).Delete(model.SubscribedUser{})
		}
	}
}

// SubscribeMsgChange 用户改变订阅消息事件
func SubscribeMsgChange(ctx iris.Context, popupEvent *server.SubscribeMsgChangeEvent) {
	currentSite := provider.CurrentSite(ctx)
	for _, v := range popupEvent.SubscribeMsgChangeEvent {
		subscribedUser := model.SubscribedUser{
			Openid:     popupEvent.FromUserName,
			TemplateId: v.TemplateId,
		}
		if v.SubscribeStatusString == server.SubscribeResultAccept {
			//订阅
			currentSite.DB.Where("openid = ? AND template_id = ?", subscribedUser.Openid, subscribedUser.TemplateId).FirstOrCreate(&subscribedUser)
		} else {
			//拒绝
			currentSite.DB.Unscoped().Where("openid = ? AND template_id = ?", subscribedUser.Openid, subscribedUser.TemplateId).Delete(model.SubscribedUser{})
		}
	}
}

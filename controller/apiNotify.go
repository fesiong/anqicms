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
	"time"

	"github.com/go-pay/gopay"
	"github.com/go-pay/gopay/alipay"
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
		library.DebugLog(currentSite.CachePath, "wechat.log", "err", "order need refund")
		// todo 已支付了，这个payment需要退款
		rsp.ReturnCode = gopay.SUCCESS
		rsp.ReturnMsg = gopay.OK
		ctx.WriteString(rsp.ToXmlString())
		return
	}

	refundId := notifyReq.GetString("refund_id")

	if refundId != "" {
		// this is a refund order
		refund, err := currentSite.GetOrderRefundByOrderId(order.OrderId)
		if err == nil {
			if notifyReq.GetString("refund_status") == "SUCCESS" {
				//退款成功
				refund.Status = config.OrderRefundStatusDone
			} else {
				refund.Status = config.OrderRefundStatusFailed
			}
			refund.Remark = notifyReq.GetString("refund_status")
			parsedTime, err := time.ParseInLocation("2006-01-02 15:04:05", notifyReq.GetString("success_time"), time.Local)
			if err != nil {
				parsedTime = time.Now()
			}
			refund.RefundTime = parsedTime.Unix()

			_ = currentSite.SuccessRefundOrder(refund, order)
		}
	} else {
		// this is a pay order
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
		payment.TerraceId = notifyReq.GetString("transaction_id")
		payment.PaidTime = time.Now().Unix()
		payment.BuyerId = notifyReq.GetString("openid")
		currentSite.DB.Save(payment)
		order.PaymentId = payment.PaymentId
		currentSite.DB.Save(order)

		//生成用户支付记录
		var userBalance int64
		err = currentSite.DB.Model(&model.User{}).Where("`id` = ?", payment.UserId).Pluck("balance", &userBalance).Error
		//状态更改了，增加一条记录到用户
		finance := model.Finance{
			UserId:      payment.UserId,
			Direction:   config.FinanceOutput,
			Amount:      payment.Amount,
			AfterAmount: userBalance,
			Action:      config.FinanceActionBuy,
			OrderId:     payment.OrderId,
			Status:      1,
		}
		err = currentSite.DB.Create(&finance).Error
		if err != nil {
			library.DebugLog(currentSite.CachePath, "wechat.log", "err", "order pay failed")
			rsp.ReturnCode = gopay.FAIL
			rsp.ReturnMsg = ctx.Tr("PaymentFailed")
			ctx.WriteString(rsp.ToXmlString())
			return
		}

		//支付成功逻辑处理
		err = currentSite.SuccessPaidOrder(order, payment)
		if err != nil {
			library.DebugLog(currentSite.CachePath, "wechat.log", "err", "order pay failed")
			rsp.ReturnCode = gopay.FAIL
			rsp.ReturnMsg = ctx.Tr("PaymentFailed")
			ctx.WriteString(rsp.ToXmlString())
			return
		}
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
	if payment.PaidTime > 0 {
		ctx.WriteString("success")
		return
	}
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

	ok, err := alipay.VerifySign(string(fileContent), bm)
	if ok == false || err != nil {
		library.DebugLog(currentSite.CachePath, "alipay.log", "err", err, "sign-verify-failed", bm.GetString("out_trade_no"))
		ctx.WriteString("fail")
		return
	}

	// 只有支付通知才需要校验终态；退款通知（带 refund_id）走下面的分支
	refundId := bm.GetString("refund_id")
	if refundId == "" {
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
	}

	order, err := currentSite.GetOrderInfoByOrderId(payment.OrderId)
	if err != nil {
		//ctx.WriteString(bm.NotOK("支付失败"))
		return
	}
	if order.PaidTime > 0 {
		// todo 已支付了，这个payment需要退款

		ctx.WriteString("success")
		return
	}
	if refundId != "" {
		// this is a refund order
		refund, err := currentSite.GetOrderRefundByOrderId(order.OrderId)
		if err == nil {
			if bm.GetString("refund_status") == "SUCCESS" {
				//退款成功
				refund.Status = config.OrderRefundStatusDone
			} else {
				refund.Status = config.OrderRefundStatusFailed
			}
			refund.Remark = bm.GetString("refund_status")
			parsedTime, err := time.ParseInLocation("2006-01-02 15:04:05", bm.GetString("success_time"), time.Local)
			if err != nil {
				parsedTime = time.Now()
			}
			refund.RefundTime = parsedTime.Unix()

			_ = currentSite.SuccessRefundOrder(refund, order)
		}
	} else {
		// this is a pay order
		payment.PayWay = "alipay"
		payment.PaidTime = time.Now().Unix()
		payment.TerraceId = bm.GetString("trade_no")
		payment.BuyerId = bm.GetString("buyer_id")
		if bm.GetString("buyer_open_id") != "" {
			payment.BuyerId = bm.GetString("buyer_open_id")
		}
		payment.BuyerInfo = bm.GetString("buyer_logon_id")
		currentSite.DB.Save(payment)
		order.PaymentId = payment.PaymentId
		currentSite.DB.Save(order)
		//生成用户支付记录
		var userBalance int64
		err = currentSite.DB.Model(&model.User{}).Where("`id` = ?", payment.UserId).Pluck("balance", &userBalance).Error
		//状态更改了，增加一条记录到用户
		finance := model.Finance{
			UserId:      payment.UserId,
			Direction:   config.FinanceOutput,
			Amount:      payment.Amount,
			AfterAmount: userBalance,
			Action:      config.FinanceActionBuy,
			OrderId:     payment.OrderId,
			Status:      1,
		}
		err = currentSite.DB.Create(&finance).Error
		if err != nil {
			ctx.WriteString("success")
			return
		}
		//支付成功逻辑处理
		err = currentSite.SuccessPaidOrder(order, payment)
		if err != nil {
			ctx.WriteString("success")
			return
		}
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
		library.DebugLog(currentSite.CachePath, "alipay.log", "account-notfound", err)
		ctx.WriteString("fail")
		return
	}
	client, err := paypal.NewClient(account.PayConfig.AppId, account.PayConfig.AppSecret, account.PayConfig.Sandbox == false)
	if err != nil {
		// 处理token获取失败
		ctx.WriteString("failed")
		return
	}
	bm := make(gopay.BodyMap)
	bm.Set("auth_algo", ctx.GetHeader("Paypal-Auth-Algo")).
		Set("cert_url", ctx.GetHeader("Paypal-Cert-Url")).
		Set("transmission_id", ctx.GetHeader("Paypal-Transmission-Id")).
		Set("transmission_sig", ctx.GetHeader("Paypal-Transmission-Sig")).
		Set("transmission_time", ctx.GetHeader("Paypal-Transmission-Time")).
		Set("webhook_id", account.PayConfig.WebhookId).
		SetBodyMap("webhook_event", func(b gopay.BodyMap) {
			err = json.Unmarshal(body, &b)
			if err != nil {
				xlog.Errorf("[%+v]: %v, bytes: %s", gopay.UnmarshalErr, err, string(body))
			}
		})

	xlog.Debug("bm：", bm.JsonBody())
	verifyRes, err := client.VerifyWebhookSignature(ctx, bm)
	if err != nil {
		xlog.Error(err)
		// 验签报错多半是账号里的 webhook_id 没配或与 PayPal 后台不一致，
		// 返回非 2xx 才能让问题暴露出来（原来 200 空响应 = 回调静默消失）
		ctx.StatusCode(http.StatusBadRequest)
		ctx.WriteString("verify error")
		return
	}
	xlog.Debugf("verifyRes: %+v", verifyRes)

	if verifyRes.VerificationStatus != "SUCCESS" {
		// 处理验证失败
		xlog.Error("paypal webhook verify error")
		ctx.StatusCode(http.StatusBadRequest)
		ctx.WriteString("verify failed")
		return
	}

	// 使用协程异步处理事件，避免阻塞响应
	go currentSite.ProcessPaypalEvent(&event)

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
		Id       string            `json:"id"`
		Metadata map[string]string `json:"metadata"`
	}
	if err := json.Unmarshal(raw, &obj); err != nil {
		return "", ""
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
	handledType := e.Type == "payment_intent.succeeded" || e.Type == "payment_intent.payment_failed"
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
	if payment.PaidTime > 0 {
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
		// payment 同上，已在验签前定位，不按 pi.ID 重查
		order, err := currentSite.GetOrderInfoByOrderId(payment.OrderId)
		if err == nil {
			_ = currentSite.SetOrderCanceled(order)
		}

	default:
		log.Printf("Unhandled event type: %s", event.Type)
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

package controller

import (
	"fmt"
	"github.com/go-pay/gopay"
	"github.com/go-pay/gopay/alipay"
	"github.com/go-pay/gopay/wechat"
	"github.com/kataras/iris/v12"
	"github.com/medivhzhan/weapp/v3/server"
	"io"
	"kandaoni.com/anqicms/config"
	"kandaoni.com/anqicms/library"
	"kandaoni.com/anqicms/model"
	"kandaoni.com/anqicms/provider"
	"log"
	"os"
	"time"
)

func NotifyWeappMsg(ctx iris.Context) {
	currentSite := provider.CurrentSite(ctx)
	srv, err := server.NewServer(currentSite.PluginWeapp.AppID, currentSite.PluginWeapp.Token, currentSite.PluginWeapp.EncodingAESKey, currentSite.PluginPay.WechatMchId, currentSite.PluginPay.WechatApiKey, true, nil)
	if err != nil {
		log.Println(fmt.Sprintf("init server error: %s", err))
	}

	srv.OnSubscribeMsgPopup(func(popupEvent *server.SubscribeMsgPopupEvent) {
		SubscribeMsgPopup(ctx, popupEvent)
	})
	srv.OnSubscribeMsgChange(func(popupEvent *server.SubscribeMsgChangeEvent) {
		SubscribeMsgChange(ctx, popupEvent)
	})

	err = srv.Serve(ctx.ResponseWriter(), ctx.Request())
	if err != nil {
		log.Println(fmt.Sprintf("serving error: %s", err))
	}
}

// NotifyWechatPay 公众号和小程序共用支付回调通知
func NotifyWechatPay(ctx iris.Context) {
	currentSite := provider.CurrentSite(ctx)
	body, err := ctx.GetBody()
	library.DebugLog(currentSite.CachePath, "wechat.log", string(body))
	notifyReq, err := wechat.ParseNotifyToBodyMap(ctx.Request())
	rsp := new(wechat.NotifyResponse) // 回复微信的数据

	ok, err := wechat.VerifySign(currentSite.PluginPay.WechatApiKey, wechat.SignType_MD5, notifyReq)
	if !ok {
		library.DebugLog(currentSite.CachePath, "wechat.log", "err", err, fmt.Sprintf("%#v", notifyReq))
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
		payment.PayWay = "wechat"
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
		err = currentSite.SuccessPaidOrder(order)
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

	file, err := os.Open(currentSite.DataPath + "cert/" + currentSite.PluginPay.AlipayPublicCertPath)
	if err != nil {
		return
	}
	fileContent, err := io.ReadAll(file)

	ok, err := alipay.VerifySign(string(fileContent), bm)
	if ok == false || err != nil {
		library.DebugLog(currentSite.CachePath, "alipay.log", err)
		ctx.WriteString("fail")
		return
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
	refundId := bm.GetString("refund_id")
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
		err = currentSite.SuccessPaidOrder(order)
		if err != nil {
			ctx.WriteString("success")
			return
		}
	}

	//通知成功
	ctx.WriteString("success")
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

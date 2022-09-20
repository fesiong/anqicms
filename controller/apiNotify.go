package controller

import (
	"fmt"
	"github.com/go-pay/gopay"
	"github.com/go-pay/gopay/alipay"
	"github.com/go-pay/gopay/wechat"
	"github.com/kataras/iris/v12"
	"github.com/medivhzhan/weapp/v3/server"
	"io/ioutil"
	"kandaoni.com/anqicms/config"
	"kandaoni.com/anqicms/dao"
	"kandaoni.com/anqicms/library"
	"kandaoni.com/anqicms/model"
	"kandaoni.com/anqicms/provider"
	"log"
	"os"
	"time"
)

func NotifyWeixinMsg(ctx iris.Context) {
	srv, err := server.NewServer(config.JsonData.PluginWeapp.AppID, config.JsonData.PluginWeapp.Token, config.JsonData.PluginWeapp.EncodingAESKey, config.JsonData.PluginPay.WeixinMchId, config.JsonData.PluginPay.WeixinApiKey, true, nil)
	if err != nil {
		log.Println(fmt.Sprintf("init server error: %s", err))
	}

	srv.OnSubscribeMsgPopup(SubscribeMsgPopup)
	srv.OnSubscribeMsgChange(SubscribeMsgChange)

	err = srv.Serve(ctx.ResponseWriter(), ctx.Request())
	if err != nil {
		log.Println(fmt.Sprintf("serving error: %s", err))
	}
}

func NotifyWexinPay(ctx iris.Context) {
	body, err := ctx.GetBody()
	library.DebugLog("wechat", string(body))
	notifyReq, err := wechat.ParseNotifyToBodyMap(ctx.Request())
	rsp := new(wechat.NotifyResponse) // 回复微信的数据

	ok, err := wechat.VerifySign(config.JsonData.PluginPay.WeixinApiKey, wechat.SignType_MD5, notifyReq)
	if !ok {
		library.DebugLog("wechat", "err", err, fmt.Sprintf("%#v", notifyReq))
		rsp.ReturnCode = gopay.FAIL
		rsp.ReturnMsg = "支付失败"
		ctx.WriteString(rsp.ToXmlString())
		return
	}
	//检查payment订单
	payment, err := provider.GetPaymentInfoByPaymentId(notifyReq.GetString("out_trade_no"))
	if err != nil {
		library.DebugLog("wechat", "err", "payment-not found")
		rsp.ReturnCode = gopay.FAIL
		rsp.ReturnMsg = "支付失败"
		ctx.WriteString(rsp.ToXmlString())
		return
	}
	if payment.PaidTime > 0 {
		library.DebugLog("wechat", "err", "already-paid")
		rsp.ReturnCode = gopay.SUCCESS
		rsp.ReturnMsg = gopay.OK
		ctx.WriteString(rsp.ToXmlString())
		return
	}
	order, err := provider.GetOrderInfoByOrderId(payment.OrderId)
	if err != nil {
		library.DebugLog("wechat", "err", "order not found")
		rsp.ReturnCode = gopay.FAIL
		rsp.ReturnMsg = "支付失败"
		ctx.WriteString(rsp.ToXmlString())
		return
	}
	if order.PaidTime > 0 {
		library.DebugLog("wechat", "err", "order need refund")
		// todo 已支付了，这个payment需要退款
		rsp.ReturnCode = gopay.SUCCESS
		rsp.ReturnMsg = gopay.OK
		ctx.WriteString(rsp.ToXmlString())
		return
	}

	refundId := notifyReq.GetString("refund_id")

	if refundId != "" {
		// this is a refund order
		refund, err := provider.GetOrderRefundByOrderId(order.OrderId)
		if err == nil {
			if notifyReq.GetString("refund_status") == "SUCCESS" {
				//退款成功
				refund.Status = config.OrderRefundStatusDone
			} else {
				refund.Status = config.OrderRefundStatusFailed
			}
			refund.Remark = notifyReq.GetString("refund_status")
			parsedTime, err := time.Parse("2006-01-02 15:04:05", notifyReq.GetString("success_time"))
			if err != nil {
				parsedTime = time.Now()
			}
			refund.RefundTime = parsedTime.Unix()

			_ = provider.SuccessRefundOrder(refund, order)
		}
	} else {
		// this is a pay order
		payment.PaidWay = "wechat"
		payment.TerraceId = notifyReq.GetString("transaction_id")
		payment.PaidTime = time.Now().Unix()
		dao.DB.Save(payment)

		//生成用户支付记录
		var userBalance int64
		err = dao.DB.Model(&model.User{}).Where("`id` = ?", payment.UserId).Pluck("balance", &userBalance).Error
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
		err = dao.DB.Create(&finance).Error
		if err != nil {
			library.DebugLog("wechat", "err", "order pay failed")
			rsp.ReturnCode = gopay.FAIL
			rsp.ReturnMsg = "支付失败"
			ctx.WriteString(rsp.ToXmlString())
			return
		}

		//支付成功逻辑处理
		err = provider.SuccessPaidOrder(order)
		if err != nil {
			library.DebugLog("wechat", "err", "order pay failed")
			rsp.ReturnCode = gopay.FAIL
			rsp.ReturnMsg = "支付失败"
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
	if err := ctx.Request().ParseForm(); err != nil {
		return
	}
	var form map[string][]string = ctx.Request().Form
	library.DebugLog("alipay", form)
	bm := make(gopay.BodyMap, len(form)+1)
	for k, v := range form {
		if len(v) == 1 {
			bm.Set(k, v[0])
		}
	}

	file, err := os.Open(config.ExecPath + config.JsonData.PluginPay.AlipayCertPath)
	if err != nil {
		return
	}
	fileContent, err := ioutil.ReadAll(file)

	ok, err := alipay.VerifySign(string(fileContent), bm)
	if ok == false || err != nil {
		ctx.WriteString("fail")
		return
	}

	//检查payment订单
	payment, err := provider.GetPaymentInfoByPaymentId(bm.GetString("out_trade_no"))
	if err != nil {
		//ctx.WriteString(bm.NotOK("支付失败"))
		return
	}
	if payment.PaidTime > 0 {
		ctx.WriteString("success")
		return
	}
	order, err := provider.GetOrderInfoByOrderId(payment.OrderId)
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
		refund, err := provider.GetOrderRefundByOrderId(order.OrderId)
		if err == nil {
			if bm.GetString("refund_status") == "SUCCESS" {
				//退款成功
				refund.Status = config.OrderRefundStatusDone
			} else {
				refund.Status = config.OrderRefundStatusFailed
			}
			refund.Remark = bm.GetString("refund_status")
			parsedTime, err := time.Parse("2006-01-02 15:04:05", bm.GetString("success_time"))
			if err != nil {
				parsedTime = time.Now()
			}
			refund.RefundTime = parsedTime.Unix()

			_ = provider.SuccessRefundOrder(refund, order)
		}
	} else {
		// this is a pay order
		payment.PaidWay = "alipay"
		payment.PaidTime = time.Now().Unix()
		payment.TerraceId = bm.GetString("trade_no")
		dao.DB.Save(payment)
		//生成用户支付记录
		var userBalance int64
		err = dao.DB.Model(&model.User{}).Where("`id` = ?", payment.UserId).Pluck("balance", &userBalance).Error
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
		err = dao.DB.Create(&finance).Error
		if err != nil {
			ctx.WriteString("success")
			return
		}
		//支付成功逻辑处理
		err = provider.SuccessPaidOrder(order)
		if err != nil {
			ctx.WriteString("success")
			return
		}
	}

	//通知成功
	ctx.WriteString("success")
}

// SubscribeMsgPopup 订阅消息弹框事件
func SubscribeMsgPopup(popupEvent *server.SubscribeMsgPopupEvent) {
	for _, v := range popupEvent.SubscribeMsgPopupEvent {
		subscribedUser := model.SubscribedUser{
			Openid:     popupEvent.FromUserName,
			TemplateId: v.TemplateId,
		}
		if v.SubscribeStatusString == server.SubscribeResultAccept {
			//订阅
			dao.DB.Where("openid = ? AND template_id = ?", subscribedUser.Openid, subscribedUser.TemplateId).FirstOrCreate(&subscribedUser)
		} else {
			//拒绝
			dao.DB.Unscoped().Where("openid = ? AND template_id = ?", subscribedUser.Openid, subscribedUser.TemplateId).Delete(model.SubscribedUser{})
		}
	}
}

// SubscribeMsgChange 用户改变订阅消息事件
func SubscribeMsgChange(popupEvent *server.SubscribeMsgChangeEvent) {
	for _, v := range popupEvent.SubscribeMsgChangeEvent {
		subscribedUser := model.SubscribedUser{
			Openid:     popupEvent.FromUserName,
			TemplateId: v.TemplateId,
		}
		if v.SubscribeStatusString == server.SubscribeResultAccept {
			//订阅
			dao.DB.Where("openid = ? AND template_id = ?", subscribedUser.Openid, subscribedUser.TemplateId).FirstOrCreate(&subscribedUser)
		} else {
			//拒绝
			dao.DB.Unscoped().Where("openid = ? AND template_id = ?", subscribedUser.Openid, subscribedUser.TemplateId).Delete(model.SubscribedUser{})
		}
	}
}

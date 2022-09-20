package controller

import (
	"context"
	"github.com/go-pay/gopay"
	"github.com/go-pay/gopay/pkg/util"
	"github.com/go-pay/gopay/wechat"
	"github.com/kataras/iris/v12"
	"kandaoni.com/anqicms/config"
	"kandaoni.com/anqicms/model"
	"kandaoni.com/anqicms/provider"
	"kandaoni.com/anqicms/request"
	"strconv"
	"time"
)

func ApiGetOrders(ctx iris.Context) {
	currentPage := ctx.URLParamIntDefault("current", 1)
	pageSize := ctx.URLParamIntDefault("pageSize", 20)
	status := ctx.URLParam("status")

	userId := ctx.Values().GetUintDefault("userId", 0)

	orders, total := provider.GetOrderList(userId, status, currentPage, pageSize)

	ctx.JSON(iris.Map{
		"code":  config.StatusOK,
		"msg":   "",
		"total": total,
		"data":  orders,
	})
}

func ApiGetOrderDetail(ctx iris.Context) {
	orderId := ctx.URLParam("order_id")
	userId := ctx.Values().GetUintDefault("userId", 0)

	order, err := provider.GetOrderInfoByOrderId(orderId)
	if err != nil {
		ctx.JSON(iris.Map{
			"code": config.StatusFailed,
			"msg":  err.Error(),
		})
		return
	}
	if order.UserId != userId {
		ctx.JSON(iris.Map{
			"code": config.StatusFailed,
			"msg":  "权限不足",
		})
		return
	}

	ctx.JSON(iris.Map{
		"code": config.StatusOK,
		"msg":  "",
		"data": order,
	})
}

func ApiCreateOrder(ctx iris.Context) {
	var req request.OrderRequest
	if err := ctx.ReadJSON(&req); err != nil {
		ctx.JSON(iris.Map{
			"code": config.StatusFailed,
			"msg":  err.Error(),
		})
		return
	}
	userId := ctx.Values().GetUintDefault("userId", 0)

	order, err := provider.CreateOrder(userId, &req)
	if err != nil {
		ctx.JSON(iris.Map{
			"code": config.StatusFailed,
			"msg":  err.Error(),
		})
		return
	}

	ctx.JSON(iris.Map{
		"code": config.StatusOK,
		"msg":  "",
		"data": order,
	})
}

func ApiCancelOrder(ctx iris.Context) {
	var req request.OrderRequest
	if err := ctx.ReadJSON(&req); err != nil {
		ctx.JSON(iris.Map{
			"code": config.StatusFailed,
			"msg":  err.Error(),
		})
		return
	}
	userId := ctx.Values().GetUintDefault("userId", 0)

	order, err := provider.GetOrderInfoByOrderId(req.OrderId)
	if err != nil {
		ctx.JSON(iris.Map{
			"code": config.StatusFailed,
			"msg":  err.Error(),
		})
		return
	}
	if order.UserId != userId {
		ctx.JSON(iris.Map{
			"code": config.StatusFailed,
			"msg":  "该订单不可操作",
		})
		return
	}

	err = provider.SetOrderCanceled(order)
	if err != nil {
		ctx.JSON(iris.Map{
			"code": config.StatusFailed,
			"msg":  err.Error(),
		})
		return
	}

	ctx.JSON(iris.Map{
		"code": config.StatusOK,
		"msg":  "订单已取消",
	})
}

func ApiApplyRefundOrder(ctx iris.Context) {
	var req request.OrderRequest
	if err := ctx.ReadJSON(&req); err != nil {
		ctx.JSON(iris.Map{
			"code": config.StatusFailed,
			"msg":  err.Error(),
		})
		return
	}
	userId := ctx.Values().GetUintDefault("userId", 0)

	order, err := provider.GetOrderInfoByOrderId(req.OrderId)
	if err != nil {
		ctx.JSON(iris.Map{
			"code": config.StatusFailed,
			"msg":  err.Error(),
		})
		return
	}
	if order.UserId != userId {
		ctx.JSON(iris.Map{
			"code": config.StatusFailed,
			"msg":  "该订单不可操作",
		})
		return
	}

	err = provider.ApplyOrderRefund(order)
	if err != nil {
		ctx.JSON(iris.Map{
			"code": config.StatusFailed,
			"msg":  err.Error(),
		})
		return
	}

	ctx.JSON(iris.Map{
		"code": config.StatusOK,
		"msg":  "退款申请已提交",
	})
}

func ApiFinishedOrder(ctx iris.Context) {
	var req request.OrderRequest
	if err := ctx.ReadJSON(&req); err != nil {
		ctx.JSON(iris.Map{
			"code": config.StatusFailed,
			"msg":  err.Error(),
		})
		return
	}
	userId := ctx.Values().GetUintDefault("userId", 0)

	order, err := provider.GetOrderInfoByOrderId(req.OrderId)
	if err != nil {
		ctx.JSON(iris.Map{
			"code": config.StatusFailed,
			"msg":  err.Error(),
		})
		return
	}
	if order.UserId != userId {
		ctx.JSON(iris.Map{
			"code": config.StatusFailed,
			"msg":  "该订单不可操作",
		})
		return
	}

	err = provider.SetOrderFinished(order)
	if err != nil {
		ctx.JSON(iris.Map{
			"code": config.StatusFailed,
			"msg":  err.Error(),
		})
		return
	}

	ctx.JSON(iris.Map{
		"code": config.StatusOK,
		"msg":  "订单已完成",
	})
}

func ApiGetOrderAddress(ctx iris.Context) {
	userId := ctx.Values().GetUintDefault("userId", 0)

	address, err := provider.GetOrderAddressByUserId(userId)
	if err != nil {
		ctx.JSON(iris.Map{
			"code": config.StatusFailed,
			"msg":  err.Error(),
		})
		return
	}

	ctx.JSON(iris.Map{
		"code": config.StatusOK,
		"msg":  nil,
		"data": address,
	})
}

func ApiSaveOrderAddress(ctx iris.Context) {
	var req request.OrderAddressRequest
	if err := ctx.ReadJSON(&req); err != nil {
		ctx.JSON(iris.Map{
			"code": config.StatusFailed,
			"msg":  err.Error(),
		})
		return
	}
	userId := ctx.Values().GetUintDefault("userId", 0)

	address, err := provider.GetOrderAddressByUserId(userId)
	if err != nil {
		ctx.JSON(iris.Map{
			"code": config.StatusFailed,
			"msg":  err.Error(),
		})
		return
	}

	ctx.JSON(iris.Map{
		"code": config.StatusOK,
		"msg":  nil,
		"data": address,
	})
}

func ApiCreateOrderPayment(ctx iris.Context) {
	var req request.OrderRequest
	if err := ctx.ReadJSON(&req); err != nil {
		ctx.JSON(iris.Map{
			"code": config.StatusFailed,
			"msg":  err.Error(),
		})
		return
	}

	userId := ctx.Values().GetUintDefault("userId", 0)
	//注入userID
	req.UserId = userId

	order, err := provider.GetOrderInfoByOrderId(req.OrderId)
	if err != nil {
		ctx.JSON(iris.Map{
			"code": config.StatusFailed,
			"msg":  err.Error(),
		})
		return
	}

	payment, err := provider.GeneratePayment(order)
	if err != nil {
		ctx.JSON(iris.Map{
			"code": config.StatusFailed,
			"msg":  err.Error(),
		})
		return
	}

	createWeixinPayment(ctx, payment)
}

func createWeixinPayment(ctx iris.Context, payment *model.Payment) {
	//根据订单生成支付信息
	//生成支付信息
	client := wechat.NewClient(config.JsonData.PluginPay.WeixinAppId, config.JsonData.PluginPay.WeixinMchId, config.JsonData.PluginPay.WeixinApiKey, true)

	userWeixin, err := provider.GetUserWeixinByUserId(payment.UserId)
	if err != nil {
		ctx.JSON(iris.Map{
			"code": config.StatusFailed,
			"msg":  err.Error(),
		})
		return
	}

	bm := make(gopay.BodyMap)
	bm.Set("body", payment.Remark).
		Set("nonce_str", util.RandomString(32)).
		Set("spbill_create_ip", ctx.RemoteAddr()).
		Set("out_trade_no", payment.PaymentId). // 传的是paymentID，因此notify的时候，需要处理paymentID
		Set("total_fee", payment.Amount).
		Set("trade_type", wechat.TradeType_Mini).
		Set("notify_url", config.JsonData.System.BaseUrl+"/notify/weixin/pay").
		Set("sign_type", wechat.SignType_MD5).
		Set("openid", userWeixin.Openid)

	wxRsp, err := client.UnifiedOrder(context.Background(), bm)
	if err != nil {
		ctx.JSON(iris.Map{
			"code": config.StatusFailed,
			"msg":  err.Error(),
		})
		return
	}

	if wxRsp.ReturnCode != gopay.SUCCESS {
		ctx.JSON(iris.Map{
			"code": config.StatusFailed,
			"msg":  wxRsp.ReturnMsg,
		})
		return
	}

	if wxRsp.ResultCode != gopay.SUCCESS {
		ctx.JSON(iris.Map{
			"code": config.StatusFailed,
			"msg":  wxRsp.ErrCodeDes,
		})
		return
	}

	// 微信小程序支付需要 paySign
	timeStamp := strconv.FormatInt(time.Now().Unix(), 10)
	packages := "prepay_id=" + wxRsp.PrepayId
	paySign := wechat.GetMiniPaySign(config.JsonData.PluginPay.WeixinAppId, wxRsp.NonceStr, packages, wechat.SignType_MD5, timeStamp, config.JsonData.PluginPay.WeixinApiKey)

	ctx.JSON(iris.Map{
		"code": config.StatusOK,
		"msg":  "",
		"data": iris.Map{
			"code_url":  wxRsp.CodeUrl,
			"paySign":   paySign,
			"timeStamp": timeStamp,
			"package":  packages,
			"nonceStr":  wxRsp.NonceStr,
			"signType": wechat.SignType_MD5,
		},
	})
	return
}

func ApiPaymentCheck(ctx iris.Context) {
	orderId := ctx.URLParam("order_id")
	order, err := provider.GetOrderInfoByOrderId(orderId)
	if err != nil {
		ctx.JSON(iris.Map{
			"code": config.StatusFailed,
			"msg":  err.Error(),
		})
		return
	}
	if order.Status == config.OrderStatusPaid {
		//支付成功
		ctx.JSON(iris.Map{
			"code": config.StatusOK,
			"msg":  "支付成功",
		})
		return
	}

	for i := 0; i < 20; i++ {
		order, _ := provider.GetOrderInfoByOrderId(orderId)
		if order.Status == config.OrderStatusPaid {
			//支付成功
			ctx.JSON(iris.Map{
				"code": config.StatusOK,
				"msg":  "支付成功",
			})
			return
		}
		time.Sleep(1 * time.Second)
	}

	ctx.JSON(iris.Map{
		"code": config.StatusFailed,
		"msg":  "未支付",
	})
}

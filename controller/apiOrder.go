package controller

import (
	"context"
	"encoding/base64"
	"fmt"
	"github.com/go-pay/gopay"
	"github.com/go-pay/gopay/alipay"
	"github.com/go-pay/gopay/pkg/util"
	"github.com/go-pay/gopay/wechat"
	"github.com/kataras/iris/v12"
	"github.com/skip2/go-qrcode"
	"kandaoni.com/anqicms/config"
	"kandaoni.com/anqicms/model"
	"kandaoni.com/anqicms/provider"
	"kandaoni.com/anqicms/request"
	"os"
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
			"msg":  config.Lang("权限不足"),
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
			"msg":  config.Lang("该订单不可操作"),
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
		"msg":  config.Lang("订单已取消"),
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
			"msg":  config.Lang("该订单不可操作"),
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
		"msg":  config.Lang("退款申请已提交"),
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
			"msg":  config.Lang("该订单不可操作"),
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
		"msg":  config.Lang("订单已完成"),
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
	var req request.PaymentRequest
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

	payment, err := provider.GeneratePayment(order, req.PayWay)
	if err != nil {
		ctx.JSON(iris.Map{
			"code": config.StatusFailed,
			"msg":  err.Error(),
		})
		return
	}
	if req.PayWay == "" {
		req.PayWay = payment.PayWay
	}

	if req.PayWay == config.PayWayWechat {
		createWechatPayment(ctx, payment)
	} else if req.PayWay == config.PayWayWeapp {
		createWeappPayment(ctx, payment)
	} else if req.PayWay == config.PayWayAlipay {
		createAlipayPayment(ctx, payment)
	} else {
		ctx.JSON(iris.Map{
			"code": config.StatusFailed,
			"msg":  config.Lang("无法创建支付订单"),
		})
	}
}

func createWechatPayment(ctx iris.Context, payment *model.Payment) {
	//根据订单生成支付信息
	//生成支付信息
	client := wechat.NewClient(config.JsonData.PluginPay.WechatAppId, config.JsonData.PluginPay.WechatMchId, config.JsonData.PluginPay.WechatApiKey, true)

	bm := make(gopay.BodyMap)
	bm.Set("body", payment.Remark).
		Set("nonce_str", util.RandomString(32)).
		Set("spbill_create_ip", ctx.RemoteAddr()).
		Set("out_trade_no", payment.PaymentId). // 传的是paymentID，因此notify的时候，需要处理paymentID
		Set("total_fee", payment.Amount).
		Set("trade_type", wechat.TradeType_Native).
		Set("notify_url", config.JsonData.System.BaseUrl+"/notify/wechat/pay").
		Set("sign_type", wechat.SignType_MD5)

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

	png, _ := qrcode.Encode(wxRsp.CodeUrl, qrcode.Medium, 256)

	ctx.JSON(iris.Map{
		"code": config.StatusOK,
		"msg":  "",
		"data": iris.Map{
			"pay_way":  "wechat",
			"code_url": fmt.Sprintf("data:image/png;base64,%s", base64.StdEncoding.EncodeToString(png)),
		},
	})
	return
}

func createWeappPayment(ctx iris.Context, payment *model.Payment) {
	//根据订单生成支付信息
	//生成支付信息
	client := wechat.NewClient(config.JsonData.PluginPay.WeappAppId, config.JsonData.PluginPay.WechatMchId, config.JsonData.PluginPay.WechatApiKey, true)

	userWechat, err := provider.GetUserWechatByUserId(payment.UserId)
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
		Set("notify_url", config.JsonData.System.BaseUrl+"/notify/wechat/pay").
		Set("sign_type", wechat.SignType_MD5).
		Set("openid", userWechat.Openid)

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
	paySign := wechat.GetMiniPaySign(config.JsonData.PluginPay.WeappAppId, wxRsp.NonceStr, packages, wechat.SignType_MD5, timeStamp, config.JsonData.PluginPay.WechatApiKey)

	ctx.JSON(iris.Map{
		"code": config.StatusOK,
		"msg":  "",
		"data": iris.Map{
			"pay_way":   "weapp",
			"paySign":   paySign,
			"timeStamp": timeStamp,
			"package":   packages,
			"nonceStr":  wxRsp.NonceStr,
			"signType":  wechat.SignType_MD5,
		},
	})
	return
}

func createAlipayPayment(ctx iris.Context, payment *model.Payment) {
	client, err := alipay.NewClient(config.JsonData.PluginPay.AlipayAppId, config.JsonData.PluginPay.AlipayPrivateKey, true)
	if err != nil {
		ctx.JSON(iris.Map{
			"code": config.StatusFailed,
			"msg":  err.Error(),
		})
		return
	}

	//配置公共参数
	client.SetCharset("utf-8").
		SetSignType(alipay.RSA2).
		SetNotifyUrl(config.JsonData.System.BaseUrl + "/notify/alipay/pay").
		SetReturnUrl(config.JsonData.System.BaseUrl + "/")

	// 自动同步验签（只支持证书模式）
	certPath := fmt.Sprintf("%sdata/cert/alipay_cert_path.pem", config.ExecPath)
	rootCertPath := fmt.Sprintf("%sdata/cert/alipay_root_cert_path.pem", config.ExecPath)
	publicCertPath := fmt.Sprintf("%sdata/cert/alipay_public_cert_path.pem", config.ExecPath)
	publicKey, err := os.ReadFile(publicCertPath)
	if err != nil {
		ctx.JSON(iris.Map{
			"code": config.StatusFailed,
			"msg":  err.Error(),
		})
		return
	}
	client.AutoVerifySign(publicKey)

	// 传入证书内容
	err = client.SetCertSnByPath(certPath, rootCertPath, publicCertPath)
	if err != nil {
		ctx.JSON(iris.Map{
			"code": config.StatusFailed,
			"msg":  err.Error(),
		})
		return
	}

	//请求参数
	bm := make(gopay.BodyMap)

	bm.Set("subject", payment.Remark)
	bm.Set("out_trade_no", payment.PaymentId)
	bm.Set("total_amount", fmt.Sprintf("%.2f", float32(payment.Amount)/100))

	//创建订单
	payUrl, err := client.TradePagePay(context.Background(), bm)

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
		"data": iris.Map{
			"pay_way":  "alipay",
			"jump_url": payUrl,
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
	if order.Status != config.OrderStatusWaiting {
		//支付成功
		ctx.JSON(iris.Map{
			"code": config.StatusOK,
			"msg":  config.Lang("支付成功"),
		})
		return
	}

	for i := 0; i < 20; i++ {
		order, _ := provider.GetOrderInfoByOrderId(orderId)
		if order.Status != config.OrderStatusWaiting {
			//支付成功
			ctx.JSON(iris.Map{
				"code": config.StatusOK,
				"msg":  config.Lang("支付成功"),
			})
			return
		}
		time.Sleep(1 * time.Second)
	}

	ctx.JSON(iris.Map{
		"code": config.StatusFailed,
		"msg":  config.Lang("未支付"),
	})
}

func ApiArchiveOrderCheck(ctx iris.Context) {
	archiveId := uint(ctx.URLParamIntDefault("id", 0))
	userId := ctx.Values().GetUintDefault("userId", 0)
	exist := provider.CheckArchiveHasOrder(userId, archiveId)

	ctx.JSON(iris.Map{
		"code": config.StatusOK,
		"msg":  "",
		"data": exist,
	})
	return
}

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
	currentSite := provider.CurrentSite(ctx)
	currentPage := ctx.URLParamIntDefault("current", 1)
	pageSize := ctx.URLParamIntDefault("pageSize", 20)
	status := ctx.URLParam("status")

	userId := ctx.Values().GetUintDefault("userId", 0)

	orders, total := currentSite.GetOrderList(userId, status, currentPage, pageSize)

	ctx.JSON(iris.Map{
		"code":  config.StatusOK,
		"msg":   "",
		"total": total,
		"data":  orders,
	})
}

func ApiGetOrderDetail(ctx iris.Context) {
	currentSite := provider.CurrentSite(ctx)
	orderId := ctx.URLParam("order_id")
	userId := ctx.Values().GetUintDefault("userId", 0)

	order, err := currentSite.GetOrderInfoByOrderId(orderId)
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
			"msg":  currentSite.Lang("权限不足"),
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
	currentSite := provider.CurrentSite(ctx)
	var req request.OrderRequest
	if err := ctx.ReadJSON(&req); err != nil {
		ctx.JSON(iris.Map{
			"code": config.StatusFailed,
			"msg":  err.Error(),
		})
		return
	}
	userId := ctx.Values().GetUintDefault("userId", 0)

	order, err := currentSite.CreateOrder(userId, &req)
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
	currentSite := provider.CurrentSite(ctx)
	var req request.OrderRequest
	if err := ctx.ReadJSON(&req); err != nil {
		ctx.JSON(iris.Map{
			"code": config.StatusFailed,
			"msg":  err.Error(),
		})
		return
	}
	userId := ctx.Values().GetUintDefault("userId", 0)

	order, err := currentSite.GetOrderInfoByOrderId(req.OrderId)
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
			"msg":  currentSite.Lang("该订单不可操作"),
		})
		return
	}

	err = currentSite.SetOrderCanceled(order)
	if err != nil {
		ctx.JSON(iris.Map{
			"code": config.StatusFailed,
			"msg":  err.Error(),
		})
		return
	}

	ctx.JSON(iris.Map{
		"code": config.StatusOK,
		"msg":  currentSite.Lang("订单已取消"),
	})
}

func ApiApplyRefundOrder(ctx iris.Context) {
	currentSite := provider.CurrentSite(ctx)
	var req request.OrderRequest
	if err := ctx.ReadJSON(&req); err != nil {
		ctx.JSON(iris.Map{
			"code": config.StatusFailed,
			"msg":  err.Error(),
		})
		return
	}
	userId := ctx.Values().GetUintDefault("userId", 0)

	order, err := currentSite.GetOrderInfoByOrderId(req.OrderId)
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
			"msg":  currentSite.Lang("该订单不可操作"),
		})
		return
	}

	err = currentSite.ApplyOrderRefund(order)
	if err != nil {
		ctx.JSON(iris.Map{
			"code": config.StatusFailed,
			"msg":  err.Error(),
		})
		return
	}

	ctx.JSON(iris.Map{
		"code": config.StatusOK,
		"msg":  currentSite.Lang("退款申请已提交"),
	})
}

func ApiFinishedOrder(ctx iris.Context) {
	currentSite := provider.CurrentSite(ctx)
	var req request.OrderRequest
	if err := ctx.ReadJSON(&req); err != nil {
		ctx.JSON(iris.Map{
			"code": config.StatusFailed,
			"msg":  err.Error(),
		})
		return
	}
	userId := ctx.Values().GetUintDefault("userId", 0)

	order, err := currentSite.GetOrderInfoByOrderId(req.OrderId)
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
			"msg":  currentSite.Lang("该订单不可操作"),
		})
		return
	}

	err = currentSite.SetOrderFinished(order)
	if err != nil {
		ctx.JSON(iris.Map{
			"code": config.StatusFailed,
			"msg":  err.Error(),
		})
		return
	}

	ctx.JSON(iris.Map{
		"code": config.StatusOK,
		"msg":  currentSite.Lang("订单已完成"),
	})
}

func ApiGetOrderAddress(ctx iris.Context) {
	currentSite := provider.CurrentSite(ctx)
	userId := ctx.Values().GetUintDefault("userId", 0)

	address, err := currentSite.GetOrderAddressByUserId(userId)
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
	currentSite := provider.CurrentSite(ctx)
	var req request.OrderAddressRequest
	if err := ctx.ReadJSON(&req); err != nil {
		ctx.JSON(iris.Map{
			"code": config.StatusFailed,
			"msg":  err.Error(),
		})
		return
	}
	userId := ctx.Values().GetUintDefault("userId", 0)

	address, err := currentSite.GetOrderAddressByUserId(userId)
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
	currentSite := provider.CurrentSite(ctx)
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

	order, err := currentSite.GetOrderInfoByOrderId(req.OrderId)
	if err != nil {
		ctx.JSON(iris.Map{
			"code": config.StatusFailed,
			"msg":  err.Error(),
		})
		return
	}

	payment, err := currentSite.GeneratePayment(order, req.PayWay)
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
			"msg":  currentSite.Lang("无法创建支付订单"),
		})
	}
}

func createWechatPayment(ctx iris.Context, payment *model.Payment) {
	currentSite := provider.CurrentSite(ctx)
	//根据订单生成支付信息
	//生成支付信息
	client := wechat.NewClient(currentSite.PluginPay.WechatAppId, currentSite.PluginPay.WechatMchId, currentSite.PluginPay.WechatApiKey, true)

	bm := make(gopay.BodyMap)
	bm.Set("body", payment.Remark).
		Set("nonce_str", util.RandomString(32)).
		Set("spbill_create_ip", ctx.RemoteAddr()).
		Set("out_trade_no", payment.PaymentId). // 传的是paymentID，因此notify的时候，需要处理paymentID
		Set("total_fee", payment.Amount).
		Set("trade_type", wechat.TradeType_Native).
		Set("notify_url", currentSite.System.BaseUrl+"/notify/wechat/pay").
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
	currentSite := provider.CurrentSite(ctx)
	//根据订单生成支付信息
	//生成支付信息
	client := wechat.NewClient(currentSite.PluginPay.WeappAppId, currentSite.PluginPay.WechatMchId, currentSite.PluginPay.WechatApiKey, true)

	userWechat, err := currentSite.GetUserWechatByUserId(payment.UserId)
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
		Set("notify_url", currentSite.System.BaseUrl+"/notify/wechat/pay").
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
	paySign := wechat.GetMiniPaySign(currentSite.PluginPay.WeappAppId, wxRsp.NonceStr, packages, wechat.SignType_MD5, timeStamp, currentSite.PluginPay.WechatApiKey)

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
	currentSite := provider.CurrentSite(ctx)
	client, err := alipay.NewClient(currentSite.PluginPay.AlipayAppId, currentSite.PluginPay.AlipayPrivateKey, true)
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
		SetNotifyUrl(currentSite.System.BaseUrl + "/notify/alipay/pay").
		SetReturnUrl(currentSite.System.BaseUrl + "/")

	// 自动同步验签（只支持证书模式）
	certPath := fmt.Sprintf(currentSite.DataPath + "cert/" + currentSite.PluginPay.AlipayCertPath)
	rootCertPath := fmt.Sprintf(currentSite.DataPath + "cert/" + currentSite.PluginPay.AlipayRootCertPath)
	publicCertPath := fmt.Sprintf(currentSite.DataPath + "cert/" + currentSite.PluginPay.AlipayPublicCertPath)
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
	currentSite := provider.CurrentSite(ctx)
	orderId := ctx.URLParam("order_id")
	order, err := currentSite.GetOrderInfoByOrderId(orderId)
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
			"msg":  currentSite.Lang("支付成功"),
		})
		return
	}

	for i := 0; i < 20; i++ {
		order, _ := currentSite.GetOrderInfoByOrderId(orderId)
		if order.Status != config.OrderStatusWaiting {
			//支付成功
			ctx.JSON(iris.Map{
				"code": config.StatusOK,
				"msg":  currentSite.Lang("支付成功"),
			})
			return
		}
		time.Sleep(1 * time.Second)
	}

	ctx.JSON(iris.Map{
		"code": config.StatusFailed,
		"msg":  currentSite.Lang("未支付"),
	})
}

func ApiArchiveOrderCheck(ctx iris.Context) {
	currentSite := provider.CurrentSite(ctx)
	archiveId := uint(ctx.URLParamIntDefault("id", 0))
	userId := ctx.Values().GetUintDefault("userId", 0)

	archiveDetail, err := currentSite.GetArchiveById(archiveId)
	if err != nil {
		ctx.JSON(iris.Map{
			"code": config.StatusOK,
			"msg":  "",
			"data": false,
		})
	}
	if archiveDetail.Price == 0 && archiveDetail.ReadLevel == 0 {
		archiveDetail.HasOrdered = true
	}
	if userId > 0 {
		if archiveDetail.UserId == userId {
			archiveDetail.HasOrdered = true
		} else if archiveDetail.Price > 0 {
			archiveDetail.HasOrdered = currentSite.CheckArchiveHasOrder(userId, archiveDetail.Id)
		}
		if archiveDetail.ReadLevel > 0 && !archiveDetail.HasOrdered {
			userGroup, _ := ctx.Values().Get("userGroup").(*model.UserGroup)
			if userGroup != nil && userGroup.Level >= archiveDetail.ReadLevel {
				archiveDetail.HasOrdered = true
			}
		}
	}

	ctx.JSON(iris.Map{
		"code": config.StatusOK,
		"msg":  "",
		"data": archiveDetail.HasOrdered,
	})
	return
}

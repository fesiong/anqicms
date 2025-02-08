package controller

import (
	"context"
	"encoding/base64"
	"fmt"
	"github.com/go-pay/gopay"
	"github.com/go-pay/gopay/alipay"
	"github.com/go-pay/gopay/paypal"
	"github.com/go-pay/gopay/wechat"
	"github.com/kataras/iris/v12"
	"github.com/skip2/go-qrcode"
	"kandaoni.com/anqicms/config"
	"kandaoni.com/anqicms/library"
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

	orders, total := currentSite.GetOrderList(userId, "", "", status, currentPage, pageSize)

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
			"msg":  currentSite.TplTr("InsufficientPermissions"),
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
			"msg":  currentSite.TplTr("TheOrderIsNotOperational"),
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
		"msg":  currentSite.TplTr("OrderCanceled"),
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
			"msg":  currentSite.TplTr("TheOrderIsNotOperational"),
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
		"msg":  currentSite.TplTr("RefundApplicationSubmitted"),
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
			"msg":  currentSite.TplTr("TheOrderIsNotOperational"),
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
		"msg":  currentSite.TplTr("OrderCompleted"),
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

	address, err := currentSite.SaveOrderAddress(currentSite.DB, userId, &req)
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
	} else if req.PayWay == config.PayWayPaypal {
		createPaypalPayment(ctx, payment, order)
	} else {
		ctx.JSON(iris.Map{
			"code": config.StatusFailed,
			"msg":  currentSite.TplTr("UnableToCreatePaymentOrder"),
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
		Set("nonce_str", library.GenerateRandString(32)).
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
		Set("nonce_str", library.GenerateRandString(32)).
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

func createPaypalPayment(ctx iris.Context, payment *model.Payment, order *model.Order) {
	currentSite := provider.CurrentSite(ctx)
	client, err := paypal.NewClient(currentSite.PluginPay.PaypalClientId, currentSite.PluginPay.PaypalClientSecret, true)
	if err != nil {
		ctx.JSON(iris.Map{
			"code": config.StatusFailed,
			"msg":  err.Error(),
		})
		return
	}

	var purchases = []*paypal.PurchaseUnit{
		{
			ReferenceId: payment.PaymentId,
			Amount: &paypal.Amount{
				CurrencyCode: "USD",
				Value:        fmt.Sprintf("%.2f", float32(payment.Amount)/100),
			},
		},
	}

	bm := make(gopay.BodyMap)
	bm.Set("intent", "CAPTURE").
		Set("purchase_units", purchases).
		SetBodyMap("payment_source", func(b gopay.BodyMap) {
			b.SetBodyMap("paypal", func(bb gopay.BodyMap) {
				bb.SetBodyMap("experience_context", func(bbb gopay.BodyMap) {
					bbb.Set("brand_name", currentSite.System.SiteName).
						Set("locale", "en-US").
						Set("shipping_preference", "NO_SHIPPING").
						Set("user_action", "PAY_NOW").
						Set("return_url", currentSite.System.BaseUrl+"/return/paypal/pay").
						Set("cancel_url", currentSite.System.BaseUrl+"/return/paypal/cancel")
				})
			})
		})

	ppRsp, err := client.CreateOrder(ctx, bm)
	if err != nil {
		ctx.JSON(iris.Map{
			"code": config.StatusFailed,
			"msg":  err.Error(),
		})
		return
	}
	if ppRsp.Code != 200 {
		// do something
		ctx.JSON(iris.Map{
			"code": config.StatusFailed,
			"msg":  ppRsp.Error,
		})
		return
	}
	// 更新id
	payment.TerraceId = ppRsp.Response.Id
	currentSite.DB.Save(payment)

	// payer link

	ctx.JSON(iris.Map{
		"code": config.StatusOK,
		"msg":  "",
		"data": iris.Map{
			"pay_way":  "paypal",
			"jump_url": ppRsp.Response.Links[1].Href,
		},
	})
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
			"msg":  currentSite.TplTr("PaymentSuccessful"),
		})
		return
	}

	for i := 0; i < 20; i++ {
		order, _ = currentSite.GetOrderInfoByOrderId(orderId)
		if order.Status != config.OrderStatusWaiting {
			//支付成功
			ctx.JSON(iris.Map{
				"code": config.StatusOK,
				"msg":  currentSite.TplTr("PaymentSuccessful"),
			})
			return
		}
		time.Sleep(1 * time.Second)
	}

	ctx.JSON(iris.Map{
		"code": config.StatusFailed,
		"msg":  currentSite.TplTr("Unpaid"),
	})
}

func ApiArchiveOrderCheck(ctx iris.Context) {
	currentSite := provider.CurrentSite(ctx)
	archiveId := ctx.URLParamInt64Default("id", 0)
	userId := ctx.Values().GetUintDefault("userId", 0)

	archiveDetail, err := currentSite.GetArchiveById(archiveId)
	if err != nil {
		ctx.JSON(iris.Map{
			"code": config.StatusOK,
			"msg":  "",
			"data": false,
		})
	}
	userGroup, _ := ctx.Values().Get("userGroup").(*model.UserGroup)
	archiveDetail = currentSite.CheckArchiveHasOrder(userId, archiveDetail, userGroup)

	ctx.JSON(iris.Map{
		"code": config.StatusOK,
		"msg":  "",
		"data": archiveDetail.HasOrdered,
	})
	return
}

func ApiCheckArchivePassword(ctx iris.Context) {
	currentSite := provider.CurrentSite(ctx)
	var req request.ArchivePasswordRequest
	var err error
	if err = ctx.ReadJSON(&req); err != nil {
		req.Id = ctx.PostValueInt64Default("id", 0)
		req.Password = ctx.PostValueTrim("password")
		if req.Id == 0 || len(req.Password) == 0 {
			ctx.JSON(iris.Map{
				"code": config.StatusFailed,
				"msg":  err.Error(),
			})
			return
		}
	}

	archiveDetail, err := currentSite.GetArchiveById(req.Id)
	if err != nil {
		ctx.JSON(iris.Map{
			"code": config.StatusFailed,
			"msg":  err.Error(),
		})
		return
	}
	if len(archiveDetail.Password) == 0 || archiveDetail.Password == req.Password {
		var content string
		archiveData, err := currentSite.GetArchiveDataById(archiveDetail.Id)
		if err == nil {
			content = archiveData.Content
			// render
			if currentSite.Content.Editor == "markdown" {
				content = library.MarkdownToHTML(archiveData.Content, currentSite.System.BaseUrl, currentSite.Content.FilterOutlink)
			}
		}
		ctx.JSON(iris.Map{
			"code": config.StatusOK,
			"msg":  "",
			"data": iris.Map{
				"status":  true,
				"content": content,
			},
		})
		return
	}

	ctx.JSON(iris.Map{
		"code": config.StatusOK,
		"msg":  "",
		"data": iris.Map{
			"status":  false,
			"content": "",
		},
	})
}

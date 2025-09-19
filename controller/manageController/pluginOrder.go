package manageController

import (
	"fmt"
	"github.com/kataras/iris/v12"
	"kandaoni.com/anqicms/config"
	"kandaoni.com/anqicms/model"
	"kandaoni.com/anqicms/provider"
	"kandaoni.com/anqicms/request"
	"time"
)

func PluginOrderList(ctx iris.Context) {
	currentSite := provider.CurrentSite(ctx)
	currentPage := ctx.URLParamIntDefault("current", 1)
	pageSize := ctx.URLParamIntDefault("pageSize", 20)
	orderId := ctx.URLParam("order_id")
	userName := ctx.URLParam("user_name")
	status := ctx.URLParam("status")

	orders, total := currentSite.GetOrderList(0, orderId, userName, status, currentPage, pageSize)

	ctx.JSON(iris.Map{
		"code":  config.StatusOK,
		"msg":   "",
		"total": total,
		"data":  orders,
	})
}

func PluginOrderDetail(ctx iris.Context) {
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

	order.User, _ = currentSite.GetUserInfoById(order.UserId)
	if order.ShareUserId > 0 {
		order.ShareUser, _ = currentSite.GetUserInfoById(order.ShareUserId)
	}
	if order.ShareParentUserId > 0 {
		order.ParentUser, _ = currentSite.GetUserInfoById(order.ShareParentUserId)
	}

	ctx.JSON(iris.Map{
		"code": config.StatusOK,
		"msg":  "",
		"data": order,
	})
}

func PluginOrderSetPay(ctx iris.Context) {
	currentSite := provider.CurrentSite(ctx)
	var req request.PaymentRequest
	if err := ctx.ReadJSON(&req); err != nil {
		ctx.JSON(iris.Map{
			"code": config.StatusFailed,
			"msg":  err.Error(),
		})
		return
	}
	if req.PayWay == "" {
		req.PayWay = config.PayWayOffline
	}

	payment, err := currentSite.GetPaymentInfoByOrderId(req.OrderId)
	if err != nil {
		// 生成一个payment
		order, err := currentSite.GetOrderInfoByOrderId(req.OrderId)
		if err != nil {
			ctx.JSON(iris.Map{
				"code": config.StatusFailed,
				"msg":  err.Error(),
			})
			return
		}
		payment, err = currentSite.GeneratePayment(order, req.PayWay)
		if err != nil {
			ctx.JSON(iris.Map{
				"code": config.StatusFailed,
				"msg":  err.Error(),
			})
			return
		}
	}

	if payment.PaidTime > 0 {
		ctx.JSON(iris.Map{
			"code": config.StatusFailed,
			"msg":  ctx.Tr("ThisOrderHasBeenPaid"),
		})
		return
	}
	order, err := currentSite.GetOrderInfoByOrderId(payment.OrderId)
	if err != nil {
		ctx.JSON(iris.Map{
			"code": config.StatusFailed,
			"msg":  ctx.Tr("OrderDoesNotExist"),
		})
		return
	}
	if order.PaidTime > 0 {
		ctx.JSON(iris.Map{
			"code": config.StatusFailed,
			"msg":  ctx.Tr("ThisOrderHasBeenPaid"),
		})
		return
	}

	// this is a pay order
	payment.PayWay = req.PayWay
	payment.PaidTime = time.Now().Unix()
	payment.TerraceId = fmt.Sprintf("%d", payment.PaidTime)
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
		ctx.JSON(iris.Map{
			"code": config.StatusFailed,
			"msg":  ctx.Tr("PaymentFailed"),
		})
		return
	}

	//支付成功逻辑处理
	err = currentSite.SuccessPaidOrder(order)
	if err != nil {
		ctx.JSON(iris.Map{
			"code": config.StatusFailed,
			"msg":  ctx.Tr("PaymentFailed"),
		})
		return
	}

	ctx.JSON(iris.Map{
		"code": config.StatusOK,
		"msg":  ctx.Tr("PaymentSuccessful"),
	})
}

func PluginOrderSetDeliver(ctx iris.Context) {
	currentSite := provider.CurrentSite(ctx)
	var req request.OrderRequest
	if err := ctx.ReadJSON(&req); err != nil {
		ctx.JSON(iris.Map{
			"code": config.StatusFailed,
			"msg":  err.Error(),
		})
		return
	}

	err := currentSite.SetOrderDeliver(&req)
	if err != nil {
		ctx.JSON(iris.Map{
			"code": config.StatusFailed,
			"msg":  err.Error(),
		})
		return
	}

	ctx.JSON(iris.Map{
		"code": config.StatusOK,
		"msg":  ctx.Tr("SetSuccessfully"),
	})
}

func PluginOrderSetFinished(ctx iris.Context) {
	currentSite := provider.CurrentSite(ctx)
	var req request.OrderRequest
	if err := ctx.ReadJSON(&req); err != nil {
		ctx.JSON(iris.Map{
			"code": config.StatusFailed,
			"msg":  err.Error(),
		})
		return
	}

	order, err := currentSite.GetOrderInfoByOrderId(req.OrderId)
	if err != nil {
		ctx.JSON(iris.Map{
			"code": config.StatusFailed,
			"msg":  err.Error(),
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
		"msg":  ctx.Tr("SetSuccessfully"),
	})
}

func PluginOrderSetCanceled(ctx iris.Context) {
	currentSite := provider.CurrentSite(ctx)
	var req request.OrderRequest
	if err := ctx.ReadJSON(&req); err != nil {
		ctx.JSON(iris.Map{
			"code": config.StatusFailed,
			"msg":  err.Error(),
		})
		return
	}

	order, err := currentSite.GetOrderInfoByOrderId(req.OrderId)
	if err != nil {
		ctx.JSON(iris.Map{
			"code": config.StatusFailed,
			"msg":  err.Error(),
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
		"msg":  ctx.Tr("SetSuccessfully"),
	})
}

func PluginOrderSetRefund(ctx iris.Context) {
	currentSite := provider.CurrentSite(ctx)
	var req request.OrderRefundRequest
	if err := ctx.ReadJSON(&req); err != nil {
		ctx.JSON(iris.Map{
			"code": config.StatusFailed,
			"msg":  err.Error(),
		})
		return
	}

	order, err := currentSite.GetOrderInfoByOrderId(req.OrderId)
	if err != nil {
		ctx.JSON(iris.Map{
			"code": config.StatusFailed,
			"msg":  err.Error(),
		})
		return
	}

	err = currentSite.SetOrderRefund(order, req.Status)
	if err != nil {
		ctx.JSON(iris.Map{
			"code": config.StatusFailed,
			"msg":  err.Error(),
		})
		return
	}

	ctx.JSON(iris.Map{
		"code": config.StatusOK,
		"msg":  ctx.Tr("SetSuccessfully"),
	})
}

func PluginOrderApplyRefund(ctx iris.Context) {
	currentSite := provider.CurrentSite(ctx)
	var req request.OrderRefundRequest
	if err := ctx.ReadJSON(&req); err != nil {
		ctx.JSON(iris.Map{
			"code": config.StatusFailed,
			"msg":  err.Error(),
		})
		return
	}

	order, err := currentSite.GetOrderInfoByOrderId(req.OrderId)
	if err != nil {
		ctx.JSON(iris.Map{
			"code": config.StatusFailed,
			"msg":  err.Error(),
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

	err = currentSite.SetOrderRefund(order, 1)
	if err != nil {
		ctx.JSON(iris.Map{
			"code": config.StatusFailed,
			"msg":  err.Error(),
		})
		return
	}

	ctx.JSON(iris.Map{
		"code": config.StatusOK,
		"msg":  ctx.Tr("ApplicationSuccessful"),
	})
}

func PluginOrderConfig(ctx iris.Context) {
	currentSite := provider.CurrentSite(ctx)
	setting := currentSite.PluginOrder

	ctx.JSON(iris.Map{
		"code": config.StatusOK,
		"msg":  "",
		"data": setting,
	})
}

func PluginOrderConfigForm(ctx iris.Context) {
	currentSite := provider.CurrentSite(ctx)
	var req config.PluginOrderConfig
	if err := ctx.ReadJSON(&req); err != nil {
		ctx.JSON(iris.Map{
			"code": config.StatusFailed,
			"msg":  err.Error(),
		})
		return
	}

	currentSite.PluginOrder.NoProcess = req.NoProcess
	currentSite.PluginOrder.AutoFinishDay = req.AutoFinishDay
	currentSite.PluginOrder.AutoCloseMinute = req.AutoCloseMinute
	currentSite.PluginOrder.SellerPercent = req.SellerPercent

	err := currentSite.SaveSettingValue(provider.OrderSettingKey, currentSite.PluginOrder)
	if err != nil {
		ctx.JSON(iris.Map{
			"code": config.StatusFailed,
			"msg":  err.Error(),
		})
		return
	}
	currentSite.DeleteCacheIndex()

	currentSite.AddAdminLog(ctx, ctx.Tr("UpdateOrderConfiguration"))

	ctx.JSON(iris.Map{
		"code": config.StatusOK,
		"msg":  ctx.Tr("ConfigurationUpdated"),
	})
}

func PluginOrderExport(ctx iris.Context) {
	currentSite := provider.CurrentSite(ctx)
	var req request.OrderExportRequest
	if err := ctx.ReadJSON(&req); err != nil {
		ctx.JSON(iris.Map{
			"code": config.StatusFailed,
			"msg":  err.Error(),
		})
		return
	}

	header, content := currentSite.ExportOrders(&req)

	currentSite.AddAdminLog(ctx, ctx.Tr("ExportOrder"))

	ctx.JSON(iris.Map{
		"code": config.StatusOK,
		"msg":  "",
		"data": iris.Map{
			"header":  header,
			"content": content,
		},
	})
}

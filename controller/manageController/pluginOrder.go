package manageController

import (
	"fmt"
	"github.com/kataras/iris/v12"
	"kandaoni.com/anqicms/config"
	"kandaoni.com/anqicms/provider"
	"kandaoni.com/anqicms/request"
)

func PluginOrderList(ctx iris.Context) {
	currentSite := provider.CurrentSite(ctx)
	currentPage := ctx.URLParamIntDefault("current", 1)
	pageSize := ctx.URLParamIntDefault("pageSize", 20)

	orders, total := currentSite.GetOrderList(0, "", currentPage, pageSize)

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
		"msg":  "已设置成功",
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
		"msg":  "已设置成功",
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
		"msg":  "已设置成功",
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
		"msg":  "已设置成功",
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
		"msg":  "已申请成功",
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

	currentSite.AddAdminLog(ctx, fmt.Sprintf("更新订单配置信息"))

	ctx.JSON(iris.Map{
		"code": config.StatusOK,
		"msg":  "配置已更新",
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

	currentSite.AddAdminLog(ctx, fmt.Sprintf("导出订单"))

	ctx.JSON(iris.Map{
		"code": config.StatusOK,
		"msg":  "",
		"data": iris.Map{
			"header":  header,
			"content": content,
		},
	})
}

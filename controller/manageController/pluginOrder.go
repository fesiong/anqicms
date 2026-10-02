package manageController

import (
	"fmt"
	"time"

	"github.com/kataras/iris/v12"
	"gorm.io/gorm"
	"kandaoni.com/anqicms/config"
	"kandaoni.com/anqicms/model"
	"kandaoni.com/anqicms/provider"
	"kandaoni.com/anqicms/request"
)

// PluginOrderList 获取订单列表，支持订单号、用户名、类型、状态过滤和分页。
//
// 参数说明：
//   - 查询参数 "current": 当前页码，默认为 1。
//   - 查询参数 "pageSize": 每页数量，默认为 20。
//   - 查询参数 "order_id": 订单号过滤。
//   - 查询参数 "user_name": 用户名模糊过滤。
//   - 查询参数 "status": 订单状态过滤：waiting,paid,delivery,finished,refunding,closed。
//   - 查询参数 "type": 订单类型过滤：archive|vip。
func PluginOrderList(ctx iris.Context) {
	currentSite := provider.CurrentSite(ctx)
	currentPage := ctx.URLParamIntDefault("current", 1)
	pageSize := ctx.URLParamIntDefault("pageSize", 20)
	orderId := ctx.URLParam("order_id")
	userName := ctx.URLParam("user_name")
	status := ctx.URLParam("status")
	orderType := ctx.URLParam("type")

	orders, total := currentSite.GetOrderList(func(tx *gorm.DB) *gorm.DB {
		if orderType != "" {
			tx = tx.Where("`type` = ?", orderType)
		}
		if orderId != "" {
			tx = tx.Where("`order_id` = ?", orderId)
		}
		if userName != "" {
			var userIds []uint
			currentSite.DB.Model(&model.User{}).Where("`user_name` LIKE ?", "%"+userName+"%").Pluck("id", &userIds)
			tx = tx.Where("`user_id` IN (?)", userIds)
		}
		return tx
	}, status, currentPage, pageSize)

	ctx.JSON(iris.Map{
		"code":  config.StatusOK,
		"msg":   "",
		"total": total,
		"data":  orders,
	})
}

// PluginOrderDetail 根据订单号获取订单详情，附带买家、分享用户和上级分享用户信息。
//
// 参数说明：
//   - 查询参数 "order_id": 订单号。
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

// PluginOrderSetPay 手动标记订单为已支付：必要时生成支付单，记录消费流水并处理支付成功逻辑。
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
		payment, err = currentSite.GeneratePayment(order, &req)
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
	err = currentSite.SuccessPaidOrder(order, payment)
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

// PluginOrderSetDeliver 设置订单发货。
func PluginOrderSetDeliver(ctx iris.Context) {
	currentSite := provider.CurrentSite(ctx)
	var req request.OrderDeliveryRequest
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

// PluginOrderSetFinished 将订单标记为已完成。
func PluginOrderSetFinished(ctx iris.Context) {
	currentSite := provider.CurrentSite(ctx)
	var req request.OrderFinishedRequest
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

// PluginOrderSetCanceled 将订单标记为已取消。
func PluginOrderSetCanceled(ctx iris.Context) {
	currentSite := provider.CurrentSite(ctx)
	var req request.OrderFinishedRequest
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

// PluginOrderSetRefund 处理订单退款。
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

// PluginOrderApplyRefund 为订单发起退款申请，并将退款状态置为申请中。
func PluginOrderApplyRefund(ctx iris.Context) {
	currentSite := provider.CurrentSite(ctx)
	var req request.OrderFinishedRequest
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

// PluginOrderConfig 获取当前站点的订单插件配置。
func PluginOrderConfig(ctx iris.Context) {
	currentSite := provider.CurrentSite(ctx)
	setting := currentSite.PluginOrder

	ctx.JSON(iris.Map{
		"code": config.StatusOK,
		"msg":  "",
		"data": setting,
	})
}

// PluginOrderConfigForm 保存订单插件的配置，并清理缓存索引。
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
	currentSite.PluginOrder.NoNeedLogin = req.NoNeedLogin

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

// PluginOrderExport 根据选中的时间范围和订单状态，导出订单。
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

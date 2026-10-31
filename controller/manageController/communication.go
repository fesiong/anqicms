package manageController

import (
	"github.com/kataras/iris/v12"
	"gorm.io/gorm"
	"kandaoni.com/anqicms/config"
	"kandaoni.com/anqicms/provider"
	"kandaoni.com/anqicms/request"
)

// GetCommunicationSetting 获取询盘/客服设置（无参数）。
func GetCommunicationSetting(ctx iris.Context) {
	currentSite := provider.CurrentSite(ctx)
	setting := currentSite.GetCommunicationSetting()

	ctx.JSON(iris.Map{
		"code": config.StatusOK,
		"msg":  "",
		"data": setting,
	})
}

// SaveCommunicationSetting 保存询盘/客服设置（整体覆盖）。
func SaveCommunicationSetting(ctx iris.Context) {
	currentSite := provider.CurrentSite(ctx)
	var req config.CommunicationConfig
	if !readJSONPayload(ctx, &req) {
		return
	}

	currentSite.SaveCommunicationSetting(&req)

	ctx.JSON(iris.Map{
		"code": config.StatusOK,
		"msg":  "",
	})
}

// CommunicationOrderList 分页查询询盘（沟通）订单列表。
//
// 参数说明：
//   - 查询参数 "current": 当前页码，默认为 1。
//   - 查询参数 "pageSize": 每页条数，默认为 20。
//   - 查询参数 "id": 按订单 ID 精确筛选，0 表示不限。
//   - 查询参数 "status": 按状态筛选，waiting=待处理(0)/doing=进行中(1)/quote=已报价(2)/accepted=已接受(3)/finished=已完成(4)/closed=已关闭(-1)；不传则不筛选。
func CommunicationOrderList(ctx iris.Context) {
	currentSite := provider.CurrentSite(ctx)
	currentPage := ctx.URLParamIntDefault("current", 1)
	pageSize := ctx.URLParamIntDefault("pageSize", 20)
	orderId := ctx.URLParamInt64Default("id", 0)
	status := ctx.URLParam("status")

	orders, total := currentSite.GetCommunicationOrderList(func(tx *gorm.DB) *gorm.DB {
		if orderId > 0 {
			tx = tx.Where("id = ?", orderId)
		}
		if status != "" {
			if status == "waiting" {
				tx = tx.Where("`status` = 0")
			}
			if status == "doing" {
				tx = tx.Where("`status` = 1")
			}
			if status == "quote" {
				tx = tx.Where("`status` = 2")
			}
			if status == "accepted" {
				tx = tx.Where("`status` = 3")
			}
			if status == "closed" {
				tx = tx.Where("`status` = -1")
			}
			if status == "finished" {
				tx = tx.Where("`status` = 4")
			}
		}

		return tx
	}, "id desc", currentPage, pageSize)

	var userIds []uint
	for _, v := range orders {
		userIds = append(userIds, uint(v.UserId))
	}
	users := currentSite.GetUsersInfoByIds(userIds)
	for i := range orders {
		for _, v := range users {
			if orders[i].UserId == int64(v.Id) {
				orders[i].User = v
				break
			}
		}
	}

	ctx.JSON(iris.Map{
		"code":  config.StatusOK,
		"msg":   "",
		"total": total,
		"data":  orders,
	})
}

// CommunicationOrderDetail 按 ID 查询询盘订单详情，同时回传当前的询盘设置。
//
// 参数说明：
//   - 查询参数 "id": 询盘订单 ID，必填；为 0 时返回 Invalid Params。
func CommunicationOrderDetail(ctx iris.Context) {
	currentSite := provider.CurrentSite(ctx)
	id := ctx.URLParamInt64Default("id", 0)
	if id == 0 {
		ctx.JSON(iris.Map{
			"code": config.StatusFailed,
			"msg":  "Invalid Params",
		})
		return
	}

	order, err := currentSite.GetCommunicationOrderById(id)
	if err != nil {
		ctx.JSON(iris.Map{
			"code": config.StatusFailed,
			"msg":  err.Error(),
		})
		return
	}

	setting := currentSite.GetCommunicationSetting()
	order.OrderFields = setting.OrderFields
	order.Thumb = order.GetThumb(currentSite.PluginStorage.StorageUrl, currentSite.Content.DefaultThumb)

	ctx.JSON(iris.Map{
		"code": config.StatusOK,
		"msg":  "",
		"data": order,
	})
}

// CommunicationOrderSaveImages 更新询盘订单的图片列表（整体替换，不是追加）。
func CommunicationOrderSaveImages(ctx iris.Context) {
	// 仅能更新图片
	var req request.CommunicationOrderImagesRequest
	if err := ctx.ReadJSON(&req); err != nil {
		ctx.JSON(iris.Map{
			"code": config.StatusFailed,
			"msg":  "Invalid Params",
		})
		return
	}
	currentSite := provider.CurrentSite(ctx)

	err := currentSite.SaveCommunicationOrderImages(&req)
	if err != nil {
		ctx.JSON(iris.Map{
			"code": config.StatusFailed,
			"msg":  err.Error(),
		})
		return
	}

	ctx.JSON(iris.Map{
		"code": config.StatusOK,
		"msg":  "success",
	})
}

// 发送报价
func CommunicationSendQuote(ctx iris.Context) {
	var req request.CommunicationOrderRequest
	err := ctx.ReadJSON(&req)
	if err != nil {
		ctx.JSON(iris.Map{
			"code": config.StatusFailed,
			"msg":  "Invalid Params",
		})
		return
	}
	if req.QuotePrice <= 0 {
		ctx.JSON(iris.Map{
			"code": config.StatusFailed,
			"msg":  "Invalid Params",
		})
		return
	}

	currentSite := provider.CurrentSite(ctx)

	adminId := ctx.Values().GetUintDefault("adminId", 0)
	req.SenderId = int64(adminId)
	admin, err := currentSite.GetAdminInfoById(adminId)
	if err == nil {
		req.SenderName = admin.UserName
	}

	err = currentSite.CommunicationSendQuote(&req)
	if err != nil {
		ctx.JSON(iris.Map{
			"code": config.StatusFailed,
			"msg":  err.Error(),
		})
		return
	}

	ctx.JSON(iris.Map{
		"code": config.StatusOK,
		"msg":  "success",
	})
}

// CommunicationMessageSend 管理员回复询盘消息；sender_id/sender_name 由服务端按当前登录管理员覆盖，请求里传了也会被替换。
func CommunicationMessageSend(ctx iris.Context) {
	var req request.CommunicationMessageRequest
	err := ctx.ReadJSON(&req)
	if err != nil {
		ctx.JSON(iris.Map{
			"code": config.StatusFailed,
			"msg":  "Invalid Params",
		})
		return
	}
	currentSite := provider.CurrentSite(ctx)
	adminId := ctx.Values().GetUintDefault("adminId", 0)
	req.SenderId = int64(adminId)
	admin, err := currentSite.GetAdminInfoById(adminId)
	if err == nil {
		req.SenderName = admin.UserName
	}
	req.SenderType = "admin"
	if req.MessageType == "" {
		req.MessageType = "text"
	}

	order, err := currentSite.GetCommunicationOrderById(req.CommOrderId)
	if err != nil {
		ctx.JSON(iris.Map{
			"code": config.StatusFailed,
			"msg":  err.Error(),
		})
		return
	}
	if order.Status == config.CommunicationStatusWaiting {
		order.Status = config.CommunicationStatusDoing
		currentSite.DB.Save(order)
	}

	err = currentSite.CommunicationMessageSend(&req)
	if err != nil {
		ctx.JSON(iris.Map{
			"code": config.StatusFailed,
			"msg":  err.Error(),
		})
		return
	}

	ctx.JSON(iris.Map{
		"code": config.StatusOK,
		"msg":  "success",
	})
}

// CommunicationMessageDelete 删除单条询盘消息。
func CommunicationMessageDelete(ctx iris.Context) {
	var req request.CommunicationMessageRequest
	if !readJSONPayload(ctx, &req) {
		return
	}
	// 删除必须指定消息 id，缺省时不能回"已删除"。
	if !requirePositive(ctx, req.Id, "id") {
		return
	}
	currentSite := provider.CurrentSite(ctx)

	err := currentSite.DeleteCommunicationMessage(req.Id)
	if err != nil {
		ctx.JSON(iris.Map{
			"code": config.StatusFailed,
			"msg":  err.Error(),
		})
		// 少了 return 会紧跟着再写一次成功回执，响应体变成两段 JSON 拼接。
		return
	}

	ctx.JSON(iris.Map{
		"code": config.StatusOK,
		"msg":  "deleted",
	})
}

// CommunicationMessageList 分页查询某个询盘订单下的往来消息。
//
// 参数说明：
//   - 查询参数 "comm_order_id": 询盘订单 ID，必填；为 0 时返回 Invalid Params。
//   - 查询参数 "current": 当前页码，默认为 1。
//   - 查询参数 "pageSize": 每页条数，默认为 20。
//   - 查询参数 "q": 消息内容关键词模糊搜索。
func CommunicationMessageList(ctx iris.Context) {
	commOrderId := ctx.URLParamInt64Default("comm_order_id", 0)
	currentPage := ctx.URLParamIntDefault("current", 1)
	pageSize := ctx.URLParamIntDefault("pageSize", 20)
	q := ctx.URLParam("q")
	if commOrderId == 0 {
		ctx.JSON(iris.Map{
			"code": config.StatusFailed,
			"msg":  "Invalid Params",
		})
		return
	}

	currentSite := provider.CurrentSite(ctx)

	messages, total := currentSite.GetCommunicationMessages(commOrderId, q, currentPage, pageSize)

	ctx.JSON(iris.Map{
		"code":  config.StatusOK,
		"msg":   "",
		"data":  messages,
		"total": total,
	})
}

// CommunicationOrderClose 关闭询盘订单。
func CommunicationOrderClose(ctx iris.Context) {
	var req request.CommunicationOrderRequest
	if !readJSONPayload(ctx, &req) {
		return
	}
	// 关闭订单必须指定订单 id，缺省时不能回"已关闭"。
	if !requirePositive(ctx, req.Id, "id") {
		return
	}

	currentSite := provider.CurrentSite(ctx)
	err := currentSite.CloseCommunicationOrder(req.Id)
	if err != nil {
		ctx.JSON(iris.Map{
			"code": config.StatusFailed,
			"msg":  err.Error(),
		})
		// 少了 return，错误回执之后又跟着写了一次成功的 JSON，
		// 客户端读到的是两段拼在一起的响应体（既不是合法 JSON，也藏掉了真实错误）。
		return
	}

	ctx.JSON(iris.Map{
		"code": config.StatusOK,
		"msg":  "closed",
	})
}

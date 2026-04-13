package manageController

import (
	"github.com/kataras/iris/v12"
	"gorm.io/gorm"
	"kandaoni.com/anqicms/config"
	"kandaoni.com/anqicms/provider"
	"kandaoni.com/anqicms/request"
)

func GetCommunicationSetting(ctx iris.Context) {
	currentSite := provider.CurrentSite(ctx)
	setting := currentSite.GetCommunicationSetting()

	ctx.JSON(iris.Map{
		"code": config.StatusOK,
		"msg":  "",
		"data": setting,
	})
}

func SaveCommunicationSetting(ctx iris.Context) {
	currentSite := provider.CurrentSite(ctx)
	var req config.CommunicationConfig
	if err := ctx.ReadJSON(&req); err != nil {
		ctx.JSON(iris.Map{
			"code": config.StatusFailed,
			"msg":  err.Error(),
		})
		return
	}

	currentSite.SaveCommunicationSetting(&req)

	ctx.JSON(iris.Map{
		"code": config.StatusOK,
		"msg":  "",
	})
}

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

func CommunicationOrderSaveImages(ctx iris.Context) {
	// 仅能更新图片
	var req request.CommunicationOrderImagesRequest
	if err := ctx.ReadJSON(&req); err != nil {
		ctx.JSON(iris.Map{
			"code": config.StatusFailed,
			"msg":  "Invalid Params",
		})
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

func CommunicationMessageDelete(ctx iris.Context) {
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

	err = currentSite.DeleteCommunicationMessage(req.Id)
	if err != nil {
		ctx.JSON(iris.Map{
			"code": config.StatusFailed,
			"msg":  err.Error(),
		})
	}

	ctx.JSON(iris.Map{
		"code": config.StatusOK,
		"msg":  "deleted",
	})
}

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

func CommunicationOrderClose(ctx iris.Context) {
	var req request.CommunicationOrderRequest
	if err := ctx.ReadJSON(&req); err != nil {
		ctx.JSON(iris.Map{
			"code": config.StatusFailed,
			"msg":  err.Error(),
		})
		return
	}

	currentSite := provider.CurrentSite(ctx)
	err := currentSite.CloseCommunicationOrder(req.Id)
	if err != nil {
		ctx.JSON(iris.Map{
			"code": config.StatusFailed,
			"msg":  err.Error(),
		})
	}

	ctx.JSON(iris.Map{
		"code": config.StatusOK,
		"msg":  "closed",
	})
}

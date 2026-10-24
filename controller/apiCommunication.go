package controller

import (
	"github.com/kataras/iris/v12"
	"gorm.io/gorm"
	"kandaoni.com/anqicms/config"
	"kandaoni.com/anqicms/provider"
	"kandaoni.com/anqicms/request"
)

func ApiGetCommunicationOrderList(ctx iris.Context) {
	currentSite := provider.CurrentSite(ctx)
	currentPage := ctx.URLParamIntDefault("current", 1)
	pageSize := ctx.URLParamIntDefault("pageSize", 20)
	userId := ctx.Values().GetUintDefault("userId", 0)
	status := ctx.URLParam("status")

	list, total := currentSite.GetCommunicationOrderList(func(tx *gorm.DB) *gorm.DB {
		tx = tx.Where("user_id = ?", userId)
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
	// 读取最后一条评论
	for _, item := range list {
		item.Messages, _ = currentSite.GetCommunicationMessages(item.Id, "", 1, 1)
	}
	ctx.JSON(iris.Map{
		"code":  config.StatusOK,
		"data":  list,
		"total": total,
	})
}

func ApiGetCommunicationOrderFields(ctx iris.Context) {

	currentSite := provider.CurrentSite(ctx)

	setting := currentSite.GetCommunicationSetting()

	ctx.JSON(iris.Map{
		"code": config.StatusOK,
		"msg":  "",
		"data": setting.OrderFields,
	})
}

func ApiCreateCommunicationOrder(ctx iris.Context) {
	currentSite := provider.CurrentSite(ctx)
	var req request.CommunicationOrderRequest
	if err := ctx.ReadJSON(&req); err != nil {
		ctx.JSON(iris.Map{
			"code": config.StatusFailed,
			"msg":  err.Error(),
		})
		return
	}

	if req.Title == "" || len(req.Images) == 0 {
		ctx.JSON(iris.Map{
			"code": config.StatusFailed,
			"msg":  "Invalid Params",
		})
		return
	}

	userId := ctx.Values().GetUintDefault("userId", 0)
	req.UserId = int64(userId)

	order, err := currentSite.CommunicationOrderCreate(&req)
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
		"data": order,
	})
}

func ApiCommunicationOrderUpdate(ctx iris.Context) {
	currentSite := provider.CurrentSite(ctx)
	var req request.CommunicationOrderRequest
	err := ctx.ReadJSON(&req)
	if err != nil {
		ctx.JSON(iris.Map{
			"code": config.StatusFailed,
			"msg":  err.Error(),
		})
		return
	}
	// 可以更新orderextra， title, content
	userId := ctx.Values().GetUintDefault("userId", 0)
	order, err := currentSite.GetCommunicationOrderByUser(int64(userId), req.Id)
	if err != nil {
		ctx.JSON(iris.Map{
			"code": config.StatusFailed,
			"msg":  err.Error(),
		})
		return
	}
	if req.Content != "" {
		order.Content = req.Content
	}
	if req.Title != "" {
		order.Title = req.Title
	}
	if req.OrderExtra != nil {
		order.OrderExtra = req.OrderExtra
	}
	currentSite.DB.Save(order)

	ctx.JSON(iris.Map{
		"code": config.StatusOK,
		"msg":  "success",
	})
}

func ApiGetCommunicationOrderDetail(ctx iris.Context) {
	id := ctx.URLParamInt64Default("id", 0)
	userId := ctx.Values().GetUintDefault("userId", 0)
	currentSite := provider.CurrentSite(ctx)

	order, err := currentSite.GetCommunicationOrderByUser(int64(userId), id)
	if err != nil {
		ctx.JSON(iris.Map{
			"code": config.StatusFailed,
			"msg":  err.Error(),
		})
		return
	}

	if order == nil {
		// 获取第一个
		order, err = currentSite.GetLastCommunicationOrderByUser(int64(userId))
		if err != nil {
			ctx.JSON(iris.Map{
				"code": config.StatusFailed,
				"msg":  err.Error(),
			})
			return
		}
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

func ApiGetCommunicationMessages(ctx iris.Context) {
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

	userId := ctx.Values().GetUintDefault("userId", 0)

	currentSite := provider.CurrentSite(ctx)

	order, err := currentSite.GetCommunicationOrderByUser(int64(userId), commOrderId)
	if err != nil {
		ctx.JSON(iris.Map{
			"code": config.StatusFailed,
			"msg":  err.Error(),
		})
		return
	}

	messages, total := currentSite.GetCommunicationMessages(order.Id, q, currentPage, pageSize)
	if err != nil {
		ctx.JSON(iris.Map{
			"code": config.StatusFailed,
			"msg":  err.Error(),
		})
		return
	}

	ctx.JSON(iris.Map{
		"code":  config.StatusOK,
		"msg":   "",
		"data":  messages,
		"total": total,
	})
}

func ApiSendCommunicationMessage(ctx iris.Context) {
	var req request.CommunicationMessageRequest
	err := ctx.ReadJSON(&req)
	if err != nil {
		ctx.JSON(iris.Map{
			"code": config.StatusFailed,
			"msg":  "Invalid Params",
		})
		return
	}
	userId := ctx.Values().GetUintDefault("userId", 0)

	currentSite := provider.CurrentSite(ctx)

	req.SenderId = int64(userId)
	if req.SenderName == "" {
		user, _ := currentSite.GetUserInfoById(uint(req.SenderId))
		if user != nil {
			if user.RealName != "" {
				req.SenderName = user.RealName
			} else {
				req.SenderName = user.UserName
			}
		}
	}
	req.SenderType = "user"

	_, err = currentSite.GetCommunicationOrderByUser(int64(userId), req.CommOrderId)
	if err != nil {
		ctx.JSON(iris.Map{
			"code": config.StatusFailed,
			"msg":  err.Error(),
		})
		return
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

func ApiCommunicationOrderAcceptQuote(ctx iris.Context) {
	var req request.OrderRequest
	err := ctx.ReadJSON(&req)
	if err != nil {
		ctx.JSON(iris.Map{
			"code": config.StatusFailed,
			"msg":  "Invalid Params",
		})
		return
	}
	userId := ctx.Values().GetUintDefault("userId", 0)
	currentSite := provider.CurrentSite(ctx)

	communicationOrder, err := currentSite.GetCommunicationOrderByUser(int64(userId), req.Details[0].GoodsId)
	if err != nil {
		ctx.JSON(iris.Map{
			"code": config.StatusFailed,
			"msg":  err.Error(),
		})
		return
	}
	req.Type = config.OrderTypeCommunication

	// 创建订单
	order, err := currentSite.CreateOrder(uint(communicationOrder.UserId), &req)
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
		"data": order,
	})
}

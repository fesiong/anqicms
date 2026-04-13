package manageController

import (
	"github.com/kataras/iris/v12"
	"kandaoni.com/anqicms/config"
	"kandaoni.com/anqicms/model"
	"kandaoni.com/anqicms/provider"
	"kandaoni.com/anqicms/request"
)

func PluginGetSubscribers(ctx iris.Context) {
	currentSite := provider.CurrentSubSite(ctx)
	currentPage := ctx.URLParamIntDefault("current", 1)
	pageSize := ctx.URLParamIntDefault("pageSize", 20)
	email := ctx.URLParam("email")
	categoryId := ctx.URLParamInt64Default("category_id", 0)

	subscribers, total := currentSite.GetSubscribers(categoryId, email, currentPage, pageSize)

	ctx.JSON(iris.Map{
		"code":  config.StatusOK,
		"msg":   "",
		"total": total,
		"data":  subscribers,
	})
}

func PluginSaveSubscriber(ctx iris.Context) {
	currentSite := provider.CurrentSubSite(ctx)
	var req request.SubscriberRequest
	if err := ctx.ReadJSON(&req); err != nil {
		ctx.JSON(iris.Map{
			"code": config.StatusFailed,
			"msg":  err.Error(),
		})
		return
	}
	if req.Email == "" {
		ctx.JSON(iris.Map{
			"code": config.StatusFailed,
			"msg":  ctx.Tr("PleaseFillInTheEmail"),
		})
		return
	}

	err := currentSite.SaveSubscriber(&req)
	if err != nil {
		ctx.JSON(iris.Map{
			"code": config.StatusFailed,
			"msg":  err.Error(),
		})
		return
	}

	currentSite.AddAdminLog(ctx, ctx.Tr("UpdateSubscriber%s", req.Email))

	ctx.JSON(iris.Map{
		"code": config.StatusOK,
		"msg":  ctx.Tr("OperationSuccessful"),
	})
}

func PluginDeleteSubscriber(ctx iris.Context) {
	currentSite := provider.CurrentSubSite(ctx)
	var req request.SubscriberRequest
	if err := ctx.ReadJSON(&req); err != nil {
		ctx.JSON(iris.Map{
			"code": config.StatusFailed,
			"msg":  err.Error(),
		})
		return
	}

	subscriber, err := currentSite.GetSubscriber(req.Id)
	if err != nil {
		ctx.JSON(iris.Map{
			"code": config.StatusFailed,
			"msg":  err.Error(),
		})
		return
	}
	// 后台，根据ID删除
	currentSite.DeleteSubscriber(subscriber)

	currentSite.AddAdminLog(ctx, ctx.Tr("DeleteSubscriber%s", req.Email))

	ctx.JSON(iris.Map{
		"code": config.StatusOK,
		"msg":  ctx.Tr("OperationSuccessful"),
	})
}

func SendSubscriberMail(ctx iris.Context) {
	currentSite := provider.CurrentSubSite(ctx)
	var req request.SubscriberMailRequest
	if err := ctx.ReadJSON(&req); err != nil {
		ctx.JSON(iris.Map{
			"code": config.StatusFailed,
			"msg":  err.Error(),
		})
		return
	}

	go currentSite.SendSubscriberMail(&req)

	currentSite.AddAdminLog(ctx, ctx.Tr("SendSubscriberMail%s"))

	ctx.JSON(iris.Map{
		"code": config.StatusOK,
		"msg":  ctx.Tr("OperationSuccessful"),
	})
}

func GetSubscriberCategories(ctx iris.Context) {
	currentSite := provider.CurrentSubSite(ctx)

	categories := currentSite.GetSubscriberCategories()

	ctx.JSON(iris.Map{
		"code":  config.StatusOK,
		"msg":   "",
		"data":  categories,
		"total": len(categories),
	})
}

func SaveSubscriberCategory(ctx iris.Context) {
	currentSite := provider.CurrentSubSite(ctx)
	var req request.SubscriberCategoryRequest
	if err := ctx.ReadJSON(&req); err != nil {
		ctx.JSON(iris.Map{
			"code": config.StatusFailed,
			"msg":  err.Error(),
		})
		return
	}
	if req.Title == "" {
		ctx.JSON(iris.Map{
			"code": config.StatusFailed,
			"msg":  ctx.Tr("PleaseFillInTheCategoryName"),
		})
		return
	}
	var category model.SubscriberCategory
	exist, err := currentSite.GetSubscriberCategoryByTitle(req.Title)
	if err == nil {
		// 存在
		if (req.Id > 0 && exist.Id != req.Id) || req.Id == 0 {
			ctx.JSON(iris.Map{
				"code": config.StatusFailed,
				"msg":  ctx.Tr("TheCategoryNameAlreadyExists"),
			})
			return
		}
	}
	category.Id = req.Id
	category.Title = req.Title

	currentSite.DB.Save(&category)

	currentSite.AddAdminLog(ctx, ctx.Tr("UpdateSubscriberCategory%d:%s", category.Id, category.Title))

	ctx.JSON(iris.Map{
		"code": config.StatusOK,
		"msg":  ctx.Tr("OperationSuccessful"),
	})
}

func DeleteSubscriberCategory(ctx iris.Context) {
	currentSite := provider.CurrentSubSite(ctx)
	var req request.SubscriberCategoryRequest
	if err := ctx.ReadJSON(&req); err != nil {
		ctx.JSON(iris.Map{
			"code": config.StatusFailed,
			"msg":  err.Error(),
		})
		return
	}

	currentSite.DeleteSubscriberCategory(req.Id)

	currentSite.AddAdminLog(ctx, ctx.Tr("DeleteSubscriberCategory%d:%s", req.Id, req.Title))

	ctx.JSON(iris.Map{
		"code": config.StatusOK,
		"msg":  ctx.Tr("DeleteSuccessful"),
	})
}

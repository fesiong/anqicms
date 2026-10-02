package manageController

import (
	"github.com/kataras/iris/v12"
	"kandaoni.com/anqicms/config"
	"kandaoni.com/anqicms/model"
	"kandaoni.com/anqicms/provider"
	"kandaoni.com/anqicms/request"
)

// PluginGetSubscribers 分页获取订阅用户列表。
//
// 参数说明：
//   - 查询参数 "current": 当前页码，默认为 1。
//   - 查询参数 "pageSize": 每页数量，默认为 20。
//   - 查询参数 "email": 按邮箱筛选订阅用户。
//   - 查询参数 "category_id": 按订阅分类 ID 筛选，默认为 0（全部分类）。
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

// PluginGetSubscriber 根据ID获取订阅用户详情。
//
// 参数说明：
//   - 查询参数 "id": 订阅用户ID，必填。
func PluginGetSubscriber(ctx iris.Context) {
	currentSite := provider.CurrentSubSite(ctx)
	id := ctx.URLParamInt64Default("id", 0)
	subscriber, err := currentSite.GetSubscriber(id)
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
		"data": subscriber,
	})
}

// PluginSaveSubscriber 保存（新增或更新）订阅用户。
//
// 参数说明：
//   - 请求体 "email": 订阅用户邮箱，必填。
//   - 请求体 "id": 订阅用户ID，大于 0 时为更新。
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

	subscriber, err := currentSite.SaveSubscriber(&req)
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
		"data": subscriber,
	})
}

// PluginDeleteSubscriber 根据ID删除订阅用户。
//
// 参数说明：
//   - 请求体 "id": 订阅用户ID。
func PluginDeleteSubscriber(ctx iris.Context) {
	currentSite := provider.CurrentSubSite(ctx)
	var req request.SubscriberDeleteRequest
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

	currentSite.AddAdminLog(ctx, ctx.Tr("DeleteSubscriber%s", subscriber.Email))

	ctx.JSON(iris.Map{
		"code": config.StatusOK,
		"msg":  ctx.Tr("OperationSuccessful"),
	})
}

// SendSubscriberMail 向订阅用户异步发送邮件。
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

// GetSubscriberCategories 获取全部订阅分类列表。
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

// SaveSubscriberCategory 保存（新增或更新）订阅分类。
//
// 参数说明：
//   - 请求体 "id": 订阅分类ID，大于 0 时为更新。
//   - 请求体 "title": 分类名称，必填且不能与其他分类重名。
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
		"data": category,
	})
}

// DeleteSubscriberCategory 根据 ID 删除指定订阅分类
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

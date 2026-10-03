package manageController

import (
	"time"

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

// SendSubscriberMail 向订阅用户异步发送邮件，返回 job_id 供查询进度。
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

	// 先校验参数再异步：原先不管参数对不对都 go 出去然后报「操作成功」，
	// 参数写错时 goroutine 静默什么都不做，调用方无从察觉（2026-10-03 修）。
	if req.Type != "all" && req.Type != "category" && req.Type != "email" {
		ctx.JSON(iris.Map{
			"code": config.StatusFailed,
			"msg":  "type 必须是 all / category / email 之一",
		})
		return
	}
	if req.Type == "email" && len(req.Emails) == 0 {
		ctx.JSON(iris.Map{
			"code": config.StatusFailed,
			"msg":  "type=email 时必须提供非空的 emails 列表",
		})
		return
	}

	// 群发是长任务（SendMail 是同步 SMTP 调用、单封数秒），必须异步，
	// 否则收件人一多 API 必然超时。立即返回 job_id，调用方用 send_status 轮询。
	jobIDCh := make(chan string, 1)
	go func() { jobIDCh <- currentSite.SendSubscriberMail(&req) }()

	// 函数一进来就建 job，通常毫秒级即可拿到 job_id；3 秒是兜底。
	var jobID string
	select {
	case jobID = <-jobIDCh:
	case <-time.After(3 * time.Second):
	}
	if jobID == "" {
		ctx.JSON(iris.Map{
			"code": config.StatusFailed,
			"msg":  "群发任务创建失败：缺少 recommendation 邮件模板，请先在邮件模板中补齐",
		})
		return
	}

	currentSite.AddAdminLog(ctx, ctx.Tr("SendSubscriberMail%s"))

	ctx.JSON(iris.Map{
		"code": config.StatusOK,
		"msg":  ctx.Tr("OperationSuccessful"),
		"data": map[string]any{
			"job_id": jobID,
			"hint":   "群发已受理，异步执行中。用 send_status 传 job_id 轮询进度（total/sent/failed/processed/running）。",
		},
	})
}

// GetSubscriberSendStatus 查询群发进度。
//
// 群发是异步长任务、send 立即返回，调用方除了本端点没有别的手段判断成败。
func GetSubscriberSendStatus(ctx iris.Context) {
	jobID := ctx.URLParam("job_id")
	if jobID == "" {
		// iris 的 FormValue 对 GET 请求读的是 query string
		jobID = ctx.FormValue("job_id")
	}
	if jobID == "" {
		ctx.JSON(iris.Map{
			"code": config.StatusFailed,
			"msg":  "缺少 job_id 参数",
		})
		return
	}
	r, ok := provider.GetSendMailResult(jobID)
	if !ok {
		ctx.JSON(iris.Map{
			"code": config.StatusFailed,
			"msg":  "未找到该 job_id（可能已过期：仅保留最近 20 次群发，或服务已重启）",
		})
		return
	}
	ctx.JSON(iris.Map{
		"code": config.StatusOK,
		"msg":  "",
		"data": r,
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

package manageController

import (
	"github.com/kataras/iris/v12"
	"kandaoni.com/anqicms/config"
	"kandaoni.com/anqicms/model"
	"kandaoni.com/anqicms/provider"
	"kandaoni.com/anqicms/request"
)

// HandleCollectSetting 获取采集配置。
func HandleCollectSetting(ctx iris.Context) {
	currentSite := provider.CurrentSite(ctx)
	collector := currentSite.GetUserCollectorSetting()

	ctx.JSON(iris.Map{
		"code": config.StatusOK,
		"msg":  "",
		"data": collector,
	})
}

// HandleSaveCollectSetting 保存采集配置
func HandleSaveCollectSetting(ctx iris.Context) {
	currentSite := provider.CurrentSite(ctx)
	var req config.CollectorJson
	if err := ctx.ReadJSON(&req); err != nil {
		ctx.JSON(iris.Map{
			"code": config.StatusFailed,
			"msg":  err.Error(),
		})
		return
	}

	//将现有配置写回文件
	w2 := provider.GetWebsite(currentSite.Id)
	err := w2.SaveUserCollectorSetting(req, true)
	if err != nil {
		ctx.JSON(iris.Map{
			"code": config.StatusFailed,
			"msg":  err.Error(),
		})
		return
	}

	currentSite.AddAdminLog(ctx, ctx.Tr("ModifyAcquisitionConfiguration"))

	ctx.JSON(iris.Map{
		"code": config.StatusOK,
		"msg":  ctx.Tr("SaveSuccessfully"),
	})
}

// HandleReplaceArticles 保存内容替换关键词配置；当 replace 为 true 时触发批量替换文章内容的异步任务。
func HandleReplaceArticles(ctx iris.Context) {
	currentSite := provider.CurrentSite(ctx)
	var req request.ArchiveReplaceRequest
	if err := ctx.ReadJSON(&req); err != nil {
		ctx.JSON(iris.Map{
			"code": config.StatusFailed,
			"msg":  err.Error(),
		})
		return
	}

	if len(req.ContentReplace) == 0 {
		ctx.JSON(iris.Map{
			"code": config.StatusFailed,
			"msg":  ctx.Tr("ReplaceKeywordIsEmpty"),
		})
		return
	}
	// 先尝试保存
	collectorJson := config.CollectorJson{
		ContentReplace: req.ContentReplace,
	}
	w2 := provider.GetWebsite(currentSite.Id)
	err := currentSite.SaveUserCollectorSetting(collectorJson, false)
	if err != nil {
		ctx.JSON(iris.Map{
			"code": config.StatusFailed,
			"msg":  err.Error(),
		})
		return
	}

	if req.Replace {

		currentSite.AddAdminLog(ctx, ctx.Tr("BatchReplaceDocumentContent"))

		go w2.ReplaceArticles()
		ctx.JSON(iris.Map{
			"code": config.StatusOK,
			"msg":  ctx.Tr("ReplacementTaskHasBeenTriggered"),
		})
		return
	}

	currentSite.AddAdminLog(ctx, ctx.Tr("UpdateReplacementKeywordConfiguration"))

	ctx.JSON(iris.Map{
		"code": config.StatusOK,
		"msg":  ctx.Tr("KeywordsHaveBeenSaved"),
	})
}

// HandleDigKeywords 立即执行关键词挖掘任务。后台异步执行，接口立即返回。
func HandleDigKeywords(ctx iris.Context) {
	currentSite := provider.CurrentSite(ctx)
	go currentSite.StartDigKeywords(true)

	currentSite.AddAdminLog(ctx, ctx.Tr("ManuallyTriggerTheKeywordExpansionTask"))

	ctx.JSON(iris.Map{
		"code": config.StatusOK,
		"msg":  ctx.Tr("KeywordExpansionTaskHasBeenTriggered"),
	})
}

// HandleArticleCollect 根据指定关键词立即执行采集操作，不受时间限制。后台异步执行，接口立即返回。
func HandleArticleCollect(ctx iris.Context) {
	currentSite := provider.CurrentSite(ctx)
	var req request.KeywordRequest
	if err := ctx.ReadJSON(&req); err != nil {
		ctx.JSON(iris.Map{
			"code": config.StatusFailed,
			"msg":  err.Error(),
		})
		return
	}
	if req.Title != "" {
		// 保存关键词
		keyword, err := currentSite.GetKeywordByTitle(req.Title)
		if err != nil {
			// 不存在，则创建
			keyword = &model.Keyword{
				Title:  req.Title,
				Status: 1,
			}
			currentSite.SaveKeyword(keyword)
		}
		req.Id = keyword.Id
	}
	keyword, err := currentSite.GetKeywordById(req.Id)
	if err != nil {
		ctx.JSON(iris.Map{
			"code": config.StatusFailed,
			"msg":  err.Error(),
		})
		return
	}

	go currentSite.CollectArticlesByKeyword(*keyword, true)

	ctx.JSON(iris.Map{
		"code": config.StatusOK,
		"msg":  ctx.Tr("CollectionTaskHasBeenTriggered"),
	})
}

// HandleStartArticleCollect 立即执行采集任务，后台异步执行，接口立即返回。
func HandleStartArticleCollect(ctx iris.Context) {
	currentSite := provider.CurrentSite(ctx)
	go currentSite.CollectArticles()

	ctx.JSON(iris.Map{
		"code": config.StatusOK,
		"msg":  ctx.Tr("CollectionTaskHasBeenTriggered"),
	})
}

// HandleArticleCombinationGet 根据指定关键词立即执行获取问答组合文章
func HandleArticleCombinationGet(ctx iris.Context) {
	currentSite := provider.CurrentSite(ctx)
	var req request.KeywordRequest
	if err := ctx.ReadJSON(&req); err != nil {
		ctx.JSON(iris.Map{
			"code": config.StatusFailed,
			"msg":  err.Error(),
		})
		return
	}

	keyword := model.Keyword{Title: req.Title}
	archive, err := currentSite.GetCombinationArticle(&keyword)
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
		"data": archive,
	})
}

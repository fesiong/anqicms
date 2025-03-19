package manageController

import (
	"github.com/kataras/iris/v12"
	"kandaoni.com/anqicms/config"
	"kandaoni.com/anqicms/model"
	"kandaoni.com/anqicms/provider"
	"kandaoni.com/anqicms/request"
)

func HandleAiGenerateSetting(ctx iris.Context) {
	currentSite := provider.CurrentSite(ctx)
	setting := currentSite.AiGenerateConfig

	ctx.JSON(iris.Map{
		"code": config.StatusOK,
		"msg":  "",
		"data": setting,
	})
}

func HandleAiGenerateSettingSave(ctx iris.Context) {
	currentSite := provider.CurrentSite(ctx)
	var req config.AiGenerateConfig
	if err := ctx.ReadJSON(&req); err != nil {
		ctx.JSON(iris.Map{
			"code": config.StatusFailed,
			"msg":  err.Error(),
		})
		return
	}

	//将现有配置写回文件
	w2 := provider.GetWebsite(currentSite.Id)
	err := w2.SaveAiGenerateSetting(req, true)
	if err != nil {
		ctx.JSON(iris.Map{
			"code": config.StatusFailed,
			"msg":  err.Error(),
		})
		return
	}

	currentSite.AddAdminLog(ctx, ctx.Tr("ModifyAiAutomaticWritingConfiguration"))

	ctx.JSON(iris.Map{
		"code": config.StatusOK,
		"msg":  ctx.Tr("SaveSuccessfully"),
	})
}

// HandleArticleAiGenerate 手动生成不受时间限制，并且需要指定关键词
func HandleArticleAiGenerate(ctx iris.Context) {
	currentSite := provider.CurrentSite(ctx)
	var req request.KeywordRequest
	if err := ctx.ReadJSON(&req); err != nil {
		ctx.JSON(iris.Map{
			"code": config.StatusFailed,
			"msg":  err.Error(),
		})
		return
	}

	keyword, err := currentSite.GetKeywordById(req.Id)
	if err != nil {
		ctx.JSON(iris.Map{
			"code": config.StatusFailed,
			"msg":  err.Error(),
		})
		return
	}

	go currentSite.AiGenerateArticlesByKeyword(*keyword, true)

	ctx.JSON(iris.Map{
		"code": config.StatusOK,
		"msg":  ctx.Tr("AiGenerationTaskHasBeenTriggered"),
	})
}

func HandleStartArticleAiGenerate(ctx iris.Context) {
	currentSite := provider.CurrentSite(ctx)
	go currentSite.AiGenerateArticles()

	ctx.JSON(iris.Map{
		"code": config.StatusOK,
		"msg":  ctx.Tr("AiGenerationTaskHasBeenTriggered"),
	})
}

func HandleAiGenerateCheckApi(ctx iris.Context) {
	currentSite := provider.CurrentSite(ctx)
	result := currentSite.CheckOpenAIAPIValid()
	if result {
		ctx.JSON(iris.Map{
			"code": config.StatusOK,
			"msg":  ctx.Tr("TheServerCanAccessTheOpenaiInterface"),
		})
	} else {
		ctx.JSON(iris.Map{
			"code": config.StatusFailed,
			"msg":  ctx.Tr("TheServerCannotAccessTheOpenaiInterface"),
		})
	}
}

func HandleAiGenerateGetPlans(ctx iris.Context) {
	currentSite := provider.CurrentSite(ctx)
	currentPage := ctx.URLParamIntDefault("current", 1)
	pageSize := ctx.URLParamIntDefault("pageSize", 20)
	aiType := uint(ctx.URLParamIntDefault("type", 0))
	status := ctx.URLParamIntDefault("status", 0)
	keyword := ctx.URLParam("keyword")

	var total int64
	var plans []*model.AiArticlePlan
	tx := currentSite.DB.Model(&model.AiArticlePlan{})
	if aiType > 0 {
		tx = tx.Where("`type` = ?", aiType)
	}
	if status != 0 {
		tx = tx.Where("`status` = ?", status)
	}
	if len(keyword) > 0 {
		tx = tx.Where("`keyword` like ?", keyword+"%")
	}
	offset := 0
	if currentPage > 0 {
		offset = (currentPage - 1) * pageSize
	}
	tx.Count(&total).Order("id desc").Limit(pageSize).Offset(offset).Find(&plans)
	for i := range plans {
		// 获取文章
		if plans[i].ArticleId > 0 {
			archive, err := currentSite.GetArchiveById(plans[i].ArticleId)
			if err == nil {
				plans[i].Title = archive.Title
			} else {
				// 来自草稿
				archiveDraft, err := currentSite.GetArchiveDraftById(plans[i].ArticleId)
				if err == nil {
					plans[i].Title = archiveDraft.Title
				}
			}
		}
	}

	ctx.JSON(iris.Map{
		"code":  config.StatusOK,
		"msg":   "",
		"total": total,
		"data":  plans,
	})
}

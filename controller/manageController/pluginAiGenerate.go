package manageController

import (
	"github.com/kataras/iris/v12"
	"kandaoni.com/anqicms/config"
	"kandaoni.com/anqicms/model"
	"kandaoni.com/anqicms/provider"
	"kandaoni.com/anqicms/request"
)

// HandleAiGenerateSetting 获取AI自动写作的配置信息。
func HandleAiGenerateSetting(ctx iris.Context) {
	currentSite := provider.CurrentSite(ctx)
	setting := currentSite.AiGenerateConfig

	ctx.JSON(iris.Map{
		"code": config.StatusOK,
		"msg":  "",
		"data": setting,
	})
}

// HandleAiGenerateSettingSave 保存AI自动写作的配置信息。
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

// HandleArticleAiGenerate 根据指定关键词手动触发AI文章生成任务。
//
// 参数说明：
//   - 请求体 "id": 关键词ID，用于查找待生成文章的关键词。
//   - 请求体 "title": 关键词名称，也可以直接指派关键词。
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

	go currentSite.AiGenerateArticlesByKeyword(*keyword, true)

	ctx.JSON(iris.Map{
		"code": config.StatusOK,
		"msg":  ctx.Tr("AiGenerationTaskHasBeenTriggered"),
	})
}

// HandleStartArticleAiGenerate 启动AI文章自动生成任务，异步批量生成文章。
func HandleStartArticleAiGenerate(ctx iris.Context) {
	currentSite := provider.CurrentSite(ctx)
	go currentSite.AiGenerateArticles()

	ctx.JSON(iris.Map{
		"code": config.StatusOK,
		"msg":  ctx.Tr("AiGenerationTaskHasBeenTriggered"),
	})
}

// HandleAiGenerateCheckApi 检查服务器能否正常访问OpenAI接口。
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

// HandleAiGenerateGetPlans 获取AI文章自动生成计划列表。
//
// 参数说明：
//   - 获取参数 "current": 当前页码，默认为1。
//   - 获取参数 "pageSize": 每页显示的记录数，默认为20。
//   - 获取参数 "type": AI文章生成计划类型:1=AI写作，2=AI翻译，3=AI改写
//   - 获取参数 "status": AI文章生成计划状态:1=已推送进行中，2=已完成，4=写作出错
//   - 获取参数 "keyword": 按计划关键词前缀匹配过滤（like 'keyword%'，前缀匹配而非模糊匹配）
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

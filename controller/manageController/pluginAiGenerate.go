package manageController

import (
	"fmt"
	"github.com/kataras/iris/v12"
	"kandaoni.com/anqicms/config"
	"kandaoni.com/anqicms/provider"
	"kandaoni.com/anqicms/request"
)

func HandleAiGenerateSetting(ctx iris.Context) {
	currentSite := provider.CurrentSite(ctx)
	setting := currentSite.GetAiGenerateSetting()

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
	err := currentSite.SaveAiGenerateSetting(req, true)
	if err != nil {
		ctx.JSON(iris.Map{
			"code": config.StatusFailed,
			"msg":  err.Error(),
		})
		return
	}

	currentSite.AddAdminLog(ctx, fmt.Sprintf("修改AI自动写作配置"))

	ctx.JSON(iris.Map{
		"code": config.StatusOK,
		"msg":  "保存成功",
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
		"msg":  "AI生成任务已触发，预计1分钟后即可查看生成结果",
	})
}

func HandleStartArticleAiGenerate(ctx iris.Context) {
	currentSite := provider.CurrentSite(ctx)
	go currentSite.AiGenerateArticles()

	ctx.JSON(iris.Map{
		"code": config.StatusOK,
		"msg":  "AI生成任务已触发，预计1分钟后即可查看生成结果",
	})
}

func HandleAiGenerateCheckApi(ctx iris.Context) {
	currentSite := provider.CurrentSite(ctx)
	result := currentSite.CheckOpenAIAPIValid()
	if result {
		ctx.JSON(iris.Map{
			"code": config.StatusFailed,
			"msg":  "该服务器可以正常访问 OpenAI 接口地址",
		})
	} else {
		ctx.JSON(iris.Map{
			"code": config.StatusFailed,
			"msg":  "该服务器无法正常访问 OpenAI 接口地址",
		})
	}
}

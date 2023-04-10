package manageController

import (
	"fmt"
	"github.com/kataras/iris/v12"
	"kandaoni.com/anqicms/config"
	"kandaoni.com/anqicms/model"
	"kandaoni.com/anqicms/provider"
	"kandaoni.com/anqicms/request"
)

// HandleCollectSetting 全局配置
func HandleCollectSetting(ctx iris.Context) {
	currentSite := provider.CurrentSite(ctx)
	collector := currentSite.GetUserCollectorSetting()

	ctx.JSON(iris.Map{
		"code": config.StatusOK,
		"msg":  "",
		"data": collector,
	})
}

// HandleSaveCollectSetting 全局配置保存
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
	err := currentSite.SaveUserCollectorSetting(req, true)
	if err != nil {
		ctx.JSON(iris.Map{
			"code": config.StatusFailed,
			"msg":  err.Error(),
		})
		return
	}

	currentSite.AddAdminLog(ctx, fmt.Sprintf("修改采集配置"))

	ctx.JSON(iris.Map{
		"code": config.StatusOK,
		"msg":  "保存成功",
	})
}

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
			"msg":  "替换关键词为空",
		})
		return
	}
	// 先尝试保存
	collectorJson := config.CollectorJson{
		ContentReplace: req.ContentReplace,
	}
	err := currentSite.SaveUserCollectorSetting(collectorJson, false)
	if err != nil {
		ctx.JSON(iris.Map{
			"code": config.StatusFailed,
			"msg":  err.Error(),
		})
		return
	}

	if req.Replace {

		currentSite.AddAdminLog(ctx, fmt.Sprintf("批量替换文档内容"))

		go currentSite.ReplaceArticles()
		ctx.JSON(iris.Map{
			"code": config.StatusOK,
			"msg":  "替换任务已触发",
		})
		return
	}

	currentSite.AddAdminLog(ctx, fmt.Sprintf("更新替换关键词配置"))

	ctx.JSON(iris.Map{
		"code": config.StatusOK,
		"msg":  "关键词已保存",
	})
}

func HandleDigKeywords(ctx iris.Context) {
	currentSite := provider.CurrentSite(ctx)
	go currentSite.StartDigKeywords(true)

	currentSite.AddAdminLog(ctx, fmt.Sprintf("手动触发关键词拓词任务"))

	ctx.JSON(iris.Map{
		"code": config.StatusOK,
		"msg":  "关键词拓词任务已触发",
	})
}

// HandleArticleCollect 手动采集不受时间限制，并且需要指定关键词
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
		"msg":  "采集任务已触发，预计1分钟后即可查看采集结果",
	})
}

// HandleArticleCombinationGet 获取问答组合文章
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

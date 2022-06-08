package manageController

import (
	"github.com/kataras/iris/v12"
	"kandaoni.com/anqicms/config"
	"kandaoni.com/anqicms/provider"
	"kandaoni.com/anqicms/request"
)

// HandleCollectSetting 全局配置
func HandleCollectSetting(ctx iris.Context) {
	collector := provider.GetUserCollectorSetting()

	ctx.JSON(iris.Map{
		"code": config.StatusOK,
		"msg":  "",
		"data": collector,
	})
}

// HandleSaveCollectSetting 全局配置保存
func HandleSaveCollectSetting(ctx iris.Context) {
	var req config.CollectorJson
	if err := ctx.ReadJSON(&req); err != nil {
		ctx.JSON(iris.Map{
			"code": config.StatusFailed,
			"msg":  err.Error(),
		})
		return
	}

	//将现有配置写回文件
	err := provider.SaveUserCollectorSetting(req, true)
	if err != nil {
		ctx.JSON(iris.Map{
			"code": config.StatusFailed,
			"msg":  err.Error(),
		})
		return
	}

	ctx.JSON(iris.Map{
		"code": config.StatusOK,
		"msg":  "保存成功",
	})
}

func HandleReplaceArticles(ctx iris.Context) {
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
	err := provider.SaveUserCollectorSetting(collectorJson, false)
	if err != nil {
		ctx.JSON(iris.Map{
			"code": config.StatusFailed,
			"msg":  err.Error(),
		})
		return
	}

	if req.Replace {
		go provider.ReplaceArticles()
		ctx.JSON(iris.Map{
			"code": config.StatusOK,
			"msg":  "替换任务已触发",
		})
		return
	}

	ctx.JSON(iris.Map{
		"code": config.StatusOK,
		"msg":  "关键词已保存",
	})
}

func HandleArticlePseudo(ctx iris.Context) {
	var req request.Archive
	if err := ctx.ReadJSON(&req); err != nil {
		ctx.JSON(iris.Map{
			"code": config.StatusFailed,
			"msg":  err.Error(),
		})
		return
	}

	archiveData, err := provider.GetArchiveDataById(req.Id)
	if err != nil {
		ctx.JSON(iris.Map{
			"code": config.StatusFailed,
			"msg":  err.Error(),
		})
		return
	}

	err = provider.PseudoOriginalArticle(archiveData)
	if err != nil {
		ctx.JSON(iris.Map{
			"code": config.StatusFailed,
			"msg":  err.Error(),
		})
		return
	}

	ctx.JSON(iris.Map{
		"code": config.StatusOK,
		"msg":  "伪原创已完成",
	})
}

func HandleDigKeywords(ctx iris.Context) {
	go provider.StartDigKeywords()

	ctx.JSON(iris.Map{
		"code": config.StatusOK,
		"msg":  "关键词拓词任务已触发",
	})
}

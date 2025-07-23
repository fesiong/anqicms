package manageController

import (
	"github.com/kataras/iris/v12"
	"kandaoni.com/anqicms/config"
	"kandaoni.com/anqicms/provider"
)

func PluginFulltextConfig(ctx iris.Context) {
	currentSite := provider.CurrentSubSite(ctx)
	setting := currentSite.PluginFulltext

	ctx.JSON(iris.Map{
		"code": config.StatusOK,
		"msg":  "",
		"data": setting,
	})
}

func PluginFulltextConfigForm(ctx iris.Context) {
	currentSite := provider.CurrentSubSite(ctx)
	var req config.PluginFulltextConfig
	if err := ctx.ReadJSON(&req); err != nil {
		ctx.JSON(iris.Map{
			"code": config.StatusFailed,
			"msg":  err.Error(),
		})
		return
	}
	oldEngine := currentSite.PluginFulltext.Engine

	currentSite.PluginFulltext.Open = req.Open
	currentSite.PluginFulltext.UseContent = req.UseContent
	currentSite.PluginFulltext.Modules = req.Modules
	currentSite.PluginFulltext.UseCategory = req.UseCategory
	currentSite.PluginFulltext.UseTag = req.UseTag
	currentSite.PluginFulltext.Engine = req.Engine
	currentSite.PluginFulltext.EngineUrl = req.EngineUrl
	currentSite.PluginFulltext.EngineUser = req.EngineUser
	currentSite.PluginFulltext.EnginePass = req.EnginePass

	err := currentSite.SaveSettingValue(provider.FulltextSettingKey, currentSite.PluginFulltext)
	if err != nil {
		ctx.JSON(iris.Map{
			"code": config.StatusFailed,
			"msg":  err.Error(),
		})
		return
	}

	currentSite.AddAdminLog(ctx, ctx.Tr("UpdateFullTextIndexConfiguration"))
	w2 := provider.GetWebsite(currentSite.Id)
	if req.Open {
		w2.CloseFulltext()
		go w2.InitFulltext(oldEngine != req.Engine)
	} else {
		w2.CloseFulltext()
	}

	ctx.JSON(iris.Map{
		"code": config.StatusOK,
		"msg":  ctx.Tr("ConfigurationUpdated"),
	})
}

func PluginFulltextRebuild(ctx iris.Context) {
	currentSite := provider.CurrentSubSite(ctx)
	if !currentSite.PluginFulltext.Open {
		ctx.JSON(iris.Map{
			"code": config.StatusFailed,
			"msg":  "Fulltext is not open",
		})
		return
	}
	w2 := provider.GetWebsite(currentSite.Id)
	w2.CloseFulltext()
	go w2.InitFulltext(true)

	ctx.JSON(iris.Map{
		"code": config.StatusOK,
		"msg":  ctx.Tr("SubmittedForBackgroundProcessing"),
	})
}

func PluginFulltextStatus(ctx iris.Context) {
	currentSite := provider.CurrentSubSite(ctx)
	status := currentSite.GetFullTextStatus()

	ctx.JSON(iris.Map{
		"code": config.StatusOK,
		"msg":  "",
		"data": status,
	})
}

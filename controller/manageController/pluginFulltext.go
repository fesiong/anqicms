package manageController

import (
	"github.com/kataras/iris/v12"
	"kandaoni.com/anqicms/config"
	"kandaoni.com/anqicms/provider"
)

func PluginFulltextConfig(ctx iris.Context) {
	currentSite := provider.CurrentSite(ctx)
	setting := currentSite.PluginFulltext

	ctx.JSON(iris.Map{
		"code": config.StatusOK,
		"msg":  "",
		"data": setting,
	})
}

func PluginFulltextConfigForm(ctx iris.Context) {
	currentSite := provider.CurrentSite(ctx)
	var req config.PluginFulltextConfig
	if err := ctx.ReadJSON(&req); err != nil {
		ctx.JSON(iris.Map{
			"code": config.StatusFailed,
			"msg":  err.Error(),
		})
		return
	}

	currentSite.PluginFulltext.Open = req.Open
	currentSite.PluginFulltext.UseContent = req.UseContent
	currentSite.PluginFulltext.Modules = req.Modules
	currentSite.PluginFulltext.UseCategory = req.UseCategory
	currentSite.PluginFulltext.UseTag = req.UseTag

	err := currentSite.SaveSettingValue(provider.FulltextSettingKey, currentSite.PluginFulltext)
	if err != nil {
		ctx.JSON(iris.Map{
			"code": config.StatusFailed,
			"msg":  err.Error(),
		})
		return
	}

	currentSite.AddAdminLog(ctx, ctx.Tr("UpdateFullTextIndexConfiguration"))
	if req.Open {
		currentSite.CloseFulltext()
		go currentSite.InitFulltext()
	} else {
		currentSite.CloseFulltext()
	}

	ctx.JSON(iris.Map{
		"code": config.StatusOK,
		"msg":  ctx.Tr("ConfigurationUpdated"),
	})
}

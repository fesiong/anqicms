package manageController

import (
	"github.com/kataras/iris/v12"
	"kandaoni.com/anqicms/config"
	"kandaoni.com/anqicms/provider"
)

func PluginRewrite(ctx iris.Context) {
	currentSite := provider.CurrentSite(ctx)
	pluginRewrite := currentSite.PluginRewrite

	ctx.JSON(iris.Map{
		"code": config.StatusOK,
		"msg":  "",
		"data": pluginRewrite,
	})
}

func PluginRewriteForm(ctx iris.Context) {
	currentSite := provider.CurrentSite(ctx)
	var req config.PluginRewriteConfig
	if err := ctx.ReadJSON(&req); err != nil {
		ctx.JSON(iris.Map{
			"code": config.StatusFailed,
			"msg":  err.Error(),
		})
		return
	}

	if currentSite.PluginRewrite.Mode != req.Mode || currentSite.PluginRewrite.Patten != req.Patten {
		currentSite.PluginRewrite.Mode = req.Mode
		currentSite.PluginRewrite.Patten = req.Patten
		err := currentSite.SaveSettingValue(provider.RewriteSettingKey, currentSite.PluginRewrite)
		if err != nil {
			ctx.JSON(iris.Map{
				"code": config.StatusFailed,
				"msg":  err.Error(),
			})
			return
		}

		currentSite.ParsePatten(true)
		currentSite.RemoveHtmlCache()
	}

	currentSite.AddAdminLog(ctx, ctx.Tr("AdjustPseudoStaticConfigurationLog", req.Mode))

	ctx.JSON(iris.Map{
		"code": config.StatusOK,
		"msg":  ctx.Tr("ConfigurationUpdated"),
	})
}

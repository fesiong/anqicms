package manageController

import (
	"github.com/kataras/iris/v12"
	"kandaoni.com/anqicms/config"
	"kandaoni.com/anqicms/provider"
)

func PluginImportApi(ctx iris.Context) {
	currentSite := provider.CurrentSite(ctx)
	importApi := currentSite.PluginImportApi

	ctx.JSON(iris.Map{
		"code": config.StatusOK,
		"msg":  "",
		"data": iris.Map{
			"token":      importApi.Token,
			"link_token": importApi.LinkToken,
			"base_url":   currentSite.System.BaseUrl,
		},
	})
}

func PluginUpdateApiToken(ctx iris.Context) {
	currentSite := provider.CurrentSite(ctx)
	var req config.PluginImportApiConfig
	if err := ctx.ReadJSON(&req); err != nil {
		ctx.JSON(iris.Map{
			"code": config.StatusFailed,
			"msg":  err.Error(),
		})
		return
	}
	if req.Token != "" {
		currentSite.PluginImportApi.Token = req.Token
	}
	if req.LinkToken != "" {
		currentSite.PluginImportApi.LinkToken = req.LinkToken
	}
	// 回写
	err := currentSite.SaveSettingValue(provider.ImportApiSettingKey, currentSite.PluginImportApi)
	if err != nil {
		ctx.JSON(iris.Map{
			"code": config.StatusFailed,
			"msg":  err.Error(),
		})
		return
	}
	currentSite.AddAdminLog(ctx, ctx.Tr("UpdateApiImportToken"))

	ctx.JSON(iris.Map{
		"code": config.StatusOK,
		"msg":  ctx.Tr("TokenUpdated"),
	})
}

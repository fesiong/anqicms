package manageController

import (
	"github.com/kataras/iris/v12"
	"kandaoni.com/anqicms/config"
	"kandaoni.com/anqicms/provider"
)

func PluginGetGoogleSetting(ctx iris.Context) {
	currentSite := provider.CurrentSubSite(ctx)
	setting := currentSite.GetGoogleAuthSetting()

	ctx.JSON(iris.Map{
		"code": config.StatusOK,
		"msg":  "",
		"data": setting,
	})
}

func PluginSaveGoogleSetting(ctx iris.Context) {
	currentSite := provider.CurrentSubSite(ctx)
	var req config.PluginGoogleAuthConfig
	if err := ctx.ReadJSON(&req); err != nil {
		ctx.JSON(iris.Map{
			"code": config.StatusFailed,
			"msg":  err.Error(),
		})
		return
	}

	err := currentSite.SaveSettingValue(provider.GoogleAuthSettingKey, req)
	if err != nil {
		ctx.JSON(iris.Map{
			"code": config.StatusFailed,
			"msg":  err.Error(),
		})
		return
	}

	currentSite.AddAdminLog(ctx, ctx.Tr("UpdateGoogleConfiguration"))
	// 更新GoogleAuth
	w2 := provider.GetWebsite(currentSite.Id)
	_ = w2.GetGoogleAuthConfig(true)

	ctx.JSON(iris.Map{
		"code": config.StatusOK,
		"msg":  ctx.Tr("ConfigurationUpdated"),
	})
}

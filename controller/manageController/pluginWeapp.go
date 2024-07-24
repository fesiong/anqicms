package manageController

import (
	"github.com/kataras/iris/v12"
	"kandaoni.com/anqicms/config"
	"kandaoni.com/anqicms/provider"
)

func PluginWeappConfig(ctx iris.Context) {
	currentSite := provider.CurrentSite(ctx)
	setting := currentSite.PluginWeapp
	// 增加serverUrl
	setting.ServerUrl = currentSite.System.BaseUrl + "/api/wechat"

	ctx.JSON(iris.Map{
		"code": config.StatusOK,
		"msg":  "",
		"data": setting,
	})
}

func PluginWeappConfigForm(ctx iris.Context) {
	currentSite := provider.CurrentSite(ctx)
	var req config.PluginWeappConfig
	if err := ctx.ReadJSON(&req); err != nil {
		ctx.JSON(iris.Map{
			"code": config.StatusFailed,
			"msg":  err.Error(),
		})
		return
	}

	currentSite.PluginWeapp.AppID = req.AppID
	currentSite.PluginWeapp.AppSecret = req.AppSecret
	currentSite.PluginWeapp.Token = req.Token
	currentSite.PluginWeapp.EncodingAESKey = req.EncodingAESKey

	err := currentSite.SaveSettingValue(provider.WeappSettingKey, currentSite.PluginWeapp)
	if err != nil {
		ctx.JSON(iris.Map{
			"code": config.StatusFailed,
			"msg":  err.Error(),
		})
		return
	}
	// 强制更新信息
	currentSite.GetWeappClient(true)

	currentSite.AddAdminLog(ctx, ctx.Tr("UpdateMiniProgram"))

	ctx.JSON(iris.Map{
		"code": config.StatusOK,
		"msg":  ctx.Tr("ConfigurationUpdated"),
	})
}

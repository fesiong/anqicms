package manageController

import (
	"fmt"
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

	currentSite.AddAdminLog(ctx, fmt.Sprintf("更新小程序信息"))

	ctx.JSON(iris.Map{
		"code": config.StatusOK,
		"msg":  "配置已更新",
	})
}

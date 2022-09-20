package manageController

import (
	"fmt"
	"github.com/kataras/iris/v12"
	"kandaoni.com/anqicms/config"
	"kandaoni.com/anqicms/provider"
	"kandaoni.com/anqicms/request"
)

func PluginWeappConfig(ctx iris.Context) {
	pluginRewrite := config.JsonData.PluginWeapp

	ctx.JSON(iris.Map{
		"code": config.StatusOK,
		"msg":  "",
		"data": pluginRewrite,
	})
}

func PluginWeappConfigForm(ctx iris.Context) {
	var req request.PluginWeappConfig
	if err := ctx.ReadJSON(&req); err != nil {
		ctx.JSON(iris.Map{
			"code": config.StatusFailed,
			"msg":  err.Error(),
		})
		return
	}

	config.JsonData.PluginWeapp.AppID = req.AppID
	config.JsonData.PluginWeapp.AppSecret = req.AppSecret
	config.JsonData.PluginWeapp.Token = req.Token
	config.JsonData.PluginWeapp.EncodingAESKey = req.EncodingAESKey

	err := config.WriteConfig()
	if err != nil {
		ctx.JSON(iris.Map{
			"code": config.StatusFailed,
			"msg":  err.Error(),
		})
		return
	}
	// 强制更新信息
	provider.GetWeappClient(true)

	provider.AddAdminLog(ctx, fmt.Sprintf("更新小程序信息"))

	ctx.JSON(iris.Map{
		"code": config.StatusOK,
		"msg":  "配置已更新",
	})
}

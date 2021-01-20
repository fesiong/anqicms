package manageController

import (
	"github.com/kataras/iris/v12"
	"irisweb/config"
	"irisweb/request"
)

func PluginPush(ctx iris.Context) {
	pluginPush := config.JsonData.PluginPush

	ctx.JSON(iris.Map{
		"code": config.StatusOK,
		"msg":  "",
		"data": pluginPush,
	})
}

func PluginPushForm(ctx iris.Context) {
	var req request.PluginPushConfig
	if err := ctx.ReadJSON(&req); err != nil {
		ctx.JSON(iris.Map{
			"code": config.StatusFailed,
			"msg":  err.Error(),
		})
		return
	}

	config.JsonData.PluginPush.BaiduApi = req.BaiduApi
	config.JsonData.PluginPush.BingApi = req.BingApi
	config.JsonData.PluginPush.JsCode = req.JsCode

	err := config.WriteConfig()
	if err != nil {
		ctx.JSON(iris.Map{
			"code": config.StatusFailed,
			"msg":  err.Error(),
		})
		return
	}

	ctx.JSON(iris.Map{
		"code": config.StatusOK,
		"msg":  "配置已更新",
	})
}

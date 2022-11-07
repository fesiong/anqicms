package manageController

import (
	"fmt"
	"github.com/kataras/iris/v12"
	"kandaoni.com/anqicms/config"
	"kandaoni.com/anqicms/provider"
)

func PluginPush(ctx iris.Context) {
	pluginPush := config.JsonData.PluginPush

	ctx.JSON(iris.Map{
		"code": config.StatusOK,
		"msg":  "",
		"data": pluginPush,
	})
}

func PluginPushLogList(ctx iris.Context) {
	//不需要分页，只显示最后20条
	list, err := provider.GetLastPushList()
	if err != nil {
		ctx.JSON(iris.Map{
			"code": config.StatusFailed,
			"msg":  "",
		})
		return
	}

	ctx.JSON(iris.Map{
		"code": config.StatusOK,
		"msg":  "",
		"data": list,
	})
}

func PluginPushForm(ctx iris.Context) {
	var req config.PluginPushConfig
	if err := ctx.ReadJSON(&req); err != nil {
		ctx.JSON(iris.Map{
			"code": config.StatusFailed,
			"msg":  err.Error(),
		})
		return
	}

	config.JsonData.PluginPush.BaiduApi = req.BaiduApi
	config.JsonData.PluginPush.BingApi = req.BingApi
	config.JsonData.PluginPush.JsCodes = req.JsCodes

	err := provider.SaveSettingValue(provider.PushSettingKey, config.JsonData.PluginPush)
	if err != nil {
		ctx.JSON(iris.Map{
			"code": config.StatusFailed,
			"msg":  err.Error(),
		})
		return
	}
	provider.DeleteCacheIndex()

	provider.AddAdminLog(ctx, fmt.Sprintf("更新搜索引擎推送配置"))

	ctx.JSON(iris.Map{
		"code": config.StatusOK,
		"msg":  "配置已更新",
	})
}

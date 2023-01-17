package manageController

import (
	"fmt"
	"github.com/kataras/iris/v12"
	"kandaoni.com/anqicms/config"
	"kandaoni.com/anqicms/provider"
)

func PluginPush(ctx iris.Context) {
	currentSite := provider.CurrentSite(ctx)
	pluginPush := currentSite.PluginPush

	ctx.JSON(iris.Map{
		"code": config.StatusOK,
		"msg":  "",
		"data": pluginPush,
	})
}

func PluginPushLogList(ctx iris.Context) {
	currentSite := provider.CurrentSite(ctx)
	//不需要分页，只显示最后20条
	list, err := currentSite.GetLastPushList()
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
	currentSite := provider.CurrentSite(ctx)
	var req config.PluginPushConfig
	if err := ctx.ReadJSON(&req); err != nil {
		ctx.JSON(iris.Map{
			"code": config.StatusFailed,
			"msg":  err.Error(),
		})
		return
	}

	currentSite.PluginPush.BaiduApi = req.BaiduApi
	currentSite.PluginPush.BingApi = req.BingApi
	currentSite.PluginPush.JsCodes = req.JsCodes

	err := currentSite.SaveSettingValue(provider.PushSettingKey, currentSite.PluginPush)
	if err != nil {
		ctx.JSON(iris.Map{
			"code": config.StatusFailed,
			"msg":  err.Error(),
		})
		return
	}
	currentSite.DeleteCacheIndex()

	currentSite.AddAdminLog(ctx, fmt.Sprintf("更新搜索引擎推送配置"))

	ctx.JSON(iris.Map{
		"code": config.StatusOK,
		"msg":  "配置已更新",
	})
}

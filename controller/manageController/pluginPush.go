package manageController

import (
	"encoding/json"
	"github.com/kataras/iris/v12"
	"kandaoni.com/anqicms/config"
	"kandaoni.com/anqicms/library"
	"kandaoni.com/anqicms/provider"
	"kandaoni.com/anqicms/request"
)

// PluginPush 获取当前站点的搜索引擎推送插件配置。
func PluginPush(ctx iris.Context) {
	currentSite := provider.CurrentSubSite(ctx)
	pluginPush := currentSite.PluginPush

	ctx.JSON(iris.Map{
		"code": config.StatusOK,
		"msg":  "",
		"data": pluginPush,
	})
}

// PluginPushLogList 获取最近 20 条推送记录列表。
func PluginPushLogList(ctx iris.Context) {
	currentSite := provider.CurrentSubSite(ctx)
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

// PluginPushForm 保存搜索引擎推送插件配置。
func PluginPushForm(ctx iris.Context) {
	currentSite := provider.CurrentSubSite(ctx)
	var req config.PluginPushConfig
	if err := ctx.ReadJSON(&req); err != nil {
		ctx.JSON(iris.Map{
			"code": config.StatusFailed,
			"msg":  err.Error(),
		})
		return
	}
	// 部分更新：以现值为基底，仅覆盖请求体中显式出现的字段，未传字段保持原配置。
	body, err := ctx.GetBody()
	if err != nil {
		ctx.JSON(iris.Map{
			"code": config.StatusFailed,
			"msg":  err.Error(),
		})
		return
	}
	var payload map[string]json.RawMessage
	if err := json.Unmarshal(body, &payload); err != nil {
		ctx.JSON(iris.Map{
			"code": config.StatusFailed,
			"msg":  err.Error(),
		})
		return
	}
	req = *currentSite.PluginPush
	if err := library.MergeJSONPayload(payload, &req); err != nil {
		ctx.JSON(iris.Map{
			"code": config.StatusFailed,
			"msg":  err.Error(),
		})
		return
	}

	currentSite.PluginPush.BaiduApi = req.BaiduApi
	currentSite.PluginPush.BingApi = req.BingApi
	currentSite.PluginPush.GoogleJson = req.GoogleJson
	currentSite.PluginPush.JsCodes = req.JsCodes

	err = currentSite.SaveSettingValue(provider.PushSettingKey, currentSite.PluginPush)
	if err != nil {
		ctx.JSON(iris.Map{
			"code": config.StatusFailed,
			"msg":  err.Error(),
		})
		return
	}
	currentSite.DeleteCacheIndex()

	currentSite.AddAdminLog(ctx, ctx.Tr("UpdateSearchEnginePushConfiguration"))

	ctx.JSON(iris.Map{
		"code": config.StatusOK,
		"msg":  ctx.Tr("ConfigurationUpdated"),
	})
}

// PluginPushUrls 将一组URL地址通过已配置的推送地址进行推送
func PluginPushUrls(ctx iris.Context) {
	currentSite := provider.CurrentSubSite(ctx)
	var req request.PluginPushUrlsRequest
	if err := ctx.ReadJSON(&req); err != nil {
		ctx.JSON(iris.Map{
			"code": config.StatusFailed,
			"msg":  err.Error(),
		})
		return
	}

	err := currentSite.PushArchives(req.Urls)
	if err != nil {
		ctx.JSON(iris.Map{
			"code": config.StatusFailed,
			"msg":  err.Error(),
		})
		return
	}
	ctx.JSON(iris.Map{
		"code": config.StatusOK,
		"msg":  ctx.Tr("PushUrlsSuccess"),
	})
}

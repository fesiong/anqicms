package manageController

import (
	"encoding/json"
	"github.com/kataras/iris/v12"
	"kandaoni.com/anqicms/config"
	"kandaoni.com/anqicms/library"
	"kandaoni.com/anqicms/provider"
)

// PluginGetJsonLdConfig 获取结构化数据插件的配置信息。
func PluginGetJsonLdConfig(ctx iris.Context) {
	currentSite := provider.CurrentSubSite(ctx)
	setting := currentSite.GetJsonLdSetting()

	ctx.JSON(iris.Map{
		"code": config.StatusOK,
		"msg":  "",
		"data": setting,
	})
}

// PluginSaveJsonLdConfig 保存结构化数据插件的配置信息。
func PluginSaveJsonLdConfig(ctx iris.Context) {
	currentSite := provider.CurrentSubSite(ctx)
	var req config.PluginJsonLdConfig
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
	if currentSite.PluginJsonLd != nil {
		req = *currentSite.PluginJsonLd
	}
	if err := library.MergeJSONPayload(payload, &req); err != nil {
		ctx.JSON(iris.Map{
			"code": config.StatusFailed,
			"msg":  err.Error(),
		})
		return
	}

	w2 := provider.GetWebsite(currentSite.Id)
	w2.PluginJsonLd = &req
	err = currentSite.SaveSettingValue(provider.JsonLdSettingKey, w2.PluginJsonLd)
	if err != nil {
		ctx.JSON(iris.Map{
			"code": config.StatusFailed,
			"msg":  err.Error(),
		})
		return
	}

	currentSite.AddAdminLog(ctx, ctx.Tr("UpdateJsonLdConfiguration"))

	ctx.JSON(iris.Map{
		"code": config.StatusOK,
		"msg":  ctx.Tr("ConfigurationUpdated"),
	})
}

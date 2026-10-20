package manageController

import (
	"encoding/json"
	"github.com/kataras/iris/v12"
	"kandaoni.com/anqicms/config"
	"kandaoni.com/anqicms/provider"
)

// PluginGetGoogleSetting 获取Google认证插件的配置信息。
func PluginGetGoogleSetting(ctx iris.Context) {
	currentSite := provider.CurrentSubSite(ctx)
	setting := currentSite.GetGoogleAuthSetting()

	ctx.JSON(iris.Map{
		"code": config.StatusOK,
		"msg":  "",
		"data": setting,
	})
}

// PluginSaveGoogleSetting 保存Google认证插件的配置信息。
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
	if raw := currentSite.GetSettingValue(provider.GoogleAuthSettingKey); raw != "" {
		// 部分更新基底：以已保存配置为现值，随后的 Unmarshal 只覆盖请求体中出现的字段。
		_ = json.Unmarshal([]byte(raw), &req)
	}
	if err := json.Unmarshal(body, &req); err != nil {
		ctx.JSON(iris.Map{
			"code": config.StatusFailed,
			"msg":  err.Error(),
		})
		return
	}

	err = currentSite.SaveSettingValue(provider.GoogleAuthSettingKey, req)
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

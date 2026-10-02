package manageController

import (
	"github.com/kataras/iris/v12"
	"kandaoni.com/anqicms/config"
	"kandaoni.com/anqicms/provider"
)

// PluginGetAkismetSetting 获取 Akismet 垃圾评论过滤和 reCAPTCHA 插件的配置信息。
func PluginGetAkismetSetting(ctx iris.Context) {
	currentSite := provider.CurrentSubSite(ctx)
	setting := currentSite.GetAkismetSetting(true)

	ctx.JSON(iris.Map{
		"code": config.StatusOK,
		"msg":  "",
		"data": setting,
	})
}

// PluginSaveAkismetSetting 保存 Akismet 垃圾评论过滤和 reCAPTCHA 插件的配置信息。
func PluginSaveAkismetSetting(ctx iris.Context) {
	currentSite := provider.CurrentSubSite(ctx)
	var req config.PluginAkismetConfig
	if err := ctx.ReadJSON(&req); err != nil {
		ctx.JSON(iris.Map{
			"code": config.StatusFailed,
			"msg":  err.Error(),
		})
		return
	}

	err := currentSite.SaveSettingValue(provider.AkismetSettingKey, req)
	if err != nil {
		ctx.JSON(iris.Map{
			"code": config.StatusFailed,
			"msg":  err.Error(),
		})
		return
	}
	// clean cache
	currentSite.Cache.Delete(provider.AkismetSettingKey)

	currentSite.AddAdminLog(ctx, ctx.Tr("UpdateAkismetConfiguration"))
	// 更新Akismet
	w2 := provider.GetWebsite(currentSite.Id)
	w2.InitAkismet()

	ctx.JSON(iris.Map{
		"code": config.StatusOK,
		"msg":  ctx.Tr("ConfigurationUpdated"),
	})
}

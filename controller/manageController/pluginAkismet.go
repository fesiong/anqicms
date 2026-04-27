package manageController

import (
	"github.com/kataras/iris/v12"
	"kandaoni.com/anqicms/config"
	"kandaoni.com/anqicms/provider"
)

func PluginGetAkismetSetting(ctx iris.Context) {
	currentSite := provider.CurrentSubSite(ctx)
	setting := currentSite.GetAkismetSetting()

	ctx.JSON(iris.Map{
		"code": config.StatusOK,
		"msg":  "",
		"data": setting,
	})
}

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

	currentSite.AddAdminLog(ctx, ctx.Tr("UpdateAkismetConfiguration"))
	// 更新Akismet
	w2 := provider.GetWebsite(currentSite.Id)
	w2.InitAkismet()

	ctx.JSON(iris.Map{
		"code": config.StatusOK,
		"msg":  ctx.Tr("ConfigurationUpdated"),
	})
}

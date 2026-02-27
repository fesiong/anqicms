package manageController

import (
	"github.com/kataras/iris/v12"
	"kandaoni.com/anqicms/config"
	"kandaoni.com/anqicms/provider"
)

func PluginTimeFactorSetting(ctx iris.Context) {
	currentSite := provider.CurrentSite(ctx)
	setting := currentSite.PluginTimeFactor

	ctx.JSON(iris.Map{
		"code": config.StatusOK,
		"msg":  "",
		"data": setting,
	})
}

func PluginTimeFactorSettingSave(ctx iris.Context) {
	currentSite := provider.CurrentSite(ctx)
	var req config.PluginTimeFactor
	if err := ctx.ReadJSON(&req); err != nil {
		ctx.JSON(iris.Map{
			"code": config.StatusFailed,
			"msg":  err.Error(),
		})
		return
	}

	setting := currentSite.PluginTimeFactor
	req.TodayCount = setting.TodayCount
	req.LastSent = setting.LastSent
	req.TodayUpdate = setting.TodayUpdate
	req.LastUpdate = setting.LastUpdate
	err := currentSite.SaveSettingValue(provider.TimeFactorKey, &req)
	if err != nil {
		ctx.JSON(iris.Map{
			"code": config.StatusFailed,
			"msg":  err.Error(),
		})
		return
	}
	// 回写
	req.UpdateRunning = setting.UpdateRunning
	w2 := provider.GetWebsite(currentSite.Id)
	w2.PluginTimeFactor = &req

	currentSite.AddAdminLog(ctx, ctx.Tr("UpdateTimeFactorTimedReleaseConfiguration"))

	ctx.JSON(iris.Map{
		"code": config.StatusOK,
		"msg":  ctx.Tr("ConfigurationUpdated"),
	})
}

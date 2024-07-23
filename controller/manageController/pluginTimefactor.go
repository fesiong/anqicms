package manageController

import (
	"github.com/kataras/iris/v12"
	"kandaoni.com/anqicms/config"
	"kandaoni.com/anqicms/provider"
)

func PluginTimeFactorSetting(ctx iris.Context) {
	currentSite := provider.CurrentSite(ctx)
	setting := currentSite.GetTimeFactorSetting()

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

	setting := currentSite.GetTimeFactorSetting()
	req.TodayCount = setting.TodayCount
	req.LastSent = setting.LastSent
	err := currentSite.SaveSettingValue(provider.TimeFactorKey, &req)
	if err != nil {
		ctx.JSON(iris.Map{
			"code": config.StatusFailed,
			"msg":  err.Error(),
		})
		return
	}

	currentSite.AddAdminLog(ctx, ctx.Tr("更新时间因子-定时发布配置"))

	ctx.JSON(iris.Map{
		"code": config.StatusOK,
		"msg":  ctx.Tr("配置已更新"),
	})
}

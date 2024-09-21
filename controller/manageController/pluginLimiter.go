package manageController

import (
	"github.com/kataras/iris/v12"
	"kandaoni.com/anqicms/config"
	"kandaoni.com/anqicms/provider"
)

func PluginGetLimiterSetting(ctx iris.Context) {
	currentSite := provider.CurrentSite(ctx)
	setting := currentSite.GetLimiterSetting()

	ctx.JSON(iris.Map{
		"code": config.StatusOK,
		"msg":  "",
		"data": setting,
	})
}

func PluginSaveLimiterSetting(ctx iris.Context) {
	currentSite := provider.CurrentSite(ctx)
	var req config.PluginLimiter
	if err := ctx.ReadJSON(&req); err != nil {
		ctx.JSON(iris.Map{
			"code": config.StatusFailed,
			"msg":  err.Error(),
		})
		return
	}

	err := currentSite.SaveSettingValue(provider.LimiterSettingKey, req)
	if err != nil {
		ctx.JSON(iris.Map{
			"code": config.StatusFailed,
			"msg":  err.Error(),
		})
		return
	}

	currentSite.AddAdminLog(ctx, ctx.Tr("UpdateLimiterConfiguration"))
	// 更新limiter
	currentSite.InitLimiter()

	ctx.JSON(iris.Map{
		"code": config.StatusOK,
		"msg":  ctx.Tr("ConfigurationUpdated"),
	})
}

func PluginGetBlockedIPs(ctx iris.Context) {
	currentSite := provider.CurrentSite(ctx)

	blockIPs := currentSite.Limiter.GetBlockIPs()

	ctx.JSON(iris.Map{
		"code": config.StatusOK,
		"msg":  "",
		"data": blockIPs,
	})
}

func PluginRemoveBlockedIP(ctx iris.Context) {
	currentSite := provider.CurrentSite(ctx)
	ip := ctx.URLParam("ip")

	currentSite.Limiter.RemoveBlockedIP(ip)

	ctx.JSON(iris.Map{
		"code": config.StatusOK,
		"msg":  ctx.Tr("DeleteSuccessful"),
	})
}

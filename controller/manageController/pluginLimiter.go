package manageController

import (
	"github.com/kataras/iris/v12"
	"kandaoni.com/anqicms/config"
	"kandaoni.com/anqicms/provider"
	"kandaoni.com/anqicms/request"
)

func PluginGetLimiterSetting(ctx iris.Context) {
	currentSite := provider.CurrentSubSite(ctx)
	setting := currentSite.GetLimiterSetting()

	ctx.JSON(iris.Map{
		"code": config.StatusOK,
		"msg":  "",
		"data": setting,
	})
}

func PluginSaveLimiterSetting(ctx iris.Context) {
	currentSite := provider.CurrentSubSite(ctx)
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
	w2 := provider.GetWebsite(currentSite.Id)
	w2.InitLimiter()

	ctx.JSON(iris.Map{
		"code": config.StatusOK,
		"msg":  ctx.Tr("ConfigurationUpdated"),
	})
}

func PluginGetBlockedIPs(ctx iris.Context) {
	currentSite := provider.CurrentSubSite(ctx)

	var blockIPs []provider.BlockIP
	if currentSite.Limiter != nil {
		blockIPs = currentSite.Limiter.GetBlockIPs()
	}

	ctx.JSON(iris.Map{
		"code": config.StatusOK,
		"msg":  "",
		"data": blockIPs,
	})
}

func PluginRemoveBlockedIP(ctx iris.Context) {
	currentSite := provider.CurrentSubSite(ctx)
	var req request.PluginLimiterRemoveIPRequest
	if err := ctx.ReadJSON(&req); err != nil {
		ctx.JSON(iris.Map{
			"code": config.StatusFailed,
			"msg":  err.Error(),
		})
		return
	}

	if currentSite.Limiter != nil {
		currentSite.Limiter.RemoveBlockedIP(req.Ip)
	}

	ctx.JSON(iris.Map{
		"code": config.StatusOK,
		"msg":  ctx.Tr("DeleteSuccessful"),
	})
}

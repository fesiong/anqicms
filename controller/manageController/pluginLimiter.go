package manageController

import (
	"github.com/kataras/iris/v12"
	"kandaoni.com/anqicms/config"
	"kandaoni.com/anqicms/provider"
	"kandaoni.com/anqicms/request"
)

// PluginGetLimiterSetting 获取当前站点的防刷（限流）设置。
func PluginGetLimiterSetting(ctx iris.Context) {
	currentSite := provider.CurrentSubSite(ctx)
	setting := currentSite.GetLimiterSetting()

	ctx.JSON(iris.Map{
		"code": config.StatusOK,
		"msg":  "",
		"data": setting,
	})
}

// PluginSaveLimiterSetting 保存防刷（限流）设置并重新初始化限流器。
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

// PluginGetBlockedIPs 获取当前站点限流器中被屏蔽的 IP 列表。
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

// PluginRemoveBlockedIP 删除当前站点限流器中临时屏蔽的 IP。
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

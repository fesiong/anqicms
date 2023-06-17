package manageController

import (
	"fmt"
	"github.com/kataras/iris/v12"
	"kandaoni.com/anqicms/config"
	"kandaoni.com/anqicms/provider"
)

func PluginHtmlCacheConfig(ctx iris.Context) {
	currentSite := provider.CurrentSite(ctx)
	pluginHtmlCache := currentSite.PluginHtmlCache

	ctx.JSON(iris.Map{
		"code": config.StatusOK,
		"msg":  "",
		"data": pluginHtmlCache,
	})
}

func PluginHtmlCacheConfigForm(ctx iris.Context) {
	currentSite := provider.CurrentSite(ctx)
	var req config.PluginHtmlCache
	if err := ctx.ReadJSON(&req); err != nil {
		ctx.JSON(iris.Map{
			"code": config.StatusFailed,
			"msg":  err.Error(),
		})
		return
	}
	currentSite.PluginHtmlCache.Open = req.Open
	currentSite.PluginHtmlCache.IndexCache = req.IndexCache
	currentSite.PluginHtmlCache.ListCache = req.ListCache
	currentSite.PluginHtmlCache.DetailCache = req.DetailCache

	err := currentSite.SaveSettingValue(provider.HtmlCacheSettingKey, currentSite.PluginHtmlCache)
	if err != nil {
		ctx.JSON(iris.Map{
			"code": config.StatusFailed,
			"msg":  err.Error(),
		})
		return
	}

	currentSite.AddAdminLog(ctx, fmt.Sprintf("更新缓存配置"))

	ctx.JSON(iris.Map{
		"code": config.StatusOK,
		"msg":  "配置已更新",
	})
}

func PluginHtmlCacheBuild(ctx iris.Context) {
	currentSite := provider.CurrentSite(ctx)
	var req config.PluginHtmlCache
	if err := ctx.ReadJSON(&req); err != nil {
		ctx.JSON(iris.Map{
			"code": config.StatusFailed,
			"msg":  err.Error(),
		})
		return
	}

	//开始生成
	go currentSite.BuildHtmlCache()

	currentSite.AddAdminLog(ctx, fmt.Sprintf("手动生成缓存"))

	ctx.JSON(iris.Map{
		"code": config.StatusOK,
		"msg":  "生成任务执行中",
	})
}

func PluginHtmlCacheBuildStatus(ctx iris.Context) {
	currentSite := provider.CurrentSite(ctx)
	status := currentSite.GetHtmlCacheStatus()

	ctx.JSON(iris.Map{
		"code": config.StatusOK,
		"msg":  "",
		"data": status,
	})
}

func PluginCleanHtmlCache(ctx iris.Context) {
	currentSite := provider.CurrentSite(ctx)
	currentSite.RemoveHtmlCache()

	ctx.JSON(iris.Map{
		"code": config.StatusOK,
		"msg":  "",
	})
}

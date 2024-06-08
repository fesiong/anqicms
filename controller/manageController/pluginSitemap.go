package manageController

import (
	"fmt"
	"github.com/kataras/iris/v12"
	"kandaoni.com/anqicms/config"
	"kandaoni.com/anqicms/provider"
)

func PluginSitemap(ctx iris.Context) {
	currentSite := provider.CurrentSite(ctx)
	pluginSitemap := currentSite.PluginSitemap
	if pluginSitemap.ExcludeModuleIds == nil {
		pluginSitemap.ExcludeModuleIds = []uint{}
	}
	if pluginSitemap.ExcludeCategoryIds == nil {
		pluginSitemap.ExcludeCategoryIds = []uint{}
	}
	if pluginSitemap.ExcludePageIds == nil {
		pluginSitemap.ExcludePageIds = []uint{}
	}
	//由于sitemap的更新可能很频繁，因此sitemap的更新时间直接写入一个文件中
	pluginSitemap.UpdatedTime = currentSite.GetSitemapTime()
	// 写入Sitemap的url
	pluginSitemap.SitemapURL = currentSite.System.BaseUrl + "/sitemap." + pluginSitemap.Type

	ctx.JSON(iris.Map{
		"code": config.StatusOK,
		"msg":  "",
		"data": pluginSitemap,
	})
}

func PluginSitemapForm(ctx iris.Context) {
	currentSite := provider.CurrentSite(ctx)
	var req config.PluginSitemapConfig
	if err := ctx.ReadJSON(&req); err != nil {
		ctx.JSON(iris.Map{
			"code": config.StatusFailed,
			"msg":  err.Error(),
		})
		return
	}
	if req.Type != "xml" {
		req.Type = "txt"
	}
	oldType := currentSite.PluginSitemap.Type
	currentSite.PluginSitemap.AutoBuild = req.AutoBuild
	currentSite.PluginSitemap.Type = req.Type
	currentSite.PluginSitemap.ExcludeTag = req.ExcludeTag
	currentSite.PluginSitemap.ExcludeCategoryIds = req.ExcludeCategoryIds
	currentSite.PluginSitemap.ExcludePageIds = req.ExcludePageIds
	currentSite.PluginSitemap.ExcludeModuleIds = req.ExcludeModuleIds

	err := currentSite.SaveSettingValue(provider.SitemapSettingKey, currentSite.PluginSitemap)
	if err != nil {
		ctx.JSON(iris.Map{
			"code": config.StatusFailed,
			"msg":  err.Error(),
		})
		return
	}
	// 当新旧Sitemap不一致的时候，就清理Sitemap
	if oldType != req.Type {
		currentSite.DeleteSitemap(oldType)
	}

	currentSite.AddAdminLog(ctx, fmt.Sprintf("更新SItemap配置"))

	ctx.JSON(iris.Map{
		"code": config.StatusOK,
		"msg":  "配置已更新",
	})
}

func PluginSitemapBuild(ctx iris.Context) {
	currentSite := provider.CurrentSite(ctx)
	var req config.PluginSitemapConfig
	if err := ctx.ReadJSON(&req); err != nil {
		ctx.JSON(iris.Map{
			"code": config.StatusFailed,
			"msg":  err.Error(),
		})
		return
	}
	if req.Type != "xml" {
		req.Type = "txt"
	}
	//先保存一次
	currentSite.PluginSitemap.AutoBuild = req.AutoBuild
	currentSite.PluginSitemap.Type = req.Type
	currentSite.PluginSitemap.ExcludeTag = req.ExcludeTag
	currentSite.PluginSitemap.ExcludeCategoryIds = req.ExcludeCategoryIds
	currentSite.PluginSitemap.ExcludePageIds = req.ExcludePageIds
	currentSite.PluginSitemap.ExcludeModuleIds = req.ExcludeModuleIds

	err := currentSite.SaveSettingValue(provider.SitemapSettingKey, currentSite.PluginSitemap)
	if err != nil {
		ctx.JSON(iris.Map{
			"code": config.StatusFailed,
			"msg":  err.Error(),
		})
		return
	}

	//开始生成sitemap
	err = currentSite.BuildSitemap()
	if err != nil {
		ctx.JSON(iris.Map{
			"code": config.StatusFailed,
			"msg":  err.Error(),
		})
		return
	}

	pluginSitemap := currentSite.PluginSitemap

	//由于sitemap的更新可能很频繁，因此sitemap的更新时间直接写入一个文件中
	pluginSitemap.UpdatedTime = currentSite.GetSitemapTime()
	// 写入Sitemap的url
	pluginSitemap.SitemapURL = currentSite.System.BaseUrl + "/sitemap." + pluginSitemap.Type

	currentSite.AddAdminLog(ctx, fmt.Sprintf("手动更新sitemap"))

	ctx.JSON(iris.Map{
		"code": config.StatusOK,
		"msg":  "Sitemap已更新",
		"data": pluginSitemap,
	})
}

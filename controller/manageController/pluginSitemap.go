package manageController

import (
	"encoding/json"

	"github.com/kataras/iris/v12"
	"kandaoni.com/anqicms/config"
	"kandaoni.com/anqicms/library"
	"kandaoni.com/anqicms/provider"
)

// PluginSitemap 获取当前站点的 Sitemap 配置，附带最新生成时间和访问地址。
func PluginSitemap(ctx iris.Context) {
	currentSite := provider.CurrentSubSite(ctx)
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
	frontUrl := currentSite.System.BaseUrl
	if currentSite.System.FrontUrl != "" {
		frontUrl = currentSite.System.FrontUrl
	}
	pluginSitemap.SitemapURL = frontUrl + "/sitemap." + pluginSitemap.Type

	ctx.JSON(iris.Map{
		"code": config.StatusOK,
		"msg":  "",
		"data": pluginSitemap,
	})
}

// PluginSitemapForm 保存 Sitemap 配置，并在类型变化时清理旧的 Sitemap。
// 支持部分更新：请求体中显式出现的字段才会覆盖配置，未出现的字段保持原值。
func PluginSitemapForm(ctx iris.Context) {
	currentSite := provider.CurrentSubSite(ctx)
	var req config.PluginSitemapConfig
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
	req = *currentSite.PluginSitemap
	if err := library.MergeJSONPayload(payload, &req); err != nil {
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
	if req.PageSize > 0 {
		currentSite.PluginSitemap.PageSize = req.PageSize
	}
	currentSite.PluginSitemap.ExcludeCategoryIds = req.ExcludeCategoryIds
	currentSite.PluginSitemap.ExcludePageIds = req.ExcludePageIds
	currentSite.PluginSitemap.ExcludeModuleIds = req.ExcludeModuleIds

	err = currentSite.SaveSettingValue(provider.SitemapSettingKey, currentSite.PluginSitemap)
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

	currentSite.AddAdminLog(ctx, ctx.Tr("UpdateSitemapConfiguration"))

	ctx.JSON(iris.Map{
		"code": config.StatusOK,
		"msg":  ctx.Tr("ConfigurationUpdated"),
	})
}

// PluginSitemapBuild 执行 Sitemap 生成（重建）操作，并字段保存 Sitemap 配置
func PluginSitemapBuild(ctx iris.Context) {
	currentSite := provider.CurrentSubSite(ctx)
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
	frontUrl := currentSite.System.BaseUrl
	if currentSite.System.FrontUrl != "" {
		frontUrl = currentSite.System.FrontUrl
	}
	pluginSitemap.SitemapURL = frontUrl + "/sitemap." + pluginSitemap.Type

	currentSite.AddAdminLog(ctx, ctx.Tr("UpdateSitemapManually"))

	ctx.JSON(iris.Map{
		"code": config.StatusOK,
		"msg":  ctx.Tr("SitemapUpdated"),
		"data": pluginSitemap,
	})
}

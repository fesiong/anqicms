package manageController

import (
	"fmt"
	"github.com/kataras/iris/v12"
	"kandaoni.com/anqicms/config"
	"kandaoni.com/anqicms/provider"
	"kandaoni.com/anqicms/request"
)

func PluginSitemap(ctx iris.Context) {
	pluginSitemap := config.JsonData.PluginSitemap

	//由于sitemap的更新可能很频繁，因此sitemap的更新时间直接写入一个文件中
	pluginSitemap.UpdatedTime = provider.GetSitemapTime()
	// 写入Sitemap的url
	pluginSitemap.SitemapURL = config.JsonData.System.BaseUrl + "/sitemap." + pluginSitemap.Type

	ctx.JSON(iris.Map{
		"code": config.StatusOK,
		"msg":  "",
		"data": pluginSitemap,
	})
}

func PluginSitemapForm(ctx iris.Context) {
	var req request.PluginSitemapConfig
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
	config.JsonData.PluginSitemap.AutoBuild = req.AutoBuild
	config.JsonData.PluginSitemap.Type = req.Type

	err := config.WriteConfig()
	if err != nil {
		ctx.JSON(iris.Map{
			"code": config.StatusFailed,
			"msg":  err.Error(),
		})
		return
	}

	provider.AddAdminLog(ctx, fmt.Sprintf("更新SItemap配置"))

	ctx.JSON(iris.Map{
		"code": config.StatusOK,
		"msg":  "配置已更新",
	})
}

func PluginSitemapBuild(ctx iris.Context) {
	var req request.PluginSitemapConfig
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
	config.JsonData.PluginSitemap.AutoBuild = req.AutoBuild
	config.JsonData.PluginSitemap.Type = req.Type

	err := config.WriteConfig()
	if err != nil {
		ctx.JSON(iris.Map{
			"code": config.StatusFailed,
			"msg":  err.Error(),
		})
		return
	}

	//开始生成sitemap
	err = provider.BuildSitemap()
	if err != nil {
		ctx.JSON(iris.Map{
			"code": config.StatusFailed,
			"msg":  err.Error(),
		})
		return
	}

	pluginSitemap := config.JsonData.PluginSitemap

	//由于sitemap的更新可能很频繁，因此sitemap的更新时间直接写入一个文件中
	pluginSitemap.UpdatedTime = provider.GetSitemapTime()
	// 写入Sitemap的url
	pluginSitemap.SitemapURL = config.JsonData.System.BaseUrl + "/sitemap." + pluginSitemap.Type

	provider.AddAdminLog(ctx, fmt.Sprintf("手动更新sitemap"))

	ctx.JSON(iris.Map{
		"code": config.StatusOK,
		"msg":  "Sitemap已更新",
		"data": pluginSitemap,
	})
}

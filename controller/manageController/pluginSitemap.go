package manageController

import (
	"github.com/kataras/iris/v12"
	"irisweb/config"
	"irisweb/provider"
	"irisweb/request"
)

func PluginSitemap(ctx iris.Context) {
	pluginSitemap := config.JsonData.PluginSitemap

	//由于sitemap的更新可能很频繁，因此sitemap的更新时间直接写入一个文件中
	pluginSitemap.UpdatedTime = provider.GetSitemapTime()

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

	config.JsonData.PluginSitemap.AutoBuild = req.AutoBuild

	err := config.WriteConfig()
	if err != nil {
		ctx.JSON(iris.Map{
			"code": config.StatusFailed,
			"msg":  err.Error(),
		})
		return
	}

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

	//先保存一次
	config.JsonData.PluginSitemap.AutoBuild = req.AutoBuild

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

	ctx.JSON(iris.Map{
		"code": config.StatusOK,
		"msg":  "Sitemap已更新",
		"data": pluginSitemap,
	})
}
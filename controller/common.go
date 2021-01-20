package controller

import (
	"github.com/kataras/iris/v12"
	"irisweb/config"
	"irisweb/provider"
	"net/url"
)

type WebInfo struct {
	Title       string `json:"title"`
	Keywords    string `json:"keywords"`
	Description string `json:"description"`
	NavBar      uint   `json:"nav_bar"`
}

var webInfo WebInfo

func NotFound(ctx iris.Context) {
	ctx.View("errors/404.html")
}

func InternalServerError(ctx iris.Context) {
	errMessage := ctx.Values().GetString("error")
	if errMessage == "" {
		errMessage = "(Unexpected) internal server error"
	}
	ctx.ViewData("errMessage", errMessage)
	ctx.View("errors/500.html")
}

func CheckCloseSite(ctx iris.Context) {
	if config.JsonData.System.SiteClose == 1 {
		closeTips := config.JsonData.System.SiteCloseTips
		ctx.ViewData("closeTips", closeTips)
		ctx.View("errors/close.html")
		return
	}

	ctx.Next()
}

func Common(ctx iris.Context) {
	//version
	ctx.ViewData("version", config.Version)
	//修正baseUrl
	if config.JsonData.System.BaseUrl == "" {
		urlPath, err := url.Parse(ctx.FullRequestURI())
		if err == nil {
			config.JsonData.System.BaseUrl = urlPath.Scheme + "://" + urlPath.Host
		}
	}
	//读取导航
	navList, _ := provider.GetNavList(true)
	ctx.ViewData("navList", navList)
	//核心配置
	ctx.ViewData("settingSystem", config.JsonData.System)
	//js code
	ctx.ViewData("pluginJsCode", config.JsonData.PluginPush.JsCode)
	if config.DB != nil {
		//全局分类
		categories, _ := provider.GetCategories()
		ctx.ViewData("categories", categories)
	}
	webInfo.NavBar = 0
	ctx.Next()
}

func Inspect(ctx iris.Context) {
	if config.DB == nil {
		ctx.Redirect("/install")
		return
	}

	ctx.Next()
}

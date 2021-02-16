package controller

import (
	"github.com/kataras/iris/v12"
	"irisweb/config"
	"irisweb/provider"
	"net/url"
	"strings"
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
	errMessage := ctx.Values().GetString("message")
	if errMessage == "" {
		errMessage = "(Unexpected) internal server error"
	}
	ctx.ViewData("errMessage", errMessage)
	ctx.View("errors/500.html")
}

func CheckCloseSite(ctx iris.Context) {
	if config.JsonData.System.SiteClose == 1 && !strings.HasPrefix(ctx.GetCurrentRoute().Path(), config.JsonData.System.AdminUri) {
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
	//核心配置
	ctx.ViewData("settingSystem", config.JsonData.System)
	//联系方式
	ctx.ViewData("settingContact", config.JsonData.Contact)
	//js code
	ctx.ViewData("pluginJsCode", config.JsonData.PluginPush.JsCode)
	if config.DB != nil {
		//读取导航
		navList, _ := provider.GetNavList(true)
		ctx.ViewData("navList", navList)
	}
	webInfo.NavBar = 0
	ctx.Next()
}

func Inspect(ctx iris.Context) {
	if config.DB == nil && ctx.GetCurrentRoute().Path() != "/install" {
		ctx.Redirect("/install")
		return
	}

	ctx.Next()
}

func ReRouteContext(ctx iris.Context) {
	params := ctx.Params().GetEntry("params").Value().(map[string]string)
	for i, v := range params {
		ctx.Params().Set(i, v)
	}

	switch params["match"] {
	case "article":
		ArticleDetail(ctx)
		return
	case "product":
		ProductDetail(ctx)
		return
	case "category":
		CategoryPage(ctx)
		return
	case "page":
		PagePage(ctx)
		return
	case "articleIndex":
		ArticleIndexPage(ctx)
		return
	case "productIndex":
		ProductIndexPage(ctx)
		return
	}

	//如果没有合适的路由，则报错
	NotFound(ctx)
}

package controller

import (
	"path/filepath"
	"strings"

	"github.com/kataras/iris/v12"
	"kandaoni.com/anqicms/provider"
	"kandaoni.com/anqicms/response"
)

func OrderIndexPage(ctx iris.Context) {
	currentSite := provider.CurrentSite(ctx)

	route := ctx.Params().Get("route")
	if route == "" {
		route = "index"
	}
	route = filepath.Clean(route)
	if !strings.HasSuffix(route, ".html") {
		route += ".html"
	}
	tpl := "order/" + route
	tpl = filepath.Clean(tpl)
	if !strings.HasPrefix(tpl, "order") {
		ctx.StatusCode(iris.StatusNotFound)
		return
	}
	tpl, ok := currentSite.TemplateExist(tpl)
	if !ok {
		ctx.StatusCode(iris.StatusNotFound)
		return
	}
	if webInfo, ok := ctx.Value("webInfo").(*response.WebInfo); ok {
		webInfo.Title = route
		webInfo.PageName = "order"
		ctx.ViewData("webInfo", webInfo)
	}

	ctx.ViewData("currentRoute", route)

	err := ctx.View(GetViewPath(ctx, tpl))
	if err != nil {
		ctx.StatusCode(404)
		ctx.Values().Set("message", err.Error())
	}
}

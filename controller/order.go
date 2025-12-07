package controller

import (
	"fmt"
	"github.com/kataras/iris/v12"
	"kandaoni.com/anqicms/provider"
	"kandaoni.com/anqicms/response"
)

func OrderIndexPage(ctx iris.Context) {
	currentSite := provider.CurrentSite(ctx)

	route := ctx.Params().GetStringDefault("route", "index")
	tpl := fmt.Sprintf("order/%s.html", route)
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

	ctx.View(tpl)
}

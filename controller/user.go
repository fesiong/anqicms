package controller

import (
	"fmt"
	"github.com/kataras/iris/v12"
	"kandaoni.com/anqicms/provider"
	"kandaoni.com/anqicms/response"
)

func UserPage(ctx iris.Context) {
	currentSite := provider.CurrentSite(ctx)
	id := ctx.Params().GetUintDefault("id", 0)

	user, err := currentSite.GetUserInfoById(id)
	if err != nil {
		NotFound(ctx)
		return
	}

	ctx.ViewData("user", user)

	if webInfo, ok := ctx.Value("webInfo").(*response.WebInfo); ok {
		webInfo.Title = user.UserName
		webInfo.NavBar = int64(user.Id)
		webInfo.PageName = "userDetail"
		webInfo.CanonicalUrl = currentSite.GetUrl("user", user, 0)
		ctx.ViewData("webInfo", webInfo)
	}
	
	tmpTpl := fmt.Sprintf("people/detail-%d.html", user.Id)
	tplName, ok := currentSite.TemplateExist(tmpTpl, fmt.Sprintf("people-%d.html", user.Id), "people/detail.html", "people_detail.html")
	if !ok {
		NotFound(ctx)
		return
	}
	err = ctx.View(GetViewPath(ctx, tplName))
	if err != nil {
		ctx.Values().Set("message", err.Error())
	}
}

package controller

import (
	"github.com/kataras/iris/v12"
	"kandaoni.com/anqicms/config"
	"kandaoni.com/anqicms/response"
	"strings"
)

func LoginPage(ctx iris.Context) {
	userId := ctx.Values().GetUintDefault("userId", 0)
	if userId > 0 {
		ctx.Redirect("/")
	}
	if webInfo, ok := ctx.Value("webInfo").(*response.WebInfo); ok {
		webInfo.Title = ctx.Tr("登录")
		ctx.ViewData("webInfo", webInfo)
	}
	err := ctx.View(GetViewPath(ctx, "login.html"))
	if err != nil {
		ctx.Values().Set("message", err.Error())
	}
}

func RegisterPage(ctx iris.Context) {
	if webInfo, ok := ctx.Value("webInfo").(*response.WebInfo); ok {
		webInfo.Title = ctx.Tr("注册")
		ctx.ViewData("webInfo", webInfo)
	}
	err := ctx.View(GetViewPath(ctx, "register.html"))
	if err != nil {
		ctx.Values().Set("message", err.Error())
	}
}

func AccountLogout(ctx iris.Context) {
	returnType := ctx.URLParamDefault("return", "html")
	ctx.RemoveCookie("token")
	if returnType == "json" {
		ctx.JSON(iris.Map{
			"code": config.StatusNoLogin,
			"msg":  ctx.Tr("已退出登录"),
		})
		return
	}

	ShowMessage(ctx, ctx.Tr("已退出登录"), []Button{{Name: ctx.Tr("首页"), Link: "/"}})
}

func AccountIndexPage(ctx iris.Context) {
	route := ctx.Params().Get("route")
	if route == "" {
		route = "index"
	}
	if !strings.HasSuffix(route, ".html") {
		route += ".html"
	}

	err := ctx.View(GetViewPath(ctx, "account/"+route))
	if err != nil {
		ctx.StatusCode(404)
		ctx.Values().Set("message", err.Error())
	}
}

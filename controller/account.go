package controller

import (
	"strings"

	"github.com/kataras/iris/v12"
	"kandaoni.com/anqicms/config"
	"kandaoni.com/anqicms/provider"
	"kandaoni.com/anqicms/response"
)

func LoginPage(ctx iris.Context) {
	currentSite := provider.CurrentSite(ctx)
	userId := ctx.Values().GetUintDefault("userId", 0)
	if userId > 0 {
		ctx.Redirect("/")
	}
	if webInfo, ok := ctx.Value("webInfo").(*response.WebInfo); ok {
		webInfo.Title = currentSite.TplTr("Login")
		ctx.ViewData("webInfo", webInfo)
	}
	err := ctx.View(GetViewPath(ctx, "login.html"))
	if err != nil {
		ctx.Values().Set("message", err.Error())
	}
}

func RegisterPage(ctx iris.Context) {
	currentSite := provider.CurrentSite(ctx)
	if webInfo, ok := ctx.Value("webInfo").(*response.WebInfo); ok {
		webInfo.Title = currentSite.TplTr("Register")
		ctx.ViewData("webInfo", webInfo)
	}
	err := ctx.View(GetViewPath(ctx, "register.html"))
	if err != nil {
		ctx.Values().Set("message", err.Error())
	}
}

func AccountLogout(ctx iris.Context) {
	currentSite := provider.CurrentSite(ctx)
	returnType := ctx.URLParamDefault("return", "html")
	ctx.RemoveCookie("token")
	if returnType == "json" {
		ctx.JSON(iris.Map{
			"code": config.StatusNoLogin,
			"msg":  currentSite.TplTr("LoggedOut"),
		})
		return
	}

	ShowMessage(ctx, currentSite.TplTr("LoggedOut"), []Button{{Name: currentSite.TplTr("Home"), Link: "/"}})
}

func AccountPasswordResetPage(ctx iris.Context) {
	currentSite := provider.CurrentSite(ctx)
	token := ctx.URLParam("token")
	code := ctx.URLParam("code")
	email := ctx.URLParam("email")
	if !currentSite.VerifyEmailFormat(email) || len(token) == 0 {
		ShowMessage(ctx, currentSite.TplTr("invalidParameter"), []Button{{Name: currentSite.TplTr("Home"), Link: "/"}})
		return
	}
	user, err := currentSite.GetUserInfoByEmail(email)
	if err != nil {
		ShowMessage(ctx, currentSite.TplTr("UserDoesNotExist"), []Button{{Name: currentSite.TplTr("Home"), Link: "/"}})
		return
	}

	ctx.ViewData("user", user)
	ctx.ViewData("token", token)
	ctx.ViewData("email", email)
	ctx.ViewData("code", code)

	err = ctx.View(GetViewPath(ctx, "account/password_reset.html"))
	if err != nil {
		ctx.StatusCode(404)
		ctx.Values().Set("message", err.Error())
	}
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

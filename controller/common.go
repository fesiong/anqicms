package controller

import (
	"github.com/kataras/iris/v12"
	"github.com/kataras/iris/v12/sessions"
	"irisweb/config"
	"irisweb/provider"
)

type WebInfo struct {
	Title       string `json:"title"`
	Keywords    string `json:"keywords"`
	Description string `json:"description"`
	NavBar      uint   `json:"nav_bar"`
}

var webInfo WebInfo

var sess = sessions.New(sessions.Config{Cookie: "irisweb"})

func NotFound(ctx iris.Context) {
	ctx.View("errors/404.html")
}

func InternalServerError(ctx iris.Context) {
	ctx.View("errors/500.html")
}

func Common(ctx iris.Context) {
	ctx.ViewData("SiteName", config.ServerConfig.SiteName)
	//检查登录状态
	session := sess.Start(ctx)
	hasLogin := session.GetBooleanDefault("hasLogin", false)
	ctx.Values().Set("hasLogin", hasLogin)
	ctx.ViewData("hasLogin", hasLogin)
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

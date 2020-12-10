package controller

import (
	"github.com/kataras/iris/v12"
	"irisweb/config"
	"irisweb/middleware"
	"irisweb/provider"
	"irisweb/request"
)

func AccountLogin(ctx iris.Context) {
	webInfo.Title = "登录"
	ctx.ViewData("webInfo", webInfo)
	ctx.View("account/login.html")
}

func AccountLoginForm(ctx iris.Context) {
	var req request.Admin
	if err := ctx.ReadForm(&req); err != nil {
		ctx.JSON(iris.Map{
			"code": config.StatusFailed,
			"msg":  err.Error(),
		})
		return
	}

	admin, err := provider.GetAdminByUserName(req.UserName)
	if err != nil {
		ctx.JSON(iris.Map{
			"code": config.StatusFailed,
			"msg":  err.Error(),
		})
		return
	}

	if !admin.CheckPassword(req.Password) {
		ctx.JSON(iris.Map{
			"code": config.StatusFailed,
			"msg":  "登录失败",
		})
		return
	}

	session := middleware.Sess.Start(ctx)
	session.Set("hasLogin", true)

	ctx.JSON(iris.Map{
		"code": 0,
		"msg":  "登录成功",
		"data": 1,
	})
}

func AccountLogout(ctx iris.Context) {
	session := middleware.Sess.Start(ctx)
	session.Delete("hasLogin")

	ctx.Redirect("/")
}

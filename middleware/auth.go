package middleware

import (
	"github.com/kataras/iris/v12"
	"github.com/kataras/iris/v12/sessions"
	"irisweb/config"
)

var Sess = sessions.New(sessions.Config{Cookie: "irisweb"})

func Auth(ctx iris.Context) {
	//检查登录状态
	session := Sess.Start(ctx)
	adminId := session.GetIntDefault("adminId", 0)
	ctx.Values().Set("adminId", adminId)
	ctx.ViewData("adminId", adminId)

	ctx.Next()
}

func ManageAuth(ctx iris.Context) {
	//后台管理强制要求登录
	adminId := ctx.Values().GetIntDefault("adminId", 0)
	if adminId == 0 {
		ctx.JSON(iris.Map{
			"code": config.StatusNoLogin,
			"msg":  "请登录",
		})
		return
	}

	ctx.Next()
}
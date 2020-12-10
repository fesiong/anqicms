package middleware

import (
	"github.com/kataras/iris/v12"
	"github.com/kataras/iris/v12/sessions"
)

var Sess = sessions.New(sessions.Config{Cookie: "irisweb"})

func Auth(ctx iris.Context) {
	//检查登录状态
	session := Sess.Start(ctx)
	hasLogin := session.GetBooleanDefault("hasLogin", false)
	ctx.Values().Set("hasLogin", hasLogin)
	ctx.ViewData("hasLogin", hasLogin)

	ctx.Next()
}
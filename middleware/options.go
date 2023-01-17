package middleware

import (
	"github.com/kataras/iris/v12"
)

func Cors(ctx iris.Context) {
	origin := ctx.GetHeader("Origin")
	if origin == "" {
		origin = ctx.GetHeader("Referer")
		if origin == "" {
			origin = "*"
		}
	}
	ctx.Header("Access-Control-Allow-Origin", origin)
	ctx.Header("Access-Control-Allow-Credentials", "true")
	ctx.Header("Access-Control-Expose-Headers", "Content-Disposition")
	ctx.Header("Access-Control-Allow-Methods", "GET,POST,PUT,DELETE,PATCH,OPTIONS")
	ctx.Header("Access-Control-Allow-Headers", "Content-Type, Api, Accept, Authorization, Version, Admin, Token, Key")
	if ctx.Request().Method == "OPTIONS" {
		ctx.StatusCode(204)
		return
	}
	ctx.Next()
}

package controller

import (
	"github.com/kataras/iris/v12"
	"irisweb/config"
	"irisweb/middleware"
	"irisweb/provider"
	"irisweb/request"
)

func AdminLogin(ctx iris.Context) {
	webInfo.Title = "登录"
	ctx.ViewData("webInfo", webInfo)
	ctx.View("admin/login.html")
	type ListNode struct {
		     Val int
		     Next *ListNode
	}
}

func AdminLoginForm(ctx iris.Context) {
	var req request.Admin
	if err := ctx.ReadJSON(&req); err != nil {
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
	session.Set("adminId", int(admin.Id))
	ctx.JSON(iris.Map{
		"code": config.StatusOK,
		"msg":  "登录成功",
		"data": admin,
	})
}

func AdminLogout(ctx iris.Context) {
	session := middleware.Sess.Start(ctx)
	session.Delete("adminId")

	ctx.Redirect("/")
}

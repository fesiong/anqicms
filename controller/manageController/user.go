package manageController

import (
	"github.com/kataras/iris/v12"
	"irisweb/config"
	"irisweb/controller"
	"irisweb/middleware"
	"irisweb/provider"
	"irisweb/request"
)

func UserLogin(ctx iris.Context) {
	//复用 AdminLoginForm
	controller.AdminLoginForm(ctx)
}

func UserLogout(ctx iris.Context) {
	session := middleware.Sess.Start(ctx)
	session.Delete("adminId")

	ctx.JSON(iris.Map{
		"code": config.StatusOK,
		"msg":  "已退出登录",
	})
}

func UserDetail(ctx iris.Context) {
	adminId := uint(ctx.Values().GetIntDefault("adminId", 0))

	admin, err := provider.GetAdminById(adminId)
	if err != nil {
		ctx.JSON(iris.Map{
			"code": config.StatusFailed,
			"msg":  "用户不存在",
		})
		return
	}

	ctx.JSON(iris.Map{
		"code": config.StatusOK,
		"msg":  "",
		"data": admin,
	})
}

func UserDetailForm(ctx iris.Context) {
	var req request.ChangeAdmin
	if err := ctx.ReadJSON(&req); err != nil {
		ctx.JSON(iris.Map{
			"code": config.StatusFailed,
			"msg":  err.Error(),
		})
		return
	}

	adminId := uint(ctx.Values().GetIntDefault("adminId", 0))

	admin, err := provider.GetAdminById(adminId)
	if err != nil {
		ctx.JSON(iris.Map{
			"code": config.StatusFailed,
			"msg":  "用户不存在",
		})
		return
	}

	if !admin.CheckPassword(req.OldPassword) {
		ctx.JSON(iris.Map{
			"code": config.StatusFailed,
			"msg":  "当前密码不正确",
		})
		return
	}

	admin.UserName = req.UserName
	admin.Password = admin.EncryptPassword(req.Password)
	err = admin.Save(config.DB)
	if err != nil {
		ctx.JSON(iris.Map{
			"code": config.StatusFailed,
			"msg":  "更新信息出错",
		})
		return
	}

	ctx.JSON(iris.Map{
		"code": config.StatusOK,
		"msg":  "管理员信息已更新",
	})
}
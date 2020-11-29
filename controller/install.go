package controller

import (
	"fmt"
	"github.com/kataras/iris/v12"
	"irisweb/config"
	"irisweb/provider"
	"irisweb/request"
)

func Install(ctx iris.Context) {
	if config.DB != nil {
		ctx.Redirect("/")
		return
	}

	webInfo.Title = "博客初始化"
	ctx.ViewData("webInfo", webInfo)
	ctx.View("install/index.html")
}

func InstallForm(ctx iris.Context) {
	if config.DB != nil {
		ctx.JSON(iris.Map{
			"code": config.StatusFailed,
			"msg":  "Failed",
		})
		return
	}
	var req request.Install
	if err := ctx.ReadForm(&req); err != nil {
		ctx.JSON(iris.Map{
			"code": config.StatusFailed,
			"msg":  err.Error(),
		})
		return
	}

	config.JsonData.DB.Database = req.Database
	config.JsonData.DB.User = req.User
	config.JsonData.DB.Password = req.Password
	config.JsonData.DB.Host = req.Host
	config.JsonData.DB.Port = req.Port

	err := config.InitDB(&config.JsonData.DB)
	if err != nil {
		ctx.JSON(iris.Map{
			"code": config.StatusFailed,
			"msg":  err.Error(),
		})
		return
	}

	err = config.WriteConfig()
	if err != nil {
		ctx.JSON(iris.Map{
			"code": config.StatusFailed,
			"msg":  err.Error(),
		})
		return
	}

	//创建管理员
	err = provider.InitAdmin(req.AdminUser, req.AdminPassword)
	if err != nil {
		ctx.JSON(iris.Map{
			"code": config.StatusFailed,
			"msg":  err.Error(),
		})
		return
	}

	ctx.JSON(iris.Map{
		"code": config.StatusOK,
		"msg":  fmt.Sprintf("%s初始化成功", config.ServerConfig.SiteName),
	})
}

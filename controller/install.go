package controller

import (
	"fmt"
	"github.com/kataras/iris/v12"
	"irisweb/config"
	"irisweb/model"
	"irisweb/provider"
	"irisweb/request"
	"net/url"
	"strings"
)

func Install(ctx iris.Context) {
	if config.DB != nil {
		ctx.Redirect("/")
		return
	}

	baseUrl := ""
	urlPath, err := url.Parse(ctx.FullRequestURI())
	if err == nil {
		baseUrl = urlPath.Scheme + "://" + urlPath.Host
	}

	ctx.ViewData("baseUrl", baseUrl)

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
	if err := ctx.ReadJSON(&req); err != nil {
		ctx.JSON(iris.Map{
			"code": config.StatusFailed,
			"msg":  err.Error(),
		})
		return
	}
	//更新网站配置
	req.BaseUrl = strings.TrimRight(req.BaseUrl, "/")
	config.JsonData.System.BaseUrl = req.BaseUrl

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
	//自动迁移数据库
	err = model.AutoMigrateDB(config.DB)
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

package controller

import (
	"fmt"
	"github.com/kataras/iris/v12"
	"kandaoni.com/anqicms/config"
	"kandaoni.com/anqicms/dao"
	"kandaoni.com/anqicms/provider"
	"kandaoni.com/anqicms/request"
	"net/url"
	"strings"
)

func Install(ctx iris.Context) {
	if dao.DB != nil {
		ctx.Redirect("/")
		return
	}

	baseUrl := ""
	urlPath, err := url.Parse(ctx.FullRequestURI())
	if err == nil {
		baseUrl = urlPath.Scheme + "://" + urlPath.Host
	}

	ctx.ViewData("baseUrl", baseUrl)

	webInfo.Title = config.Lang("安企内容管理系统安装")
	ctx.ViewData("webInfo", webInfo)
	err = ctx.View("install/index.html")
	if err != nil {
		ctx.Values().Set("message", err.Error())
	}
}

func InstallForm(ctx iris.Context) {
	if dao.DB != nil {
		ctx.JSON(iris.Map{
			"code": config.StatusFailed,
			"msg":  "Failed",
		})
		return
	}
	var req request.Install
	// 采用post提交
	req.Database = ctx.PostValueTrim("database")
	req.User = ctx.PostValueTrim("user")
	req.Password = ctx.PostValueTrim("password")
	req.Host = ctx.PostValueTrim("host")
	req.Port = ctx.PostValueIntDefault("port", 3306)
	req.AdminUser = ctx.PostValueTrim("admin_user")
	req.AdminPassword = ctx.PostValueTrim("admin_password")
	req.BaseUrl = ctx.PostValueTrim("base_url")

	if len(req.AdminPassword) < 6 {
		ctx.JSON(iris.Map{
			"code": config.StatusFailed,
			"msg":  "请填写6位以上的管理员密码",
		})
		return
	}

	//更新网站配置
	req.BaseUrl = strings.TrimRight(req.BaseUrl, "/")
	config.JsonData.System.BaseUrl = req.BaseUrl

	config.JsonData.Mysql.Database = req.Database
	config.JsonData.Mysql.User = req.User
	config.JsonData.Mysql.Password = req.Password
	config.JsonData.Mysql.Host = req.Host
	config.JsonData.Mysql.Port = req.Port

	err := dao.InitDB()
	if err != nil {
		ctx.JSON(iris.Map{
			"code": config.StatusFailed,
			"msg":  err.Error(),
		})
		return
	}
	//自动迁移数据库
	err = dao.AutoMigrateDB(dao.DB)
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
	err = provider.InitAdmin(req.AdminUser, req.AdminPassword, true)
	if err != nil {
		ctx.JSON(iris.Map{
			"code": config.StatusFailed,
			"msg":  err.Error(),
		})
		return
	}

	ctx.JSON(iris.Map{
		"code": config.StatusOK,
		"msg":  fmt.Sprintf(config.Lang("%s安装成功"), config.ServerConfig.SiteName),
	})
}

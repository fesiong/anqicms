package route

import (
	"fmt"
	"github.com/kataras/iris/v12"
	"irisweb/config"
	"irisweb/controller"
)

func Register(app *iris.Application) {
	app.OnErrorCode(iris.StatusNotFound, controller.NotFound)
	app.OnErrorCode(iris.StatusInternalServerError, controller.InternalServerError)

	app.Use(controller.Common)
	app.HandleDir("/", fmt.Sprintf("%spublic", config.ExecPath))
	app.Get("/", controller.Inspect, controller.CheckCloseSite, controller.IndexPage)

	app.Get("/install", controller.Install)
	app.Post("/install", controller.InstallForm)

	article := app.Party("/article", controller.Inspect, controller.CheckCloseSite)
	{
		article.Get("/{id:uint}", controller.ArticleDetail)
		article.Get("/publish", controller.ArticlePublish)
		article.Post("/publish", controller.ArticlePublishForm)
	}

	category := app.Party("/category", controller.Inspect, controller.CheckCloseSite)
	{
		category.Get("/{id:uint}", controller.CategoryPage)
	}

	attachment := app.Party("/attachment", controller.Inspect, controller.CheckCloseSite)
	{
		attachment.Post("/upload", controller.AttachmentUpload)
	}

	comment := app.Party("/comment", controller.Inspect, controller.CheckCloseSite)
	{
		comment.Post("/publish", controller.CommentPublish)
		comment.Post("/praise", controller.CommentPraise)
		comment.Get("/article/{id:uint}", controller.ArticleCommentList)
	}

	admin := app.Party("/admin", controller.Inspect)
	{
		admin.Get("/login", controller.AdminLogin)
		admin.Post("/login", controller.AdminLoginForm)
		admin.Get("/logout", controller.AdminLogout)
	}

	//后台管理路由相关
	manageRoute(app)
}

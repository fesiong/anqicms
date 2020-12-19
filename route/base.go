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
	app.Get("/", controller.Inspect, controller.IndexPage)

	app.Get("/install", controller.Install)
	app.Post("/install", controller.InstallForm)

	article := app.Party("/article", controller.Inspect)
	{
		article.Get("/{id:uint}", controller.ArticleDetail)
		article.Get("/publish", controller.ArticlePublish)
		article.Post("/publish", controller.ArticlePublishForm)
	}

	attachment := app.Party("/attachment", controller.Inspect)
	{
		attachment.Post("/upload", controller.AttachmentUpload)
	}

	admin := app.Party("/admin", controller.Inspect)
	{
		admin.Get("/login", controller.AdminLogin)
		admin.Post("/login", controller.AdminLoginForm)
		admin.Get("/logout", controller.AdminLogout)
	}
}

package route

import (
	"fmt"
	"github.com/kataras/iris/v12"
	"irisweb/config"
	"irisweb/controller/manageController"
	"irisweb/middleware"
)

func manageRoute(app *iris.Application) {
	manage := app.Party(config.JsonData.System.AdminUri)
	{
		manage.HandleDir("/", fmt.Sprintf("%smanage", config.ExecPath))
		manage.Post("/user/login", manageController.UserLogin)
		manage.Get("/version", manageController.Version)
		manage.Get("/statistics", manageController.Statistics)

		user := manage.Party("/user", middleware.ManageAuth)
		{
			user.Get("/detail", manageController.UserDetail)
			user.Post("/detail", manageController.UserDetailForm)
			user.Post("/logout", manageController.UserLogout)
		}

		setting := manage.Party("/setting", middleware.ManageAuth)
		{
			setting.Get("/system", manageController.SettingSystem)
			setting.Get("/content", manageController.SettingContent)
			setting.Get("/index", manageController.SettingIndex)
			setting.Get("/nav", manageController.SettingNav)
			setting.Get("/contact", manageController.SettingContact)

			setting.Post("/system", manageController.SettingSystemForm)
			setting.Post("/content", manageController.SettingContentForm)
			setting.Post("/thumb/rebuild", manageController.SettingThumbRebuild)
			setting.Post("/index", manageController.SettingIndexForm)
			setting.Post("/nav", manageController.SettingNavForm)
			setting.Post("/nav/delete", manageController.SettingNavDelete)
			setting.Post("/contact", manageController.SettingContactForm)

		}

		attachment := manage.Party("/attachment", middleware.ManageAuth)
		{
			attachment.Get("/list", manageController.AttachmentList)
			attachment.Post("/upload", manageController.AttachmentUpload)
			attachment.Post("/delete", manageController.AttachmentDelete)
		}

		category := manage.Party("/category", middleware.ManageAuth)
		{
			category.Get("/list", manageController.CategoryList)
			category.Get("/detail", manageController.CategoryDetail)
			category.Post("/detail", manageController.CategoryDetailForm)
			category.Post("/delete", manageController.CategoryDelete)
		}

		article := manage.Party("/article", middleware.ManageAuth)
		{
			article.Get("/list", manageController.ArticleList)
			article.Get("/detail", manageController.ArticleDetail)
			article.Post("/detail", manageController.ArticleDetailForm)
			article.Post("/delete", manageController.ArticleDelete)
		}

		product := manage.Party("/product", middleware.ManageAuth)
		{
			product.Get("/list", manageController.ProductList)
			product.Get("/detail", manageController.ProductDetail)
			product.Post("/detail", manageController.ProductDetailForm)
			product.Post("/delete", manageController.ProductDelete)
		}

		plugin := manage.Party("/plugin", middleware.ManageAuth)
		{
			plugin.Get("/push", manageController.PluginPush)
			plugin.Post("/push", manageController.PluginPushForm)

			plugin.Get("/robots", manageController.PluginRobots)
			plugin.Post("/robots", manageController.PluginRobotsForm)

			plugin.Get("/sitemap", manageController.PluginSitemap)
			plugin.Post("/sitemap", manageController.PluginSitemapForm)
			plugin.Post("/sitemap/build", manageController.PluginSitemapBuild)

			plugin.Get("/rewrite", manageController.PluginRewrite)
			plugin.Post("/rewrite", manageController.PluginRewriteForm)

			link := plugin.Party("/link")
			{
				link.Get("/list", manageController.PluginLinkList)
				link.Post("/detail", manageController.PluginLinkDetailForm)
				link.Post("/delete", manageController.PluginLinkDelete)
				link.Post("/check", manageController.PluginLinkCheck)
			}

			comment := plugin.Party("/comment")
			{
				comment.Get("/list", manageController.PluginCommentList)
				comment.Get("/detail", manageController.PluginCommentDetail)
				comment.Post("/detail", manageController.PluginCommentDetailForm)
				comment.Post("/delete", manageController.PluginCommentDelete)
				comment.Post("/check", manageController.PluginCommentCheck)
			}

			anchor := plugin.Party("/anchor")
			{
				anchor.Get("/list", manageController.PluginAnchorList)
				anchor.Get("/detail", manageController.PluginAnchorDetail)
				anchor.Post("/detail", manageController.PluginAnchorDetailForm)
				anchor.Post("/replace", manageController.PluginAnchorReplace)
				anchor.Post("/delete", manageController.PluginAnchorDelete)
				anchor.Post("/export", manageController.PluginAnchorExport)
				anchor.Post("/import", manageController.PluginAnchorImport)
				anchor.Get("/setting", manageController.PluginAnchorSetting)
				anchor.Post("/setting", manageController.PluginAnchorSettingForm)
			}

			guestbook := plugin.Party("/guestbook")
			{
				guestbook.Get("/list", manageController.PluginGuestbookList)
				guestbook.Post("/delete", manageController.PluginGuestbookDelete)
				guestbook.Post("/export", manageController.PluginGuestbookExport)
				guestbook.Get("/setting", manageController.PluginGuestbookSetting)
				guestbook.Post("/setting", manageController.PluginGuestbookSettingForm)
			}

			keyword := plugin.Party("/keyword")
			{
				keyword.Get("/list", manageController.PluginKeywordList)
				keyword.Post("/detail", manageController.PluginKeywordDetailForm)
				keyword.Post("/delete", manageController.PluginKeywordDelete)
				keyword.Post("/export", manageController.PluginKeywordExport)
				keyword.Post("/import", manageController.PluginKeywordImport)
			}

			fileUpload := plugin.Party("/fileupload")
			{
				fileUpload.Get("/list", manageController.PluginFileUploadList)
				fileUpload.Post("/upload", manageController.PluginFileUploadUpload)
				fileUpload.Post("/delete", manageController.PluginFileUploadDelete)
			}
		}
	}
}
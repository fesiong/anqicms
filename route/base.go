package route

import (
	"fmt"
	"github.com/kataras/iris/v12"
	"kandaoni.com/anqicms/config"
	"kandaoni.com/anqicms/controller"
	"kandaoni.com/anqicms/middleware"
	"kandaoni.com/anqicms/provider"
	"regexp"
	"strings"
)

func Register(app *iris.Application) {
	//注册macros
	resisterMacros(app)
	//设置错误
	app.OnErrorCode(iris.StatusNotFound, controller.NotFound)
	app.OnErrorCode(iris.StatusInternalServerError, controller.InternalServerError)
	app.Use(controller.Inspect)
	app.Use(controller.CheckTemplateType)
	app.Use(controller.CheckCloseSite)
	app.Use(controller.Common)
	//由于使用了自定义路由，它不能同时解析两条到一起，因此这里不能启用fileserver，需要用nginx设置，有没研究出方法了再改进
	//app.HandleDir("/", fmt.Sprintf("%spublic", config.ExecPath))
	app.Get("/{params:rewrite}", middleware.FrontendCheck, middleware.Check301, controller.ReRouteContext)
	app.Get("/", middleware.FrontendCheck, controller.LogAccess, controller.IndexPage)

	app.Get("/install", middleware.FrontendCheck, controller.Install)
	app.Post("/install", middleware.FrontendCheck, controller.InstallForm)

	attachment := app.Party("/attachment", middleware.FrontendCheck)
	{
		attachment.Post("/upload", controller.AttachmentUpload)
	}

	comment := app.Party("/comment", middleware.FrontendCheck, controller.LogAccess)
	{
		comment.Post("/publish", controller.CommentPublish)
		comment.Post("/praise", controller.CommentPraise)
		comment.Get("/{id:uint}", controller.CommentList)
	}

	app.Get("/guestbook.html", middleware.FrontendCheck, controller.LogAccess, controller.GuestbookPage)
	app.Post("/guestbook.html", middleware.FrontendCheck, controller.GuestbookForm)

	api := app.Party("/api", middleware.FrontendCheck)
	{
		api.Get("/captcha", controller.GenerateCaptcha)

		// 内容导入API
		api.Post("/import/archive", controller.VerifyApiToken, controller.ApiImportArchive)
		api.Post("/import/categories", controller.VerifyApiToken, controller.ApiImportGetCategories)
		api.Post("/friendlink/create", controller.VerifyApiToken, controller.ApiImportCreateFriendLink)
		api.Post("/friendlink/delete", controller.VerifyApiToken, controller.ApiImportDeleteFriendLink)
	}

	//后台管理路由相关
	manageRoute(app)
}

func resisterMacros(app *iris.Application) {
	//注册rewrite
	app.Macros().Register("rewrite", "", false, true, func(paramValue string) (interface{}, bool) {
		//这里总共有6条正则规则，需要逐一匹配
		// 由于用户可能会采用相同的配置，因此这里需要尝试多次读取
		matchMap := map[string]string{}
		// 静态资源直接返回
		if strings.HasPrefix(paramValue, "uploads/") ||
			strings.HasPrefix(paramValue, "static/") ||
			strings.HasPrefix(paramValue, "system/") {
			return matchMap, true
		}
		// 如果匹配到固化链接，则直接返回
		archiveId := provider.GetFixedLinkFromCache("/" + paramValue)
		if archiveId > 0 {
			matchMap["match"] = "archive"
			matchMap["id"] = fmt.Sprintf("%d", archiveId)
			return matchMap, true
		}
		// 搜索
		if paramValue == "search" {
			matchMap["match"] = "search"
			return matchMap, true
		}
		rewritePattern := config.ParsePatten(false)
		//archivePage
		reg := regexp.MustCompile(rewritePattern.ArchiveIndexRule)
		match := reg.FindStringSubmatch(paramValue)
		if len(match) > 0 {
			matchMap["match"] = "archiveIndex"
			for i, v := range match {
				key := rewritePattern.ArchiveIndexTags[i]
				if i == 0 {
					key = "route"
				}
				matchMap[key] = v
			}
			// 这个规则可能与下面的冲突，因此检查一遍
			module := provider.GetModuleFromCacheByToken(matchMap["module"])
			if module != nil {
				return matchMap, true
			}
			matchMap = map[string]string{}
		}
		//category
		reg = regexp.MustCompile(rewritePattern.CategoryRule)
		match = reg.FindStringSubmatch(paramValue)
		if len(match) > 1 {
			matchMap["match"] = "category"
			for i, v := range match {
				key := rewritePattern.CategoryTags[i]
				if i == 0 {
					key = "route"
				}
				matchMap[key] = v
			}
			if matchMap["filename"] != "" || matchMap["catname"] != "" {
				if matchMap["catname"] != "" {
					matchMap["filename"] = matchMap["catname"]
				}
				// 这个规则可能与下面的冲突，因此检查一遍
				category := provider.GetCategoryFromCacheByToken(matchMap["filename"])
				if category != nil {
					return matchMap, true
				}
			} else {
				return matchMap, true
			}
			matchMap = map[string]string{}
		}
		//page
		reg = regexp.MustCompile(rewritePattern.PageRule)
		match = reg.FindStringSubmatch(paramValue)
		if len(match) > 1 {
			matchMap["match"] = "page"
			for i, v := range match {
				key := rewritePattern.PageTags[i]
				if i == 0 {
					key = "route"
				}
				matchMap[key] = v
			}
			if matchMap["filename"] != "" {
				// 这个规则可能与下面的冲突，因此检查一遍
				category := provider.GetCategoryFromCacheByToken(matchMap["filename"])
				if category != nil {
					return matchMap, true
				}
			} else {
				return matchMap, true
			}
			matchMap = map[string]string{}
		}
		//最后archive
		reg = regexp.MustCompile(rewritePattern.ArchiveRule)
		match = reg.FindStringSubmatch(paramValue)
		if len(match) > 1 {
			matchMap["match"] = "archive"
			for i, v := range match {
				key := rewritePattern.ArchiveTags[i]
				if i == 0 {
					key = "route"
				}
				matchMap[key] = v
			}
			return matchMap, true
		}
		//tagIndex
		reg = regexp.MustCompile(rewritePattern.TagIndexRule)
		match = reg.FindStringSubmatch(paramValue)
		if len(match) > 1 {
			matchMap["match"] = "tagIndex"
			for i, v := range match {
				key := rewritePattern.TagIndexTags[i]
				if i == 0 {
					key = "route"
				}
				matchMap[key] = v
			}
			return matchMap, true
		}
		//tag
		reg = regexp.MustCompile(rewritePattern.TagRule)
		match = reg.FindStringSubmatch(paramValue)
		if len(match) > 1 {
			matchMap["match"] = "tag"
			for i, v := range match {
				key := rewritePattern.TagTags[i]
				if i == 0 {
					key = "route"
				}
				matchMap[key] = v
			}
			return matchMap, true
		}

		//不存在，定义到notfound
		matchMap["match"] = "notfound"
		return matchMap, true
	})
}
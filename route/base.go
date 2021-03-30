package route

import (
	"fmt"
	"github.com/kataras/iris/v12"
	"irisweb/config"
	"irisweb/controller"
	"irisweb/middleware"
	"irisweb/model"
	"irisweb/provider"
	"regexp"
	"strconv"
)

func Register(app *iris.Application) {
	//注册macros
	resisterMacros(app)
	//设置错误
	app.OnErrorCode(iris.StatusNotFound, controller.NotFound)
	app.OnErrorCode(iris.StatusInternalServerError, controller.InternalServerError)
	app.Options("*", middleware.Cors)
	app.Use(controller.Inspect)
	app.Use(controller.CheckTemplateType)
	app.Use(controller.CheckCloseSite)
	app.Use(controller.Common)
	app.Use(controller.LogAccess)
	//由于使用了自定义路由，它不能同时解析两条到一起，因此这里不能启用fileserver，需要用nginx设置，有没研究出方法了再改进
	//app.HandleDir("/", fmt.Sprintf("%spublic", config.ExecPath))
	app.Get("/{params:rewrite}", controller.ReRouteContext)
	app.Get("/", controller.IndexPage)

	app.Get("/install", controller.Install)
	app.Post("/install", controller.InstallForm)

	attachment := app.Party("/attachment")
	{
		attachment.Post("/upload", controller.AttachmentUpload)
	}

	comment := app.Party("/comment")
	{
		comment.Post("/publish", controller.CommentPublish)
		comment.Post("/praise", controller.CommentPraise)
		comment.Get("/article/{id:uint}", controller.ArticleCommentList)
	}

	app.Get("/guestbook.html", controller.GuestbookPage)
	app.Post("/guestbook.html", controller.GuestbookForm)

	//后台管理路由相关
	manageRoute(app)
}

func resisterMacros(app *iris.Application) {
	rewritePattern := config.ParsePatten()
	fmt.Println(rewritePattern.Parsed)
	//注册rewrite
	app.Macros().Register("rewrite", "", false, true, func(paramValue string) (interface{}, bool) {
		//这里总共有4条正则规则，需要逐一匹配
		matchMap := map[string]string{}
		//articlePage
		reg := regexp.MustCompile(rewritePattern.ArticleIndexRule)
		match := reg.FindStringSubmatch(paramValue)
		if len(match) > 0 {
			matchMap["match"] = "articleIndex"
			return matchMap, true
		}
		//productPage
		reg = regexp.MustCompile(rewritePattern.ProductIndexRule)
		match = reg.FindStringSubmatch(paramValue)
		if len(match) > 0 {
			matchMap["match"] = "productIndex"
			return matchMap, true
		}
		//article
		reg = regexp.MustCompile(rewritePattern.ArticleRule)
		match = reg.FindStringSubmatch(paramValue)
		if len(match) > 1 {
			matchMap["match"] = "article"
			for i, v := range match {
				key := rewritePattern.ArticleTags[i]
				if i == 0 {
					key = "route"
				}
				matchMap[key] = v
			}
			//如果article和product设置了同一个规则，是无法判断它实际属于什么的，如果设置了catid或catname，那么还能挽救一下
			if matchMap["catname"] != "" {
				//存在catname
				category, err := provider.GetCategoryByUrlToken(matchMap["catname"])
				if err == nil && category.Type == model.CategoryTypeProduct {
					//好家伙，把product的正则当做article了，修正。
					matchMap["match"] = "product"
				}
			} else if matchMap["catid"] != "" {
				//存在catid
				categoryId, _ := strconv.Atoi(matchMap["catid"])
				category, err := provider.GetCategoryById(uint(categoryId))
				if err == nil && category.Type == model.CategoryTypeProduct {
					//好家伙，把product的正则当做article了，修正。
					matchMap["match"] = "product"
				}
			}
			return matchMap, true
		}
		//product
		reg = regexp.MustCompile(rewritePattern.ProductRule)
		match = reg.FindStringSubmatch(paramValue)
		if len(match) > 1 {
			matchMap["match"] = "product"
			for i, v := range match {
				key := rewritePattern.ProductTags[i]
				if i == 0 {
					key = "route"
				}
				matchMap[key] = v
			}
			return matchMap, true
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
			return matchMap, true
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
			return matchMap, true
		}

		return nil, false
	})
}
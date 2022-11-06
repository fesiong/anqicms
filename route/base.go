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
	app.Get("/{params:rewrite}", middleware.FrontendCheck, middleware.Check301, middleware.ParseUserToken, controller.ReRouteContext)
	app.Get("/", middleware.FrontendCheck, controller.LogAccess, middleware.ParseUserToken, controller.IndexPage)

	app.Get("/install", middleware.FrontendCheck, controller.Install)
	app.Post("/install", middleware.FrontendCheck, controller.InstallForm)

	attachment := app.Party("/attachment", middleware.FrontendCheck, middleware.ParseUserToken)
	{
		attachment.Post("/upload", controller.AttachmentUpload)
	}

	comment := app.Party("/comment", middleware.FrontendCheck, controller.LogAccess, middleware.ParseUserToken)
	{
		comment.Post("/publish", controller.CommentPublish)
		comment.Post("/praise", controller.CommentPraise)
		comment.Get("/{id:uint}", controller.CommentList)
	}

	app.Get("/guestbook.html", middleware.FrontendCheck, controller.LogAccess, middleware.ParseUserToken, controller.GuestbookPage)
	app.Post("/guestbook.html", middleware.FrontendCheck, middleware.ParseUserToken, controller.GuestbookForm)

	// login and register
	app.Get("/login", middleware.FrontendCheck, controller.LoginPage)
	app.Get("/register", middleware.FrontendCheck, controller.RegisterPage)
	app.Get("/logout", middleware.FrontendCheck, middleware.ParseUserToken, controller.AccountLogout)
	// account party
	app.Get("/account/{route:path}", middleware.FrontendCheck, middleware.ParseUserToken, controller.AccountIndexPage)

	api := app.Party("/api", middleware.FrontendCheck, middleware.ParseUserToken)
	{
		api.Get("/captcha", controller.GenerateCaptcha)
		// WeChat official account api
		api.Get("/wechat/auth", controller.WechatAuthApi)
		api.Get("/wechat", controller.WechatApi)
		api.Post("/wechat", controller.WechatApi)

		// 内容导入API
		api.Post("/import/archive", controller.VerifyApiToken, controller.ApiImportArchive)
		api.Get("/import/categories", controller.VerifyApiToken, controller.ApiImportGetCategories)
		api.Post("/import/categories", controller.VerifyApiToken, controller.ApiImportGetCategories)
		// 友链API
		api.Post("/friendlink/create", controller.VerifyApiLinkToken, controller.ApiImportCreateFriendLink)
		api.Post("/friendlink/delete", controller.VerifyApiLinkToken, controller.ApiImportDeleteFriendLink)
		api.Get("/friendlink/list", controller.VerifyApiLinkToken, controller.ApiImportGetFriendLinks)
		api.Post("/friendlink/list", controller.VerifyApiLinkToken, controller.ApiImportGetFriendLinks)
		api.Get("/friendlink/check", controller.VerifyApiLinkToken, controller.ApiImportCheckFriendLink)
		api.Post("/friendlink/check", controller.VerifyApiLinkToken, controller.ApiImportCheckFriendLink)
		// 前端api
		api.Post("/login", controller.ApiLogin)
		api.Post("/register", controller.ApiRegister)
		api.Get("/user/detail", middleware.UserAuth, controller.ApiGetUserDetail)
		api.Post("/user/detail", middleware.UserAuth, controller.ApiUpdateUserDetail)
		api.Get("/user/groups", middleware.UserAuth, controller.ApiGetUserGroups)
		api.Get("/user/group/detail", middleware.UserAuth, controller.ApiGetUserGroupDetail)
		api.Post("/user/password", middleware.UserAuth, controller.ApiUpdateUserPassword)
		api.Get("/orders", middleware.UserAuth, controller.ApiGetOrders)
		api.Post("/order/create", middleware.UserAuth, controller.ApiCreateOrder)
		api.Get("/order/address", middleware.UserAuth, controller.ApiGetOrderAddress)
		api.Post("/order/address", middleware.UserAuth, controller.ApiSaveOrderAddress)
		api.Get("/order/detail", middleware.UserAuth, controller.ApiGetOrderDetail)
		api.Post("/order/cancel", middleware.UserAuth, controller.ApiCancelOrder)
		api.Post("/order/refund", middleware.UserAuth, controller.ApiApplyRefundOrder)
		api.Post("/order/finish", middleware.UserAuth, controller.ApiFinishedOrder)
		api.Post("/order/payment", middleware.UserAuth, controller.ApiCreateOrderPayment)
		api.Post("/weapp/qrcode", middleware.UserAuth, controller.ApiCreateWeappQrcode)
		//检查支付情况
		api.Get("/archive/order/check", controller.ApiArchiveOrderCheck)
		api.Get("/payment/check", controller.ApiPaymentCheck)
		api.Get("/retailer/info", controller.ApiGetRetailerInfo)
		api.Get("/retailer/statistics", middleware.UserAuth, controller.ApiGetRetailerStatistics)
		api.Post("/retailer/update", middleware.UserAuth, controller.ApiUpdateRetailerInfo)
		api.Get("/retailer/orders", middleware.UserAuth, controller.ApiGetRetailerOrders)
		api.Get("/retailer/withdraw", middleware.UserAuth, controller.ApiGetRetailerWithdraws)
		api.Post("/retailer/withdraw", middleware.UserAuth, controller.ApiRetailerWithdraw)
		api.Get("/retailer/members", middleware.UserAuth, controller.ApiGetRetailerMembers)
		api.Get("/retailer/commissions", middleware.UserAuth, controller.ApiGetRetailerCommissions)
		// 发布文档
		api.Post("/archive/publish", middleware.UserAuth, controller.ApiArchivePublish)
		// common api
		api.Get("/archive/detail", controller.ApiArchiveDetail)
		api.Get("/archive/filters", controller.ApiArchiveFilters)
		api.Get("/archive/list", controller.ApiArchiveList)
		api.Get("/archive/params", controller.ApiArchiveParams)
		api.Get("/category/detail", controller.ApiCategoryDetail)
		api.Get("/category/list", controller.ApiCategoryList)
		api.Get("/comment/list", controller.ApiCommentList)
		api.Get("/setting/contact", controller.ApiContact)
		api.Get("/setting/system", controller.ApiSystem)
		api.Get("/guestbook/fields", controller.ApiGuestbook)
		api.Get("/friendlink/list", controller.ApiLinkList)
		api.Get("/nav/list", controller.ApiNavList)
		api.Get("/archive/next", controller.ApiNextArchive)
		api.Get("/archive/prev", controller.ApiPrevArchive)
		api.Get("/page/detail", controller.ApiPageDetail)
		api.Get("/page/list", controller.ApiPageList)
		api.Get("/tag/detail", controller.ApiTagDetail)
		api.Get("/tag/data/list", controller.ApiTagDataList)
		api.Get("/tag/list", controller.ApiTagList)
		api.Post("/attachment/upload", controller.ApiAttachmentUpload)
		api.Post("/comment/publish", controller.ApiCommentPublish)
		api.Post("/comment/praise", controller.ApiCommentPraise)
		api.Post("/guestbook.html", controller.ApiGuestbookForm)
	}

	notify := app.Party("/notify")
	{
		notify.Get("/weapp/msg", controller.NotifyWeappMsg)
		notify.Post("/wechat/pay", controller.NotifyWechatPay)
		notify.Post("/alipay/pay", controller.NotifyAlipay)
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
				if category != nil && category.Type == config.CategoryTypePage {
					return matchMap, true
				}
			} else {
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
			if matchMap["catname"] != "" {
				matchMap["filename"] = matchMap["catname"]
			}
			if matchMap["multicatname"] != "" {
				chunkCatNames := strings.Split(matchMap["multicatname"], "/")
				matchMap["filename"] = chunkCatNames[len(chunkCatNames)-1]
			}
			if matchMap["module"] != "" {
				// 需要先验证是否是module
				module := provider.GetModuleFromCacheByToken(matchMap["module"])
				if module != nil {
					if matchMap["filename"] != "" {
						// 这个规则可能与下面的冲突，因此检查一遍
						category := provider.GetCategoryFromCacheByToken(matchMap["filename"])
						if category != nil && category.Type != config.CategoryTypePage {
							return matchMap, true
						}
					} else {
						return matchMap, true
					}
				}
			} else {
				if matchMap["filename"] != "" {
					// 这个规则可能与下面的冲突，因此检查一遍
					category := provider.GetCategoryFromCacheByToken(matchMap["filename"])
					if category != nil && category.Type != config.CategoryTypePage {
						return matchMap, true
					}
				} else {
					return matchMap, true
				}
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
			if matchMap["module"] != "" {
				// 需要先验证是否是module
				module := provider.GetModuleFromCacheByToken(matchMap["module"])
				if module != nil {
					return matchMap, true
				}
			} else {
				return matchMap, true
			}
		}

		//不存在，定义到notfound
		matchMap["match"] = "notfound"
		return matchMap, true
	})
}

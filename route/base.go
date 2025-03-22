package route

import (
	"embed"
	"github.com/kataras/iris/v12"
	"kandaoni.com/anqicms/controller"
	"kandaoni.com/anqicms/middleware"
)

func Register(app *iris.Application, systemFiles embed.FS) {
	//注册macros
	//设置错误
	app.Use(controller.Inspect)
	app.OnErrorCode(iris.StatusNotFound, controller.NotFound)
	app.OnErrorCode(iris.StatusInternalServerError, controller.InternalServerError)
	app.Use(controller.CheckTemplateType)
	app.Use(controller.Common)
	//由于使用了自定义路由，它不能同时解析两条到一起，因此这里不能启用fileserver，需要用nginx设置，有没研究出方法了再改进
	//app.HandleDir("/", fmt.Sprintf("%spublic", config.ExecPath)
	app.Get("/{path:path}", middleware.Check301, middleware.ParseUserToken, middleware.HandlerTimeout, controller.ReRouteContext)

	app.Get("/install", controller.Install)
	app.Post("/install", controller.InstallForm)
	app.HandleMany(iris.MethodPost, "/comment/publish /{base:string}/comment/publish", controller.LogAccess, middleware.ParseUserToken, middleware.UserAuth, controller.CommentPublish)
	app.HandleMany(iris.MethodPost, "/comment/praise /{base:string}/comment/praise", controller.LogAccess, middleware.ParseUserToken, middleware.UserAuth, controller.CommentPraise)
	app.HandleMany(iris.MethodGet, "/comment/{id:uint} /{base:string}/comment/{id:uint}", controller.LogAccess, middleware.ParseUserToken, middleware.HandlerTimeout, controller.CommentList)

	app.HandleMany(iris.MethodGet, "/guestbook.html /{base:string}/guestbook.html", controller.LogAccess, middleware.ParseUserToken, middleware.HandlerTimeout, controller.GuestbookPage)
	app.HandleMany(iris.MethodPost, "/guestbook.html /{base:string}/guestbook.html", middleware.ParseUserToken, controller.GuestbookForm)

	// 内容导入API

	app.HandleMany(iris.MethodPost, "/attachment/upload /{base:string}/attachment/upload", middleware.ParseUserToken, controller.VerifyApiToken, controller.AttachmentUpload)
	app.HandleMany("GET POST", "/api/import/archive /{base:string}/api/import/archive", middleware.ParseUserToken, controller.VerifyApiToken, controller.ApiImportArchive)
	app.HandleMany("GET POST", "/api/import/categories /{base:string}/api/import/categories", middleware.ParseUserToken, controller.VerifyApiToken, controller.ApiImportGetCategories)
	app.HandleMany(iris.MethodPost, "/api/import/sitemap /{base:string}/api/import/sitemap", middleware.ParseUserToken, controller.VerifyApiToken, controller.ApiImportMakeSitemap)

	// login and register
	app.Get("/login", controller.LoginPage)
	app.Get("/register", controller.RegisterPage)
	app.Get("/logout", middleware.ParseUserToken, controller.AccountLogout)
	// account party
	app.Get("/account/{route:path}", middleware.ParseUserToken, controller.AccountIndexPage)

	api := app.Party("/api", middleware.ParseUserToken)
	{
		api.Get("/captcha", controller.GenerateCaptcha)
		// WeChat official account api
		api.Get("/wechat/auth", controller.WechatAuthApi)
		api.Get("/wechat", controller.WechatApi)
		api.Post("/wechat", controller.WechatApi)

		// 友链API
		api.Post("/friendlink/create", controller.VerifyApiLinkToken, controller.ApiImportCreateFriendLink)
		api.Post("/friendlink/delete", controller.VerifyApiLinkToken, controller.ApiImportDeleteFriendLink)
		api.Get("/friendlink/check", controller.VerifyApiLinkToken, controller.ApiImportCheckFriendLink)
		api.Post("/friendlink/check", controller.VerifyApiLinkToken, controller.ApiImportCheckFriendLink)
		// 前端api
		api.Post("/login", controller.ApiLogin)
		api.Post("/register", controller.ApiRegister)
		api.Get("/user/detail", middleware.UserAuth, controller.ApiGetUserDetail)
		api.Post("/user/detail", middleware.UserAuth, controller.ApiUpdateUserDetail)
		api.Post("/user/avatar", middleware.UserAuth, controller.ApiUpdateUserAvatar)
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
		api.Post("/archive/password/check", controller.ApiCheckArchivePassword)
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
		api.Get("/archive/detail", controller.CheckApiOpen, middleware.HandlerTimeout, controller.ApiArchiveDetail)
		api.Get("/archive/filters", controller.CheckApiOpen, middleware.HandlerTimeout, controller.ApiArchiveFilters)
		api.Get("/archive/list", controller.CheckApiOpen, middleware.HandlerTimeout, controller.ApiArchiveList)
		api.Get("/archive/params", controller.CheckApiOpen, middleware.HandlerTimeout, controller.ApiArchiveParams)
		api.Get("/category/detail", controller.CheckApiOpen, middleware.HandlerTimeout, controller.ApiCategoryDetail)
		api.Get("/category/list", controller.CheckApiOpen, middleware.HandlerTimeout, controller.ApiCategoryList)
		api.Get("/module/detail", controller.CheckApiOpen, middleware.HandlerTimeout, controller.ApiModuleDetail)
		api.Get("/module/list", controller.CheckApiOpen, middleware.HandlerTimeout, controller.ApiModuleList)
		api.Get("/comment/list", controller.CheckApiOpen, middleware.HandlerTimeout, controller.ApiCommentList)
		api.Get("/setting/contact", controller.CheckApiOpen, middleware.HandlerTimeout, controller.ApiContact)
		api.Get("/setting/system", controller.CheckApiOpen, middleware.HandlerTimeout, controller.ApiSystem)
		api.Get("/setting/diy", controller.CheckApiOpen, middleware.HandlerTimeout, controller.ApiDiyField)
		api.Get("/setting/index", controller.CheckApiOpen, middleware.HandlerTimeout, controller.ApiIndexTdk)
		api.Get("/languages", controller.CheckApiOpen, middleware.HandlerTimeout, controller.ApiLanguages)
		api.Get("/guestbook/fields", controller.CheckApiOpen, middleware.HandlerTimeout, controller.ApiGuestbook)
		api.Get("/friendlink/list", controller.CheckApiOpen, middleware.HandlerTimeout, controller.ApiLinkList)
		api.Get("/nav/list", controller.CheckApiOpen, middleware.HandlerTimeout, controller.ApiNavList)
		api.Get("/archive/next", controller.CheckApiOpen, middleware.HandlerTimeout, controller.ApiNextArchive)
		api.Get("/archive/prev", controller.CheckApiOpen, middleware.HandlerTimeout, controller.ApiPrevArchive)
		api.Get("/page/detail", controller.CheckApiOpen, middleware.HandlerTimeout, controller.ApiPageDetail)
		api.Get("/page/list", controller.CheckApiOpen, middleware.HandlerTimeout, controller.ApiPageList)
		api.Get("/tag/detail", controller.CheckApiOpen, middleware.HandlerTimeout, controller.ApiTagDetail)
		api.Get("/tag/data/list", controller.CheckApiOpen, middleware.HandlerTimeout, controller.ApiTagDataList)
		api.Get("/tag/list", controller.CheckApiOpen, middleware.HandlerTimeout, controller.ApiTagList)
		api.Get("/banner/list", controller.CheckApiOpen, middleware.HandlerTimeout, controller.ApiBannerList)
		api.Post("/attachment/upload", controller.CheckApiOpen, middleware.UserAuth, controller.ApiAttachmentUpload)
		api.Post("/comment/publish", controller.CheckApiOpen, middleware.UserAuth, controller.ApiCommentPublish)
		api.Post("/comment/praise", controller.CheckApiOpen, middleware.UserAuth, controller.ApiCommentPraise)
		api.Post("/guestbook.html", controller.CheckApiOpen, controller.ApiGuestbookForm)
	}

	notify := app.Party("/notify")
	{
		notify.Get("/weapp/msg", controller.NotifyWeappMsg)
		notify.Post("/wechat/pay", controller.NotifyWechatPay)
		notify.Post("/alipay/pay", controller.NotifyAlipay)
	}

	returnParty := app.Party("/return")
	{
		returnParty.Get("/paypal/pay", controller.PaypalReturnResult)
		returnParty.Get("/paypal/cancel", controller.PaypalCancelResult)
	}

	//后台管理路由相关
	manageRoute(app, systemFiles)
}

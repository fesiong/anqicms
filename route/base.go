package route

import (
	"embed"

	"github.com/kataras/iris/v12"
	"kandaoni.com/anqicms/controller"
	"kandaoni.com/anqicms/controller/graphql"
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
	app.HandleMany(iris.MethodPost, "/comment/publish /{base:string}/comment/publish", middleware.ParseUserToken, controller.CommentPublish)
	app.HandleMany(iris.MethodPost, "/comment/praise /{base:string}/comment/praise", middleware.ParseUserToken, middleware.UserAuth, controller.CommentPraise)
	app.HandleMany(iris.MethodGet, "/comment/{id:uint} /{base:string}/comment/{id:uint}", middleware.ParseUserToken, middleware.HandlerTimeout, controller.CommentList)

	app.HandleMany(iris.MethodGet, "/guestbook.html /{base:string}/guestbook.html", middleware.ParseUserToken, middleware.HandlerTimeout, controller.GuestbookPage)
	app.HandleMany(iris.MethodPost, "/guestbook.html /{base:string}/guestbook.html", middleware.ParseUserToken, controller.GuestbookForm)

	// 内容导入API

	app.HandleMany(iris.MethodPost, "/attachment/upload /{base:string}/attachment/upload", middleware.ParseUserToken, controller.VerifyApiToken, controller.AttachmentUpload)
	app.HandleMany("GET POST", "/api/import/archive /{base:string}/api/import/archive", middleware.ParseUserToken, controller.VerifyApiToken, controller.ApiImportArchive)
	app.HandleMany("GET POST", "/api/import/categories /{base:string}/api/import/categories", middleware.ParseUserToken, controller.VerifyApiToken, controller.ApiImportGetCategories)
	app.HandleMany(iris.MethodPost, "/api/import/sitemap /{base:string}/api/import/sitemap", middleware.ParseUserToken, controller.VerifyApiToken, controller.ApiImportMakeSitemap)
	app.HandleMany(iris.MethodPost, "/api/import/review /{base:string}/api/import/review", middleware.ParseUserToken, controller.VerifyApiToken, controller.ApiImportReview)

	// login and register
	app.HandleMany(iris.MethodGet, "/login /login/{platform:string}", controller.LoginPage)
	app.HandleMany(iris.MethodGet, "/register /register/{platform:string}", controller.RegisterPage)
	app.Get("/logout", middleware.ParseUserToken, controller.AccountLogout)
	// account party
	app.Get("/account/password/reset", controller.AccountPasswordResetPage)
	app.Get("/account/{route:path}", middleware.ParseUserToken, controller.AccountIndexPage)
	// order
	app.Get("/order/{route:path}", middleware.ParseUserToken, controller.OrderIndexPage)

	api := app.Party("/api", middleware.ParseUserToken)
	{
		// 注册GraphQL API v2路由
		graphql.RegisterRoutes(api)
		api.Get("/captcha", controller.GenerateCaptcha)
		// WeChat official account api
		api.Get("/wechat/auth", controller.WechatAuthApi)
		api.Get("/wechat", controller.WechatApi)
		api.Post("/wechat", controller.WechatApi)
		// google
		api.Get("/google/url", controller.GoogleAuthUrl)
		api.Get("/log", controller.ApiLogStatistic)

		// 友链API
		api.Post("/friendlink/create", controller.VerifyApiLinkToken, controller.ApiImportCreateFriendLink)
		api.Post("/friendlink/delete", controller.VerifyApiLinkToken, controller.ApiImportDeleteFriendLink)
		api.Get("/friendlink/check", controller.VerifyApiLinkToken, controller.ApiImportCheckFriendLink)
		api.Post("/friendlink/check", controller.VerifyApiLinkToken, controller.ApiImportCheckFriendLink)
		// 前端api
		api.Post("/login", controller.ApiLogin)
		// 下面三个端点都能判断「某个邮箱是否已注册」，共用一份按 IP 的预算：
		// register 报「已注册」、verify/email 报「用户不存在」、state=exist 回布尔值。
		api.Post("/register", middleware.AccountProbeGuard, controller.ApiRegister)
		api.Get("/verify/email", middleware.AccountProbeGuard, controller.ApiVerifyEmail)
		api.Post("/verify/email", middleware.AccountProbeGuard, controller.ApiSendVerifyEmail)
		api.Post("/password/reset", controller.ApiResetUserPassword)
		api.Get("/user/detail", middleware.UserAuth, controller.ApiGetUserDetail)
		api.Post("/user/detail", middleware.UserAuth, controller.ApiUpdateUserDetail)
		api.Post("/user/avatar", middleware.UserAuth, controller.ApiUpdateUserAvatar)
		api.Get("/user/groups", controller.ApiGetUserGroups)
		api.Get("/user/group/detail", controller.ApiGetUserGroupDetail)
		api.Post("/user/password", middleware.UserAuth, controller.ApiUpdateUserPassword)
		api.Get("/orders", middleware.UserAuth, controller.ApiGetOrders)
		api.Get("/orders/statistics", middleware.ShopModeGuard, middleware.UserAuth, controller.ApiGetOrdersStatistics)
		api.Get("/orders/chart", middleware.ShopModeGuard, middleware.UserAuth, controller.ApiGetOrderChartData)
		api.Post("/order/checkout", middleware.ShopModeGuard, controller.ApiGetOrderCheckout)
		api.Post("/order/create", controller.ApiCreateOrder)
		api.Get("/order/addresses", middleware.UserAuth, controller.ApiGetOrderAddresses)
		api.Get("/order/address", middleware.UserAuth, controller.ApiGetOrderAddress)
		api.Post("/order/address", middleware.UserAuth, controller.ApiSaveOrderAddress)
		api.Get("/order/services", middleware.ShopModeGuard, controller.ApiGetOrderServices)
		api.Get("/order/detail", controller.ApiGetOrderDetail)
		api.Post("/order/cancel", middleware.UserAuth, controller.ApiCancelOrder)
		api.Post("/order/refund", middleware.UserAuth, controller.ApiApplyRefundOrder)
		api.Post("/order/finish", middleware.UserAuth, controller.ApiFinishedOrder)
		api.Post("/order/payment", controller.ApiCreateOrderPayment)
		api.Get("/order/custom_form", middleware.ShopModeGuard, controller.ApiGetOrderCustomForm)
		api.Post("/order/custom_form", middleware.ShopModeGuard, controller.ApiSaveOrderCustomForm)
		api.Post("/weapp/qrcode", middleware.UserAuth, controller.ApiCreateWeappQrcode)
		// 地址补全
		api.Get("/place/countries", middleware.ShopModeGuard, controller.ApiPlaceCountries)
		api.Get("/place/country/states", middleware.ShopModeGuard, controller.ApiPlaceCountryStates)
		api.Get("/place/country/cities", middleware.ShopModeGuard, controller.ApiPlaceCountryCities)
		api.Post("/place/autocomplete", middleware.ShopModeGuard, controller.ApiPlaceAutocomplete)
		api.Get("/activity/list", middleware.ShopModeGuard, controller.ApiActivities)
		api.Get("/activity/detail", middleware.ShopModeGuard, controller.ApiActivityDetail)
		api.Get("/archive/sku", middleware.ShopModeGuard, controller.GetArchiveSku)
		api.Get("/archive/activity", middleware.ShopModeGuard, controller.ApiGetArchiveActivity)
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
		api.Get("/wishlist/list", middleware.ShopModeGuard, middleware.UserAuth, controller.ApiWishlist)
		api.Post("/wishlist/add", middleware.ShopModeGuard, middleware.UserAuth, controller.ApiWishlistAdd)
		api.Post("/wishlist/delete", middleware.ShopModeGuard, middleware.UserAuth, controller.ApiWishlistDelete)
		api.Get("/coupon/valid", middleware.ShopModeGuard, controller.ApiGetValidCoupons)
		api.Get("/coupon/codes", middleware.ShopModeGuard, middleware.UserAuth, controller.ApiGetCouponCodes)
		api.Post("/coupon/code", middleware.ShopModeGuard, middleware.UserAuth, controller.ApiGenerateCouponCode)
		api.Post("/coupon/check", middleware.ShopModeGuard, middleware.UserAuth, controller.ApiCheckCoupon)
		api.Get("/shipping/info", middleware.ShopModeGuard, controller.ApiGetShippingInfo)
		api.Get("/favorite/list", middleware.UserAuth, controller.ApiGetFavorites)
		api.Post("/favorite/check", middleware.UserAuth, controller.ApiCheckFavorites)
		api.Post("/favorite/add", middleware.UserAuth, controller.ApiAddFavorite)
		api.Post("/favorite/delete", middleware.UserAuth, controller.ApiDeleteFavorite)
		api.Get("/cart/list", middleware.ShopModeGuard, controller.ApiCartList)
		api.Post("/cart/add", middleware.ShopModeGuard, controller.ApiCartAdd)
		api.Post("/cart/update", middleware.ShopModeGuard, controller.ApiCartUpdate)
		api.Post("/cart/remove", middleware.ShopModeGuard, controller.ApiCartDelete)
		api.Get("/communication/fields", middleware.ShopModeGuard, controller.ApiGetCommunicationOrderFields)
		api.Get("/communication/orders", middleware.ShopModeGuard, controller.ApiGetCommunicationOrderList)
		api.Post("/communication/order/create", middleware.ShopModeGuard, middleware.UserAuth, controller.ApiCreateCommunicationOrder) // 创建订单
		api.Get("/communication/order/detail", middleware.ShopModeGuard, middleware.UserAuth, controller.ApiGetCommunicationOrderDetail)
		api.Post("/communication/order/update", middleware.ShopModeGuard, middleware.UserAuth, controller.ApiCommunicationOrderUpdate) // 更新订单字段
		api.Post("/communication/order/accept", middleware.ShopModeGuard, middleware.UserAuth, controller.ApiCommunicationOrderAcceptQuote)
		api.Get("/communication/messages", middleware.ShopModeGuard, middleware.UserAuth, controller.ApiGetCommunicationMessages)
		api.Post("/communication/message/send", middleware.ShopModeGuard, middleware.UserAuth, controller.ApiSendCommunicationMessage)

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
		api.Get("/place/detail", controller.CheckApiOpen, middleware.HandlerTimeout, controller.ApiPlaceDetail)
		api.Get("/place/list", controller.CheckApiOpen, middleware.HandlerTimeout, controller.ApiPlaceList)
		api.Get("/metadata", controller.CheckApiOpen, middleware.HandlerTimeout, controller.ApiMetadata)
		api.Get("/review/list", middleware.ShopModeGuard, controller.CheckApiOpen, controller.ApiReviewList)
		api.Post("/review/publish", middleware.ShopModeGuard, controller.CheckApiOpen, controller.ApiReviewPublish)
		api.Post("/review/praise", middleware.ShopModeGuard, controller.CheckApiOpen, middleware.UserAuth, controller.ApiReviewPraise)
		api.Post("/attachment/upload", controller.CheckApiOpen, controller.ApiAttachmentUpload)
		api.Post("/comment/publish", controller.CheckApiOpen, controller.ApiCommentPublish)
		api.Post("/comment/praise", controller.CheckApiOpen, middleware.UserAuth, controller.ApiCommentPraise)
		api.Post("/guestbook.html", controller.CheckApiOpen, controller.ApiGuestbookForm)
		api.Post("/subscription", controller.CheckApiOpen, controller.ApiAddSubscriber)
	}

	notify := app.Party("/notify")
	{
		notify.Get("/weapp/msg", controller.NotifyWeappMsg)
		notify.Post("/wechat/pay", controller.NotifyWechatPay)
		notify.Post("/alipay/pay", controller.NotifyAlipay)
		notify.Post("/paypal/pay", controller.NotifyPayPal)
		notify.Post("/stripe/pay", middleware.ShopModeGuard, controller.NotifyStripe)
	}

	returnParty := app.Party("/return")
	{
		returnParty.Get("/paypal/pay", controller.PaypalReturnResult)
		returnParty.Get("/paypal/cancel", controller.PaypalCancelResult)
	}

	//后台管理路由相关
	manageRoute(app, systemFiles)

	// MCP 对外端点：按域名解析站点 + Bearer token 鉴权
	app.Any("/api/mcp", middleware.McpTokenAuth, controller.McpStreamableHTTP)
}

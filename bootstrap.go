package irisweb

import (
	"context"
	"fmt"
	"github.com/kataras/iris/v12"
	"github.com/kataras/iris/v12/middleware/recover"
	"irisweb/config"
	"irisweb/crond"
	"irisweb/middleware"
	"irisweb/model"
	"irisweb/provider"
	"irisweb/route"
	"irisweb/tags"
	"time"
)

type Bootstrap struct {
	Application *iris.Application
	Port        int
	LoggerLevel string
}

func New(port int, loggerLevel string) *Bootstrap {
	var bootstrap Bootstrap
	bootstrap.Application = iris.New()
	bootstrap.Port = port
	bootstrap.LoggerLevel = loggerLevel

	return &bootstrap
}

func (bootstrap *Bootstrap) loadGlobalMiddleware() {
	bootstrap.Application.Use(recover.New())
	bootstrap.Application.Use(middleware.Cors)
	bootstrap.Application.Use(middleware.Auth)
}

func (bootstrap *Bootstrap) Serve() {
	//自动迁移表
	if config.DB != nil {
		_ = model.AutoMigrateDB(config.DB)
		//创建管理员，会先判断有没有的。不用担心重复
		_ = provider.InitAdmin("admin", "123456")
	}

	//开始计划任务
	crond.Crond()

	bootstrap.Application.Logger().SetLevel(bootstrap.LoggerLevel)
	bootstrap.loadGlobalMiddleware()
	route.Register(bootstrap.Application)

	pugEngine := iris.Django(fmt.Sprintf("%stemplate/%s", config.ExecPath, config.JsonData.System.TemplateName), ".html")
	if config.ServerConfig.Env == "development" {
		//测试环境下动态加载
		pugEngine.Reload(true)
	}

	pugEngine.AddFunc("stampToDate", TimestampToDate)
	pugEngine.AddFunc("getUrl", provider.GetUrl)

	pugEngine.RegisterTag("tdk", tags.TagTdkParser)
	pugEngine.RegisterTag("system", tags.TagSystemParser)
	pugEngine.RegisterTag("contact", tags.TagContactParser)
	pugEngine.RegisterTag("navList", tags.TagNavListParser)
	pugEngine.RegisterTag("categoryList", tags.TagCategoryListParser)
	pugEngine.RegisterTag("categoryDetail", tags.TagCategoryDetailParser)
	pugEngine.RegisterTag("articleDetail", tags.TagArticleDetailParser)
	pugEngine.RegisterTag("productDetail", tags.TagProductDetailParser)
	pugEngine.RegisterTag("pageList", tags.TagPageListParser)
	pugEngine.RegisterTag("pageDetail", tags.TagPageDetailParser)
	pugEngine.RegisterTag("prevArticle", tags.TagPrevArticleParser)
	pugEngine.RegisterTag("nextArticle", tags.TagNextArticleParser)
	pugEngine.RegisterTag("prevProduct", tags.TagPrevProductParser)
	pugEngine.RegisterTag("nextProduct", tags.TagNextProductParser)
	pugEngine.RegisterTag("articleList", tags.TagArticleListParser)
	pugEngine.RegisterTag("productList", tags.TagProductListParser)
	pugEngine.RegisterTag("breadcrumb", tags.TagBreadcrumbParser)
	pugEngine.RegisterTag("pagination", tags.TagPaginationParser)
	pugEngine.RegisterTag("linkList", tags.TagLinkListParser)
	pugEngine.RegisterTag("commentList", tags.TagCommentListParser)
	pugEngine.RegisterTag("guestbook", tags.TagGuestbookParser)
	pugEngine.RegisterTag("articleParams", tags.TagArticleParamsParser)
	pugEngine.RegisterTag("productParams", tags.TagProductParamsParser)

	bootstrap.Application.RegisterView(pugEngine)

	bootstrap.Application.Run(
		iris.Addr(fmt.Sprintf(":%d", bootstrap.Port)),
		iris.WithoutServerError(iris.ErrServerClosed),
		iris.WithoutBodyConsumptionOnUnmarshal,
	)

	//grace := graceful.New()
	//grace.RegisterService(graceful.NewAddress(fmt.Sprintf("127.0.0.1:%d", bootstrap.Port), "tcp"), func(ln net.Listener) error {
	//	return bootstrap.Application.Run(
	//		iris.Listener(ln),
	//		iris.WithoutServerError(iris.ErrServerClosed),
	//		iris.WithoutBodyConsumptionOnUnmarshal,
	//	)
	//}, func() error {
	//	return bootstrap.Application.Shutdown(context.Background())
	//})
	//err := grace.Run()
	//if err != nil {
	//	log.Fatal(err)
	//}
}

func TimestampToDate(in int64, layout string) string {
	t := time.Unix(in, 0)
	return t.Format(layout)
}

func (bootstrap *Bootstrap) Shutdown() error {
	bootstrap.Application.Shutdown(context.Background())

	return nil
}

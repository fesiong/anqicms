package anqicms

import (
	"context"
	"fmt"
	"github.com/kataras/iris/v12"
	"github.com/skratchdot/open-golang/open"
	"kandaoni.com/anqicms/config"
	"kandaoni.com/anqicms/crond"
	"kandaoni.com/anqicms/dao"
	"kandaoni.com/anqicms/middleware"
	"kandaoni.com/anqicms/provider"
	"kandaoni.com/anqicms/route"
	"kandaoni.com/anqicms/tags"
	"log"
	"os"
	"time"
)

type Bootstrap struct {
	Application *iris.Application
	Port        int
	LoggerLevel string
}

func New(port int, loggerLevel string) *Bootstrap {
	var bootstrap Bootstrap
	bootstrap.Port = port
	bootstrap.LoggerLevel = loggerLevel

	return &bootstrap
}

func (bootstrap *Bootstrap) loadGlobalMiddleware() {
	bootstrap.Application.Use(middleware.NewRecover())
	bootstrap.Application.Use(middleware.Cors)
	bootstrap.Application.Options("*", middleware.Cors)
}

func (bootstrap *Bootstrap) Serve() {
	//自动迁移表
	if dao.DB != nil {
		_ = dao.AutoMigrateDB(dao.DB)
		//创建管理员，会先判断有没有的。不用担心重复
		_ = provider.InitAdmin("admin", "123456", false)
	}

	//开始计划任务
	crond.Crond()

	go bootstrap.Start()
	go func() {
		time.Sleep(1 * time.Second)
		link := fmt.Sprintf("http://127.0.0.1:%d", bootstrap.Port)
		if config.JsonData.System.BaseUrl != "" {
			link = config.JsonData.System.BaseUrl
		}
		err := open.Run(link)
		if err != nil {
			log.Println("请手动在浏览器输入访问地址：", link)
		}
	}()
	// 伪静态规则和模板更改变化
	for {
		<-config.RestartChan
		fmt.Println("监听到路由更改")
		bootstrap.Application.Shutdown(context.Background())
		log.Println("进程结束，开始重启")
		// 重启
		go bootstrap.Start()
	}
}

func (bootstrap *Bootstrap) Start() {
	bootstrap.Application = iris.New()
	bootstrap.Application.Logger().SetLevel(bootstrap.LoggerLevel)
	bootstrap.loadGlobalMiddleware()
	route.Register(bootstrap.Application)

	pugEngine := iris.Django(fmt.Sprintf("%stemplate/%s", config.ExecPath, config.JsonData.System.TemplateName), ".html")
	// 始终动态加载
	pugEngine.Reload(true)

	pugEngine.AddFunc("stampToDate", TimestampToDate)
	pugEngine.AddFunc("getUrl", provider.GetUrl)

	pugEngine.RegisterTag("tdk", tags.TagTdkParser)
	pugEngine.RegisterTag("system", tags.TagSystemParser)
	pugEngine.RegisterTag("contact", tags.TagContactParser)
	pugEngine.RegisterTag("navList", tags.TagNavListParser)
	pugEngine.RegisterTag("categoryList", tags.TagCategoryListParser)
	pugEngine.RegisterTag("categoryDetail", tags.TagCategoryDetailParser)
	pugEngine.RegisterTag("archiveDetail", tags.TagArchiveDetailParser)
	pugEngine.RegisterTag("pageList", tags.TagPageListParser)
	pugEngine.RegisterTag("pageDetail", tags.TagPageDetailParser)
	pugEngine.RegisterTag("prevArchive", tags.TagPrevArchiveParser)
	pugEngine.RegisterTag("nextArchive", tags.TagNextArchiveParser)
	pugEngine.RegisterTag("archiveList", tags.TagArchiveListParser)
	pugEngine.RegisterTag("breadcrumb", tags.TagBreadcrumbParser)
	pugEngine.RegisterTag("pagination", tags.TagPaginationParser)
	pugEngine.RegisterTag("linkList", tags.TagLinkListParser)
	pugEngine.RegisterTag("commentList", tags.TagCommentListParser)
	pugEngine.RegisterTag("guestbook", tags.TagGuestbookParser)
	pugEngine.RegisterTag("archiveParams", tags.TagArchiveParamsParser)
	pugEngine.RegisterTag("tagList", tags.TagTagListParser)
	pugEngine.RegisterTag("tagDetail", tags.TagTagDetailParser)
	pugEngine.RegisterTag("tagDataList", tags.TagTagDataListParser)
	pugEngine.RegisterTag("archiveFilters", tags.TagArchiveFiltersParser)

	// 模板在最后加载，避免因为模板而导致程序无法运行
	go func() {
		time.Sleep(1 * time.Second)
		bootstrap.Application.RegisterView(pugEngine)
	}()

	err := bootstrap.Application.Run(
		iris.Addr(fmt.Sprintf(":%d", bootstrap.Port)),
		iris.WithRemoteAddrHeader("X-Real-IP"),
		iris.WithRemoteAddrHeader("X-Forwarded-For"),
		iris.WithoutServerError(iris.ErrServerClosed),
		iris.WithoutBodyConsumptionOnUnmarshal,
		iris.WithoutPathCorrection,
	)

	if err != nil {
		log.Println(err.Error())
		os.Exit(0)
	}
}

func TimestampToDate(in int64, layout string) string {
	t := time.Unix(in, 0)
	return t.Format(layout)
}

func (bootstrap *Bootstrap) Shutdown() error {
	bootstrap.Application.Shutdown(context.Background())

	return nil
}

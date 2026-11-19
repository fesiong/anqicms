package anqicms

import (
	stdContext "context"
	"fmt"
	"log"
	"log/slog"
	"os"
	"strconv"
	"strings"
	"time"

	"github.com/kataras/iris/v12"
	"github.com/skratchdot/open-golang/open"
	"kandaoni.com/anqicms/config"
	"kandaoni.com/anqicms/crond"
	"kandaoni.com/anqicms/library"
	"kandaoni.com/anqicms/middleware"
	"kandaoni.com/anqicms/pkg/mcp/server"
	"kandaoni.com/anqicms/provider"
	"kandaoni.com/anqicms/route"
	"kandaoni.com/anqicms/tags"
	"kandaoni.com/anqicms/view"
)

type Bootstrap struct {
	Application *iris.Application
	Port        int
	LoggerLevel string
	viewEngine  *view.DjangoEngine
}

func New(port int, loggerLevel string) *Bootstrap {
	var bootstrap Bootstrap
	bootstrap.Port = port
	bootstrap.LoggerLevel = loggerLevel
	if loggerLevel == "error" {
		slog.SetLogLoggerLevel(slog.LevelError)
	} else {
		slog.SetLogLoggerLevel(slog.LevelDebug)
	}

	return &bootstrap
}

func (bootstrap *Bootstrap) loadGlobalMiddleware() {
	// ⚠️ 第一个必须是启动闸门。站点初始化（含库表迁移）在后台跑，端口先开；
	// 初始化没完成时注册表是空的/只注册了一部分，任何业务 handler 跑起来
	// 都会踩到「全新安装语义」或未注册站点的 nil 兜底。详见 provider/startup.go。
	bootstrap.Application.Use(middleware.StartupGate)
	bootstrap.Application.Use(middleware.NewRecover())
	bootstrap.Application.Use(middleware.Cors)
	bootstrap.Application.Use(middleware.SecurityHeaders)
	bootstrap.Application.Options("{path:path}", middleware.Cors)
}

func (bootstrap *Bootstrap) Serve() {
	// 初始化 SiteMcpPool（缓存每站点的 StreamableHTTPHandler）
	mcpPool := server.NewSiteMcpPool(slog.Default())
	provider.SetMcpPool(mcpPool)

	// 渠道长连接管理器默认不随站点初始化启动（测试/工具不连平台）；
	// 进程入口必须在这里显式打开，且要在 InitWebsites 之前。
	//
	// 必须放在 GetDefaultDB() 判空之外：全新安装时启动阶段还没有 config.json，
	// GetDefaultDB() 返回 nil，安装流程（controller/install.go）随后调用 InitWebsite
	// 时开关若仍是 false，该进程内的渠道管理器会全程不启动 —— 装完只发
	// RestartChan{Code:0} 重载模板、不重启进程，T9 保存配置就会拿到「渠道管理器未启动」。
	provider.EnableChannelManagerOnInit()
	// 通知投递工作者同理：默认不随站点初始化启动（测试/工具不发通知），
	// 进程入口显式打开，且必须在 InitWebsites 之前。
	provider.EnableNotifyWorkerOnInit()

	// 顺序：装路由/i18n/模板 → 开端口 → 后台做站点初始化（含库表迁移）。
	//
	// 为什么不能再等初始化完成才监听（2026-11-16 客户 106 站点事故）：
	// 站点初始化里的库表迁移是同步的，单站点几秒到几分钟，106 个站点就是
	// 几十分钟到几小时。这段卡在 `go bootstrap.Start()` 之前，表现就是
	// 「进程起来了、日志不动、端口一直不开」—— 运维无法区分"在跑"和"卡死"，
	// 客户现场因此反复重启，每次重启都要从被中断的站点重来。
	//
	// 现在端口先开，业务请求由 middleware.StartupGate 挡成 503 + Retry-After，
	// 进度通过 GET /__startup 与每 15 秒一条的心跳日志对外可见；
	// 站点注册表仍然遵守「迁移成功才注册」的不变式，不会把半迁移结构暴露出去。
	bootstrap.Init()

	if provider.GetDefaultDB() != nil {
		// 闸门要在端口打开**之前**关上：端口就绪到初始化开始之间的几百微秒里，
		// 注册表还是空的，请求会拿到"全新安装"语义的一堆 404。
		provider.PrepareStartupGate()
	}
	go bootstrap.Listen()
	provider.InitWebsitesAsync()

	//开始计划任务
	crond.Crond()

	go func() {
		time.Sleep(1 * time.Second)
		currentSite := provider.CurrentSite(nil)
		link := "http://127.0.0.1:" + strconv.Itoa(bootstrap.Port)
		if currentSite != nil && currentSite.System.BaseUrl != "" {
			if strings.Contains(currentSite.System.BaseUrl, "127.0.0.1") {
				currentSite.System.BaseUrl = link
			}
			link = currentSite.System.BaseUrl
		}
		err := open.Run(link)
		if err != nil {
			log.Println("请手动在浏览器输入访问地址：", link)
		}
	}()
	// 伪静态规则和模板更改变化
	for restart := range config.RestartChan {
		switch restart.Code {
		case 1:
			fmt.Println("监听到路由更改")
			_ = bootstrap.Shutdown()
			log.Println("进程结束，开始重启")
			// 到这里监听已经停了，Restart 再失败就是「服务已下线且没人拉起」，
			// 必须让错误可见并退出，交给守护进程重新拉起，而不是静默留一个空壳进程。
			if err := provider.Restart(); err != nil {
				log.Println("重启失败，进程退出等待守护拉起：", err)
				os.Exit(1)
			}
		case 2:
			fmt.Println("监听到退出信号")
			//_ = bootstrap.Shutdown()
			os.Exit(0)
		default:
			fmt.Println("重载模板")
			bootstrap.viewEngine.LoadTemplates(restart.SiteId, nil)
		}
	}
}

// Init 装配 iris 应用：路由、i18n、模板引擎。不监听端口、不碰站点数据。
// 必须与 Listen 分开：它很快（毫秒级），而站点初始化可能要跑几小时。
func (bootstrap *Bootstrap) Init() {
	bootstrap.Application = iris.New()
	bootstrap.Application.Logger().SetLevel(bootstrap.LoggerLevel)
	bootstrap.loadGlobalMiddleware()
	route.Register(bootstrap.Application, SystemFiles)
	err := bootstrap.Application.I18n.Load(config.ExecPath+"locales/*/*.yml", config.LoadLocales()...)
	if err != nil {
		log.Println("languages err", err)
		os.Exit(1)
	}
	bootstrap.Application.I18n.Cookie = "lang"
	bootstrap.Application.I18n.Subdomain = false
	bootstrap.Application.I18n.PathRedirect = false
	bootstrap.Application.I18n.SetDefault("zh-CN")
	// 注入I18n 到 provider
	provider.SetI18n(bootstrap.Application.I18n)
	bootstrap.Application.I18n.Tags()

	pugEngine := view.Django(".html")
	// 开发模式下动态加载
	if config.Server.Server.Env == "development" {
		pugEngine.Reload(true)
	}

	pugEngine.AddFunc("stampToDate", tags.TimestampToDate)
	pugEngine.AddFunc("priceFormat", tags.PriceFormat)
	pugEngine.AddFunc("range", tags.Range)
	pugEngine.AddFunc("CustomFunc", tags.CustomFunc)

	_ = pugEngine.RegisterTag("tr", tags.TagTrParser)
	_ = pugEngine.RegisterTag("tdk", tags.TagTdkParser)
	_ = pugEngine.RegisterTag("diy", tags.TagDiyParser)
	_ = pugEngine.RegisterTag("system", tags.TagSystemParser)
	_ = pugEngine.RegisterTag("contact", tags.TagContactParser)
	_ = pugEngine.RegisterTag("navList", tags.TagNavListParser)
	_ = pugEngine.RegisterTag("categoryList", tags.TagCategoryListParser)
	_ = pugEngine.RegisterTag("categoryDetail", tags.TagCategoryDetailParser)
	_ = pugEngine.RegisterTag("archiveDetail", tags.TagArchiveDetailParser)
	_ = pugEngine.RegisterTag("pageList", tags.TagPageListParser)
	_ = pugEngine.RegisterTag("pageDetail", tags.TagPageDetailParser)
	_ = pugEngine.RegisterTag("prevArchive", tags.TagPrevArchiveParser)
	_ = pugEngine.RegisterTag("nextArchive", tags.TagNextArchiveParser)
	_ = pugEngine.RegisterTag("archiveList", tags.TagArchiveListParser)
	_ = pugEngine.RegisterTag("breadcrumb", tags.TagBreadcrumbParser)
	_ = pugEngine.RegisterTag("pagination", tags.TagPaginationParser)
	_ = pugEngine.RegisterTag("linkList", tags.TagLinkListParser)
	_ = pugEngine.RegisterTag("commentList", tags.TagCommentListParser)
	_ = pugEngine.RegisterTag("guestbook", tags.TagGuestbookParser)
	_ = pugEngine.RegisterTag("guestbookList", tags.TagGuestbookListParser)
	_ = pugEngine.RegisterTag("archiveParams", tags.TagArchiveParamsParser)
	_ = pugEngine.RegisterTag("tagList", tags.TagTagListParser)
	_ = pugEngine.RegisterTag("tagDetail", tags.TagTagDetailParser)
	_ = pugEngine.RegisterTag("tagDataList", tags.TagTagDataListParser)
	_ = pugEngine.RegisterTag("archiveFilters", tags.TagArchiveFiltersParser)
	_ = pugEngine.RegisterTag("userDetail", tags.TagUserDetailParser)
	_ = pugEngine.RegisterTag("userGroupDetail", tags.TagUserGroupDetailParser)
	_ = pugEngine.RegisterTag("bannerList", tags.TagBannerListParser)
	_ = pugEngine.RegisterTag("moduleDetail", tags.TagModuleDetailParser)
	_ = pugEngine.RegisterTag("languages", tags.TagLanguagesParser)
	_ = pugEngine.RegisterTag("jsonLd", tags.TagJsonLdParser)
	_ = pugEngine.RegisterTag("placeList", tags.TagPlaceListParser)
	_ = pugEngine.RegisterTag("placeDetail", tags.TagPlaceDetailParser)
	_ = pugEngine.RegisterTag("attachment", tags.TagAttachmentParser)
	_ = pugEngine.ReplaceTag("set", tags.TagSetParser)
	_ = pugEngine.RegisterTag("jump", tags.TagJumpParser)

	bootstrap.viewEngine = pugEngine
	// 站点注册完成后重建模板索引。
	// RegisterView 触发的 Load() 发生在站点注册之前（Init 早于 InitWebsitesAsync），
	// 那时注册表为空，模板索引会是空的 → 所有动态页 404。这里提供重载入口，
	// 由 provider.InitWebsites 在“站点已注册、启动闸门未放行”的窗口内调用。
	provider.SetTemplateReloader(func() {
		for _, site := range provider.GetWebsites() {
			if !site.Initialed {
				continue
			}
			if err := pugEngine.LoadTemplates(site.Id, site); err != nil {
				log.Println("加载模板失败：", site.Id, err)
			}
		}
	})
	// 模板在最后加载，避免因为模板而导致程序无法运行
	bootstrap.Application.RegisterView(pugEngine)
}

// Listen 开始监听端口，阻塞到服务退出。
//
// 从 Start() 里拆出来是本次修复的关键：监听必须在站点初始化之前开始，
// 否则 106 个站点做库表迁移时端口几十上百年不开（见 Serve 里的注释）。
func (bootstrap *Bootstrap) Listen() {
	err := bootstrap.Application.Run(
		iris.Addr(":"+strconv.Itoa(bootstrap.Port)),
		iris.WithRemoteAddrHeader("X-Real-IP", "X-Forwarded-For", "CF-Connecting-IP"),
		iris.WithHostProxyHeader("X-Host"),
		iris.WithoutServerError(iris.ErrServerClosed),
		iris.WithoutBodyConsumptionOnUnmarshal,
		iris.WithoutPathCorrection,
	)

	if err != nil {
		log.Println(err.Error())
		library.DebugLog(config.ExecPath, "error.log", time.Now().Format("2006-01-02 15:04:05"), "启动服务出错", err.Error())
		os.Exit(0)
	}
}

func (bootstrap *Bootstrap) Shutdown() error {
	// 监听没起来就收到重启信号时（例如启动期就被 kill/覆盖安装），
	// Application 还是 nil；这里不判空就是 nil 解引用。
	if bootstrap.Application != nil {
		bootstrap.Application.Shutdown(stdContext.Background())
	}
	provider.Shutdown()
	// 关闭一些应用
	crond.Stop()

	return nil
}

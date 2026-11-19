package provider

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"log"
	"log/slog"
	"net/url"
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"strconv"
	"strings"
	"sync"
	"time"

	"golang.org/x/oauth2"
	"kandaoni.com/anqicms/pkg/mcp/intent"
	"kandaoni.com/anqicms/pkg/mcp/server"
	"kandaoni.com/anqicms/provider/storage"

	"github.com/esap/wechat"
	"github.com/kataras/iris/v12"
	"github.com/kataras/iris/v12/i18n"
	"github.com/medivhzhan/weapp/v3"
	"gorm.io/gorm"
	"kandaoni.com/anqicms/config"
	"kandaoni.com/anqicms/library"
	"kandaoni.com/anqicms/model"
	"kandaoni.com/anqicms/provider/fulltext"
	"kandaoni.com/anqicms/provider/segmenter"
	"kandaoni.com/anqicms/response"
)

type Website struct {
	// 前端需要的字段
	Id           uint   `json:"id"`
	ParentId     uint   `json:"parent_id"`
	Name         string `json:"name"`          // 来自数据库的
	LanguageIcon string `json:"language_icon"` // 图标
	// e
	TokenSecret             string
	Mysql                   *config.MysqlConfig
	Initialed               bool
	ErrorMsg                string // 错误提示
	Scheme                  string
	Host                    string
	BaseURI                 string
	RootPath                string
	DataPath                string
	CachePath               string
	PublicPath              string
	DB                      *gorm.DB
	StatisticLog            *StatisticLog
	Storage                 storage.Storage
	CacheStorage            storage.Storage
	parsedPattern           *RewritePattern
	searcher                fulltext.Service
	fulltextStatus          *FulltextStatus
	cachedTodayArticleCount *response.CacheArticleCount
	transferWebsite         *TransferWebsite
	weappClient             *weapp.Client
	wechatServer            *wechat.Server
	Cache                   library.Cache
	HtmlCacheStatus         *HtmlCacheStatus
	HtmlCachePushStatus     *HtmlCacheStatus
	quickImportStatus       *QuickImportArchive
	AkismetClient           *AkismetClient
	googleAuthConfig        *oauth2.Config
	googleMerchantConfig    *oauth2.Config
	llmsBuildStatus         *LLMsBuildStatus

	System   *config.SystemConfig
	Content  *config.ContentConfig
	Index    *config.IndexConfig
	Contact  *config.ContactConfig
	Shipping *config.ShippingConfig
	Safe     *config.SafeConfig
	Banner   *config.BannerConfig
	//plugin
	PluginPush         *config.PluginPushConfig
	PluginSitemap      *config.PluginSitemapConfig
	PluginRewrite      *config.PluginRewriteConfig
	PluginAnchor       *config.PluginAnchorConfig
	PluginGuestbook    *config.PluginGuestbookConfig
	PluginUploadFiles  []config.PluginUploadFile
	PluginSendmail     *config.PluginSendmail
	PluginSms          *config.PluginSmsConfig
	PluginImportApi    *config.PluginImportApiConfig
	PluginStorage      *config.PluginStorageConfig
	PluginWeapp        *config.PluginWeappConfig
	PluginWechat       *config.PluginWeappConfig
	PluginRetailer     *config.PluginRetailerConfig
	PluginUser         *config.PluginUserConfig
	PluginOrder        *config.PluginOrderConfig
	PluginFulltext     *config.PluginFulltextConfig
	PluginTitleImage   *config.PluginTitleImageConfig
	PluginWatermark    *config.PluginWatermark
	PluginHtmlCache    *config.PluginHtmlCache
	SensitiveWords     []string
	AiGenerateConfig   *config.AiGenerateConfig
	PluginInterference *config.PluginInterference
	PluginTimeFactor   *config.PluginTimeFactor
	MultiLanguage      *config.PluginMultiLangConfig
	PluginTranslate    *config.PluginTranslateConfig
	PluginJsonLd       *config.PluginJsonLdConfig
	Currency           *config.CurrencyConfig
	PluginLLMs         *config.PluginLLMsConfig
	PluginPlace        *config.PluginPlaceConfig

	sensitiveAcMatcher         *library.AhoCorasick
	sensitiveRegexes           []*regexp.Regexp
	sensitiveRegexReplacements map[string]string
	anchorAcMatcher            *library.AhoCorasick

	CollectorConfig *config.CollectorJson
	KeywordConfig   *config.KeywordJson
	Proxy           *ProxyIPs // 代理

	FindPasswordInfo *response.FindPasswordInfo
	Limiter          *Limiter
	TplI18n          *i18n.I18n
	// 一些缓存内容
	languages    map[string]string
	backLanguage string
	ctx          iris.Context // 这个类型是指针，因此只能在拷贝后赋值
	Template     *StoreTemplates
	ShopCountLog *ShopCountLog
	MailService  *MailService
	// ai
	AiSrv *AiChatService
	// mcp server (per-site, 同 AiSrv 模式)
	McpSrv *server.Server
}

// NewMcpServer 为当前站点创建 mcp.Server 实例，并注册"意图导向工作流工具"。
//
// 重构 ABD（2026-09-18）：不再把 105 个 endpoint 镜像工具直接暴露，而是下沉到
// intent.Kernel —— 以"意图"作为工具边界，由策略层（能力域）与声明式能力清单
// （IntentCatalog）驱动。底层能力（原 endpoint 工具）通过 CapInvoker 回调注入，
// 内核负责注册、路由、两阶段 tools/list 与结构化输出。site_id 切换由调用方路由
// 到目标站点的 McpSrv，其内核 CapInvoker 绑定目标站点 AiSrv，天然按目标站点操作。
func (w *Website) NewMcpServer() *server.Server {
	mcpCfg := server.DefaultConfig()
	mcpSrv, err := server.New(mcpCfg)
	if err != nil {
		slog.Error("Failed to create MCP server for site", "siteId", w.Id, "error", err)
		return nil
	}
	mcpServer := mcpSrv.GetServer()

	// 底层能力注入：把意图参数序列化后调用对应 endpoint 工具 handler。
	// 延迟到调用时解析 w.AiSrv（构建期可能尚未初始化），保证 NewMcpServer 可独立调用。
	capInvoker := func(ctx context.Context, name string, args map[string]any) (string, error) {
		if w.AiSrv == nil {
			return "", fmt.Errorf("站点 AI 服务未初始化")
		}
		// 必须走 ResolveCap 而非直接读 Handlers：Handlers 中同名意图会覆盖同名 cap，
		// 委托时会命中意图 handler 自身，形成无限递归。
		h, ok := w.AiSrv.ResolveCap(name)
		if !ok {
			return "", fmt.Errorf("底层能力 %s 不存在或未接入 MCP", name)
		}
		raw, err := json.Marshal(args)
		if err != nil {
			return "", fmt.Errorf("参数序列化失败: %w", err)
		}
		return h(ctx, string(raw))
	}
	// 审计回调：内核每次意图执行后调用，风险等级由 spec.Risk 精确给出。
	auditFn := func(ctx context.Context, name, risk, argsJSON string, callErr error, start time.Time) {
		w.recordMcpAudit(ctx, name, risk, argsJSON, callErr, start)
	}

	mcpConf := GetMcpConfig()
	iconf := intent.Config{
		ExposedIntents: mcpConf.ExposedIntents,
		ExposedTools:   mcpConf.ExposedTools,
		ToolListMode:   mcpConf.ToolListMode,
		EnableSetScope: mcpConf.EnableSetScope,
	}
	kernel := intent.NewKernel(iconf, capInvoker, auditFn)
	kernel.RegisterAll(mcpServer)

	slog.Info("MCP intent kernel registered", "siteId", w.Id, "intents", kernel.RegisteredCount())

	w.McpSrv = mcpSrv
	return mcpSrv
}

// mcpPool 是全局 SiteMcpPool 实例，由 bootstrap 初始化。
var mcpPool *server.SiteMcpPool

// SetMcpPool 注入全局 SiteMcpPool（bootstrap 阶段调用）。
func SetMcpPool(pool *server.SiteMcpPool) {
	mcpPool = pool
}

// GetMcpPool 返回全局 SiteMcpPool。
func GetMcpPool() *server.SiteMcpPool {
	return mcpPool
}

func (w *Website) Ctx() context.Context {
	if w.ctx == nil {
		return context.TODO()
	}
	return w.ctx.Request().Context()
}

// Async 返回一个脱离了请求上下文的站点副本，专供 `go ...` 这类后台任务使用。
//
// 为什么必须有它：CurrentSite 返回的是 cloneWithContext 造出来的副本，副本上的
// ctx 就是**本次请求**的 ctx。handler 一旦返回，net/http 立刻 cancel 这个 ctx，
// 之后所有 `w.DB.WithContext(w.Ctx())` 的查询都会直接失败（实测 err=context canceled），
// 而调用方普遍不检查错误 —— 于是任务静默跑一半就没了。
//
//	// 错：go 出去的任务挂着请求 ctx，请求返回后查库全失败
//	go currentSite.ReplaceAnchor(nil)
//	// 对：先摘掉 ctx，Ctx() 退化成 context.TODO()
//	go currentSite.Async().ReplaceAnchor(nil)
//
// 只复制指针字段（浅拷贝），与 cloneWithContext 的行为一致；不做深拷贝是因为
// 任务需要和主实例共享 DB、Cache、模板等运行时资源。
func (w *Website) Async() *Website {
	if w.ctx == nil {
		return w
	}
	cloned := *w
	cloned.ctx = nil
	return &cloned
}

func (w *Website) CtxOri() iris.Context {
	return w.ctx
}

func (w *Website) GetLang() string {
	return w.backLanguage
}

type OrderedWebsites struct {
	mu   sync.RWMutex
	data map[uint]*Website
	keys []uint
}

func NewWebsites() *OrderedWebsites {
	return &OrderedWebsites{
		mu:   sync.RWMutex{},
		data: make(map[uint]*Website),
		keys: make([]uint, 0),
	}
}

func (ow *OrderedWebsites) Set(website *Website) {
	ow.mu.Lock()
	defer ow.mu.Unlock()

	if _, exists := ow.data[website.Id]; !exists {
		// New id
		ow.keys = append(ow.keys, website.Id)
	}
	ow.data[website.Id] = website
}

func (ow *OrderedWebsites) Get(id uint) (*Website, bool) {
	ow.mu.RLock()
	defer ow.mu.RUnlock()

	website, exists := ow.data[id]
	return website, exists
}

func (ow *OrderedWebsites) MustGet(id uint) *Website {
	website, _ := ow.Get(id)
	return website
}

func (ow *OrderedWebsites) Delete(id uint) {
	ow.mu.Lock()
	defer ow.mu.Unlock()

	if _, exists := ow.data[id]; !exists {
		return
	}
	// Remove from map
	delete(ow.data, id)

	// Remove from keys slice
	for i, key := range ow.keys {
		if key == id {
			ow.keys = append(ow.keys[:i], ow.keys[i+1:]...)
			break
		}
	}
}

func (ow *OrderedWebsites) Len() int {
	ow.mu.RLock()
	defer ow.mu.RUnlock()

	return len(ow.keys)
}

func (ow *OrderedWebsites) Keys(baseUrl string) []uint {
	ow.mu.RLock()
	defer ow.mu.RUnlock()
	var ids []uint

	if baseUrl == "" {
		return append(ids, ow.keys...)
	}

	for _, w := range ow.data {
		if strings.Contains(w.System.BaseUrl, baseUrl) {
			ids = append(ids, w.Id)
		}
	}
	return ids
}

func (ow *OrderedWebsites) Values() []*Website {
	ow.mu.RLock()
	defer ow.mu.RUnlock()

	values := make([]*Website, 0, len(ow.keys))
	for _, id := range ow.keys {
		if w, ok := ow.data[id]; ok {
			values = append(values, w)
		}
	}
	// 按 id 排序，不要改成"注册顺序"。
	//
	// 启动期现在是 2 个站点并行初始化（siteInitRunner.go），先迁完的先注册 ——
	// 注册顺序不再等于 id 顺序。而域名匹配（matchByURIAndHost / matchRootAndFallback）
	// 是"第一个命中就返回"：同一域名下有主站 + 二级目录站点时，先注册的那个会赢。
	// 排回 id 升序 = 恢复串行时代的确定性，否则主站首页可能被某个二级目录站点接管。
	sort.Slice(values, func(i, j int) bool { return values[i].Id < values[j].Id })
	return values
}

var websites = NewWebsites()

// templateReloadFn 由进程入口注入（provider 不能 import view，否则循环依赖）。
// 用于在站点注册完成后重建模板索引，见 InitWebsites 末尾的调用。
var templateReloadFn func()

// SetTemplateReloader 注入模板重载实现。必须在 InitWebsitesAsync 之前调用。
func SetTemplateReloader(fn func()) {
	templateReloadFn = fn
}

func InitWebsites() {
	// 先需要获得 default db
	db := GetDefaultDB()
	if db == nil {
		return
	}
	// 分词词典在进程内只加载一次（实测：5.3MB / 35 万词条 → 0.84s、堆常驻 +124MB）。
	// 放在遍历站点之前：
	//  1. 106 个站点的 InitFulltext goroutine 都会用到它，先加载完就没有并发首载；
	//  2. 加载耗时归属启动期，而不是"第一个请求莫名很慢"。
	// 词典缺失时这里只记错误、返回 nil，站点照常初始化（分词降级）。
	segmenter.Shared()
	// 如果是从旧版升级上来的，需要先创建website表，并注入相关信息
	if !db.Migrator().HasTable("websites") {
		db.AutoMigrate(&model.Website{})
	}
	defaultSite := model.Website{
		Id:       1,
		RootPath: config.ExecPath,
		Name:     "Default Website",
		Status:   1,
	}
	db.Where("`id` = 1").FirstOrCreate(&defaultSite)
	var sites []*model.Website
	db.Order("`id` asc").Find(&sites)
	// 迁移进度的分母：本轮一共要看多少个站点。
	BeginSiteMigrationRound(len(sites))
	// 并发度复用迁移闸门（默认 2，`ANQI_MIGRATE_CONCURRENCY` 可调）：每个 worker
	// 内部仍要抢 MigrateSiteSchema 的令牌，所以 DDL 的实际并发不会超过闸门容量。
	// 站点 1 会先单独跑完，父子站点按父先子后派发（见 siteInitRunner.go）。
	initSitesConcurrently(sites, resolveMigrateConcurrency(), InitWebsite)
	if total := len(sites); total > 0 {
		// 没有这行日志，客户只能看到"站点一个个变正常"；迁移要几十分钟时，
		// 「还在跑」和「卡死了」在面板上完全无法区分。
		log.Printf("站点初始化完成：%s", MigrateRoundSummary())
	}
	// 检查多语言站点
	values := websites.Values()
	for _, w := range values {
		if !w.Initialed {
			continue
		}
		if w.MultiLanguage != nil && w.MultiLanguage.Open {
			if w.MultiLanguage.SiteType == config.MultiLangSiteTypeMulti {
				// 读取子站点
				multiLangSites := w.GetMultiLangSites(w.Id, false)
				w.MultiLanguage.SubSites = make([]config.MultiLangSite, 0, len(multiLangSites)*2)
				mainBaseUrl := w.System.BaseUrl
				if w.System.FrontUrl != "" {
					mainBaseUrl = w.System.FrontUrl
				}
				for _, v := range multiLangSites {
					tmpSite := config.MultiLangSite{
						Id:           v.Id,
						Language:     v.Language,
						LanguageIcon: v.LanguageIcon,
					}
					w.MultiLanguage.SubSites = append(w.MultiLanguage.SubSites, tmpSite)
					// 根据多语言站点的规则，改变baseUrl 和 storageUrl
					if w.MultiLanguage.Type != config.MultiLangTypeDomain {
						curSite, ok := websites.Get(v.Id)
						if ok {
							var baseUrl string
							if w.MultiLanguage.Type == config.MultiLangTypeDirectory {
								baseUrl = mainBaseUrl + "/" + v.Language
							} else {
								baseUrl = mainBaseUrl
							}
							curSite.PluginStorage.StorageUrl = baseUrl
							//curSite.System.BaseUrl = baseUrl
						}
					}
				}
			}
		}

	}

	// 模板索引必须在这里补一次：bootstrap.Init()（含 viewEngine.Load()）跑在
	// InitWebsitesAsync() 之前，那一刻注册表是空的，LoadStart 什么也没加载。
	// 结果就是站点都"就绪"了，但模板索引为空 —— TemplateExist 恒 false，
	// 所有动态页 404，只有 FileServe 命中的静态文件还能访问。
	// 放在这里而不是 FinishStartup 之后：此时闸门还没放行，没有并发请求。
	if templateReloadFn != nil {
		templateReloadFn()
	}
}

func InitWebsite(mw *model.Website) {
	var db *gorm.DB
	var err error
	if mw.Id == 1 {
		// 站点 1的数据库信息使用 defaultDB
		db = defaultDB
		mw.RootPath = config.ExecPath
	} else {
		if mw.Mysql.UseDefault {
			mw.Mysql.User = config.Server.Mysql.User
			mw.Mysql.Password = config.Server.Mysql.Password
			mw.Mysql.Host = config.Server.Mysql.Host
			mw.Mysql.Port = config.Server.Mysql.Port
		}
		mw.RootPath = filepath.Clean(mw.RootPath)
		db, err = InitDB(&mw.Mysql)
	}
	if !strings.HasSuffix(mw.RootPath, string(filepath.Separator)) {
		mw.RootPath = mw.RootPath + string(filepath.Separator)
	}
	if mw.Id > 0 && mw.TokenSecret == "" {
		mw.TokenSecret = config.GenerateRandString(32)
		GetDefaultDB().Save(mw)
	}
	lang, exists := os.LookupEnv("LANG")
	if !exists {
		lang = "zh-CN"
	} else {
		lang = strings.ReplaceAll(strings.Split(lang, ".")[0], "_", "-")
	}
	w := Website{
		Id:           mw.Id,
		ParentId:     mw.ParentId,
		Name:         mw.Name,
		LanguageIcon: mw.LanguageIcon,
		TokenSecret:  mw.TokenSecret,
		Mysql:        &mw.Mysql,
		DB:           db,
		Scheme:       "http",
		BaseURI:      "/",
		RootPath:     mw.RootPath,
		CachePath:    mw.RootPath + "cache" + string(filepath.Separator),
		DataPath:     mw.RootPath + "data" + string(filepath.Separator),
		PublicPath:   mw.RootPath + "public" + string(filepath.Separator),
		backLanguage: lang,
		Template: &StoreTemplates{
			Templates: make(map[string]int64),
			mu:        sync.Mutex{},
		},
		cachedTodayArticleCount: &response.CacheArticleCount{},
		// 零值占位：本函数是「先 websites.Set(&w)、再 w.InitSetting()」，
		// 中间那段窗口里别的 goroutine（crond / 请求）能拿到这个站点。
		// AiGenerateConfig 原本要等 InitSetting → LoadAiGenerateSetting 才有值，
		// 提前读就是 nil 解引用。给个零值（Open=false）让半初始化站点安全地什么都不做。
		AiGenerateConfig: &config.AiGenerateConfig{},
	}
	if db == nil {
		w.ErrorMsg = w.Tr("DatabaseConnectionFailed")
		if err != nil {
			w.ErrorMsg = "：" + err.Error()
		}
		// 这是「站点列表莫名 500」的唯一线索：库连不上的站点会以 System == nil
		// 的状态留在注册表里，后台只能看到 status:0，看不出原因。
		// 2026-11-15 的线上事故就是这类站点让 /website/list 的某一页反复 500，
		// 而日志里只有 recover 栈、没有任何「哪个站点坏了」的记录。
		slog.Error("站点数据库连接失败，该站点将以未初始化状态运行",
			"siteId", mw.Id, "name", mw.Name, "reason", w.ErrorMsg)
	}
	// 先判断目录是否存在。对于迁移的站点，这个地方可能是会出错的
	pathOK := true
	_, err = os.Stat(mw.RootPath)
	if err != nil {
		pathOK = false
		w.ErrorMsg = w.Tr("SitePathError:") + err.Error()
	}
	if mw.Id == 1 {
		w.Mysql = &config.Server.Mysql
	}
	// 顺序不能动：迁移 → InitSetting → InitModelData → 注册。
	//
	// 迁移必须**同步**跑完再往下走，理由有三条（2026-11-16 客户 106 站点事故）：
	//   1. 过去这里是 `go AutoMigrateDB(db, false)`，106 个站点就有 106 份迁移并发，
	//      把 MySQL 的 DDL 打满，启动永远跑不完（日志见 provider/siteMigration.go 头注释）；
	//   2. 迁移与 InitModelData 里的 initArchiveSchema 改的是同一张 archives 表，
	//      并发跑就是在抢元数据锁；
	//   3. 站点一旦先注册，请求就能读到半迁移的表结构 —— 那比站点暂时不可用更难查。
	ready := db != nil && mw.Status == 1 && pathOK
	if db != nil {
		// 记录"当前正在处理哪个站点"：迁移一个站点可能要几分钟，
		// 期间除了心跳日志没有任何输出，这条信息让 tail -f 的人和
		// GET /__startup 都能看到具体卡在哪个站点上。
		// 启动期是并发的（默认 2 个），所以登记的是一个集合，退出时必须摘掉。
		SetStartupCurrent(w.Id, w.Name)
		defer ClearStartupCurrent(w.Id)
		if migrateErr := MigrateSiteSchema(w.Id, w.Name, db); migrateErr != nil {
			// 半迁移的站点不能当可用站点：库表可能只改了一半。
			ready = false
			w.ErrorMsg = "数据库升级失败：" + migrateErr.Error()
		} else {
			w.InitSetting()
			w.InitModelData()
			// fix BaseUri
			parsed, err := url.Parse(w.System.BaseUrl)
			if err == nil {
				if parsed.RequestURI() != "/" {
					w.BaseURI = parsed.RequestURI()
				}
				w.Scheme = parsed.Scheme
				w.Host = parsed.Host
			}
		}
	}
	// 注册要早于 GetRewritePattern：后者把解析结果回写到注册表里的同一个对象
	// （rewrite.go 的 `w2 := GetWebsite(w.Id)`），没注册就是 nil 解引用。
	// 但"注册"不等于"能对外服务"，两者分开记：可服务标记在函数末尾才打
	// （markSiteServable），闸门只认那个（见 provider/siteAccess.go）。
	unmarkSiteServable(w.Id)
	w.Initialed = ready
	websites.Set(&w)
	if w.Initialed {
		w.GetRewritePattern(true)
		// 启动限流器
		w.InitLimiter()
		w.InitStatistic()
		w.InitBucket()
		w.InitCacheBucket()
		w.InitCache()
		w.InitAkismet()
		// AI: 先创建 ai chat service（初始化工具），再创建 mcp server（注册工具）
		w.NewAiChatService()
		w.NewMcpServer()
		// 渠道长连接管理器：只对已启用渠道建立连接。老库可能还没迁移渠道表，
		// 管理器内部只记日志，不阻断站点初始化；重复调用是幂等的。
		// 必须显式开启（bootstrap.Serve 在 InitWebsites 之前调用 EnableChannelManagerOnInit），
		// 否则测试与命令行工具会拿真实凭据去连平台并写真实租约。
		if channelManagerOnInit.Load() {
			startChannelManagerOnInit(&w)
		}
		// 通知投递工作者与服务入口同生命周期：只在进程入口显式打开开关后启动，
		// 测试/命令行不会自动跑后台投递。重复调用是幂等的。
		if notifyWorkerOnInit.Load() {
			startNotifyWorkerOnInit(&w)
		}
		w.NewMailService()
		// 初始化索引,异步处理
		go w.InitFulltext(false)
		// ai chat setting
		// 仅默认站点需要配置
		if w.Id == 1 {
			w.LoadAiSetting("")
		}
	} else {
		// 站点被置为未初始化（连库失败 / 状态停用 / 路径异常）：停掉残留管理器，
		// 否则它会继续用旧的 DB 句柄续租、重连。
		w.StopChannelManager()
		w.StopNotifyWorker()
	}
	// 到这里 limiter/cache/rewrite 都建好了，才允许对外服务。
	// 在"逐站点放行"之前，这段窗口被全局闸门挡着看不出来；现在挡不住了，
	// 提前放行就是把 Cache/Storage 还是 nil 的站点交给线上流量。
	if w.Initialed {
		markSiteServable(w.Id)
	}
}

func GetWebsites() []*Website {
	return websites.Values()
}

func CurrentSite(ctx iris.Context) *Website {
	if websites.Len() == 0 {
		return createDefaultWebsite(ctx)
	}

	if ctx == nil {
		defaultWebsite, _ := websites.Get(1)
		return defaultWebsite
	}

	// Try to get from siteId in context first
	if site, ok := getWebsiteFromContext(ctx); ok {
		return site
	}

	// Match by URI and host
	if site := matchWebsiteByRequest(ctx); site != nil {
		return site
	}

	// Fallback to default website
	if fallback := websites.MustGet(1); fallback != nil {
		return cloneWithContext(fallback, ctx)
	}
	// 注册表非空但没有站点 1：给一个空站点，不要 cloneWithContext(nil, ctx)。
	// 后者是 nil 解引用 —— recover 之后只剩一条栈，看不出是"站点 1 不在表里"。
	// 启动期（站点 1 还在迁移）与站点 1 被停用都会走到这里。
	return createDefaultWebsite(ctx)
}

// 创建默认网站配置
func createDefaultWebsite(ctx iris.Context) *Website {
	site := &Website{
		Id:         0,
		Initialed:  false,
		BaseURI:    "/",
		RootPath:   config.ExecPath,
		CachePath:  config.ExecPath + "cache" + string(filepath.Separator),
		DataPath:   config.ExecPath + "data" + string(filepath.Separator),
		PublicPath: config.ExecPath + "public" + string(filepath.Separator),
		System: &config.SystemConfig{
			// 新站点的默认站名。客户建站后第一眼看到的标题就是它，
			// 留官方名等于把品牌直接写到客户网站上。
			SiteName:     config.Brand().Name,
			TemplateName: "default",
		},
		ctx: ctx,
		Template: &StoreTemplates{
			Templates: make(map[string]int64),
			mu:        sync.Mutex{},
		},
		// 同上：尚未初始化的站点也要有非 nil 的 AI 写作配置。
		AiGenerateConfig: &config.AiGenerateConfig{},
	}
	if ctx != nil {
		// GetLocale 可能返回 nil（i18n 还没加载、或该请求没有匹配的语言）。
		// 直接 .Language() 是 nil 接口调用 → panic，recover 之后只剩栈。
		if locale := ctx.GetLocale(); locale != nil {
			site.backLanguage = locale.Language()
		}
	}
	return site
}

// 从上下文中获取并验证站点
func getWebsiteFromContext(ctx iris.Context) (*Website, bool) {
	siteId, err := ctx.Values().GetUint("siteId")
	if err != nil {
		return nil, false
	}

	w, ok := websites.Get(siteId)
	if !ok {
		return nil, false
	}

	return cloneWithContext(w, ctx), true
}

// 通过请求信息匹配网站
func matchWebsiteByRequest(ctx iris.Context) *Website {
	uri := ctx.RequestPath(false)
	host := library.GetHost(ctx)
	values := websites.Values()

	// 第一遍先处理二级目录的 baseUrl
	// 优先处理非根路径的情况
	if uri != "/" {
		if site := matchByURIAndHost(values, uri, host, ctx); site != nil {
			return site
		}
	}

	// 处理根路径和后备匹配
	return matchRootAndFallback(values, host, ctx)
}

// 克隆网站配置并添加上下文
func cloneWithContext(w *Website, ctx iris.Context) *Website {
	cloned := *w
	cloned.ctx = ctx
	if ctx != nil {
		// 同 createDefaultWebsite：GetLocale() 可能返回 nil，不能直接解引用。
		if locale := ctx.GetLocale(); locale != nil {
			cloned.backLanguage = locale.Language()
		}
	}
	return &cloned
}

// URI和主机匹配逻辑
func matchByURIAndHost(sites []*Website, uri, host string, ctx iris.Context) *Website {
	for _, w := range sites {
		if !w.Initialed {
			continue
		}
		// 检查所有相关URL配置
		for _, urlToCheck := range []string{w.System.BaseUrl, w.System.FrontUrl, w.System.MobileUrl, w.System.AdminUrl} {
			if urlToCheck == "" {
				continue
			}
			// 这里不处理根路径的 domain
			parsed, err := url.Parse(urlToCheck)
			if err != nil || parsed.RequestURI() == "/" {
				continue
			}

			// 这里匹配二级目录站点，包括多语言使用二级域名的部分
			if isHostMatch(parsed, host) && strings.HasPrefix(uri, parsed.RequestURI()) {
				return handleMatchedWebsite(w, parsed.RequestURI(), ctx)
			}
		}

		// 处理多语言逻辑
		if mainSite := handleMultiLanguage(w, host, uri, ctx); mainSite != nil {
			return mainSite
		}
	}
	return nil
}

// 根路径和后备匹配逻辑
func matchRootAndFallback(sites []*Website, host string, ctx iris.Context) *Website {
	for _, w := range sites {
		if !w.Initialed {
			continue
		}
		for _, urlToCheck := range []string{w.System.BaseUrl, w.System.FrontUrl, w.System.MobileUrl, w.System.AdminUrl} {
			if urlToCheck == "" {
				continue
			}
			// 这里不处理根路径的 domain
			parsed, err := url.Parse(urlToCheck)
			if err != nil {
				continue
			}

			// 这里匹配host
			if isHostMatch(parsed, host) {
				return handleHostMatch(w, ctx)
			}

			// 这里匹配子域名，模型、地点用到
			if w.PluginPlace.Open && w.PluginPlace.UrlType == config.PlaceUrlTypeSubdomain {
				var subdomain string
				var topDomain string
				subIndex := strings.Index(host, ".")
				if subIndex > 0 {
					// 获取子域名
					subdomain = host[:subIndex]
					topDomain = host[subIndex+1:]
				}
				hostName := parsed.Hostname()
				// host 不带点（localhost、纯 IP）时 subdomain 与 topDomain 都是空串，
				// 而 strings.HasSuffix(x, "") 恒为真 → 会拿空 token 去查城市站，
				// 一旦库里存在 url_token 为空的城市站记录，整站每个 URL（含首页）
				// 都会被当成城市站详情页。空值必须直接跳过。
				if subdomain != "" && topDomain != "" && strings.HasSuffix(hostName, topDomain) {
					// 同域名，继续处理
					place := w.GetPlaceFromCacheByToken(subdomain)
					if place != nil {
						// 模型、地点的子域名
						ctx.Values().Set("place", place)
						ctx.ViewData("place", place)

						return cloneWithContext(w, ctx)
					}
				}
			}
		}
	}
	return nil
}

// 处理多语言配置
func handleMultiLanguage(w *Website, host, uri string, ctx iris.Context) *Website {
	// 多语言只根据 baseUrl 来处理
	frontUrl := w.System.BaseUrl
	if w.System.FrontUrl != "" {
		frontUrl = w.System.FrontUrl
	}
	parsed, err := url.Parse(frontUrl)
	if err != nil || parsed.Hostname() != host {
		return nil
	}

	mainSite := w.GetMainWebsite()
	if !mainSite.MultiLanguage.Open {
		return nil
	}

	lang := detectLanguage(mainSite, ctx, host, uri)
	langSite := mainSite.MultiLanguage.GetSite(lang)
	if langSite != nil {
		ctx.SetLanguage(langSite.Language)
		// 如果是 single 形式的，就返回主站点
		if mainSite.MultiLanguage.SiteType == config.MultiLangSiteTypeSingle {
			ctx.Values().Set("siteId", mainSite.Id)
			return cloneWithContext(mainSite, ctx)
		} else {
			ctx.Values().Set("siteId", langSite.Id)
			if subSite, ok := websites.Get(langSite.Id); ok {
				return cloneWithContext(subSite, ctx)
			}
		}
	}
	return nil
}

// 检测语言设置
func detectLanguage(mainSite *Website, ctx iris.Context, host string, uri string) string {
	switch mainSite.MultiLanguage.Type {
	case config.MultiLangTypeDomain:
		// 匹配 domain
		for _, langSite := range mainSite.MultiLanguage.SubSites {
			parsed, err := url.Parse(langSite.BaseUrl)
			if err == nil && isHostMatch(parsed, host) {
				return langSite.Language
			}
		}
	case config.MultiLangTypeDirectory:
		lang := strings.SplitN(strings.TrimPrefix(uri, "/"), "/", 2)[0]
		if !strings.Contains(lang, ".") {
			return lang
		}
	case config.MultiLangTypeSame:
		if lang := ctx.GetCookie("lang"); lang != "" {
			return lang
		}
		return ctx.URLParam("lang")
	}
	return ""
}

// 主机匹配检查
func isHostMatch(parsed *url.URL, host string) bool {
	return parsed.Hostname() == host
}

// 子域名匹配检查
func isSubdomainMatch(parsedHost, currentHost string) bool {
	// 先移除 the first subdomain
	topDomain := strings.SplitN(parsedHost, ".", 2)[1]
	if strings.Count(topDomain, ".") == 1 {
		topDomain = parsedHost
	}
	if strings.HasSuffix(currentHost, "."+topDomain) {
		return true
	}

	return false
}

// 处理匹配到的网站
func handleMatchedWebsite(w *Website, baseURI string, ctx iris.Context) *Website {
	w.BaseURI = baseURI
	ctx.Values().Set("siteId", w.Id)
	return cloneWithContext(w, ctx)
}

// 处理主机匹配的情况
func handleHostMatch(w *Website, ctx iris.Context) *Website {
	// 处理多语言逻辑
	mainSite := w.GetMainWebsite()
	if mainSite.MultiLanguage.Open {
		// 采用目录形式
		var lang string
		if mainSite.MultiLanguage.Type == config.MultiLangTypeSame {
			// url不变
			lang = ctx.GetCookie("lang")
			if lang == "" {
				lang = ctx.URLParam("lang")
			}
		}
		langSite := mainSite.MultiLanguage.GetSite(lang)
		if langSite != nil {
			ctx.SetLanguage(langSite.Language)
			// 如果是 single 形式的，就返回主站点
			if mainSite.MultiLanguage.SiteType == config.MultiLangSiteTypeSingle {
				ctx.Values().Set("siteId", mainSite.Id)
				return cloneWithContext(mainSite, ctx)
			} else {
				ctx.Values().Set("siteId", langSite.Id)
				if subSite, ok := websites.Get(langSite.Id); ok {
					return cloneWithContext(subSite, ctx)
				}
			}
		}
	}
	ctx.Values().Set("siteId", w.Id)
	return cloneWithContext(w, ctx)
}

func CurrentSubSite(ctx iris.Context) *Website {
	currentSite := CurrentSite(ctx)
	// 多语言站点允许在多个站点中切换
	if currentSite.MultiLanguage.Open {
		tmpSiteId := ctx.GetHeader("Sub-Site-Id")
		if tmpSiteId != "" {
			siteId, _ := strconv.Atoi(tmpSiteId)
			if siteId > 0 {
				for i := range currentSite.MultiLanguage.SubSites {
					if currentSite.MultiLanguage.SubSites[i].Id == uint(siteId) {
						// 存在这样的子站点
						currentSite = GetWebsite(uint(siteId))
						break
					}
				}
			}
		}
	}

	return currentSite
}

// GetWebsite default 1
func GetWebsite(siteId uint) *Website {
	website, _ := websites.Get(siteId)

	return website
}

func RemoveWebsite(siteId uint, removeFile bool) {
	// 删除前先停掉该站点的渠道长连接与续租：站点从注册表摘除后，
	// 管理器仍绑着已失效的 *Website/DB 句柄，会继续连平台并写租约。
	// 通知工作者同理：它每 30 秒扫一次表并 UPDATE 投递行，站点删掉后仍在跑就是空转。
	// 注意：下面的 GetWebsite 在 Delete 之后返回 nil 是既有行为（DROP DATABASE
	// 因此不会执行），本任务不改变它，只加停止逻辑。
	if stopped := GetWebsite(siteId); stopped != nil {
		stopped.StopChannelManager()
		stopped.StopNotifyWorker()
	}
	websites.Delete(siteId)
	unmarkSiteServable(siteId)
	site := GetWebsite(siteId)
	if site != nil {
		if removeFile {
			// 仅删除模板和public目录
			_ = os.RemoveAll(site.RootPath + "template")
			_ = os.RemoveAll(site.RootPath + "public")
			// 删除数据库
			if site.DB != nil {
				site.DB.Exec("DROP DATABASE `?`", gorm.Expr(site.Mysql.Database))
			}
		}
	}
}

func (w *Website) GetTemplateDir() string {
	if w == nil {
		return config.ExecPath + "template" + string(filepath.Separator) + "default"
	}
	// System 为 nil 表示站点半初始化（库连不上 → InitSetting 未执行）。
	// 这里不能取 TemplateName：裸解引用会 panic，而调用方（备份、模板读写、
	// 缓存清理）大多拿不到 recover 兜底，崩的是整个请求甚至进程。
	if w.System == nil {
		return w.RootPath + "template" + string(filepath.Separator) + "default"
	}
	if len(w.System.TemplateName) == 0 {
		w.System.TemplateName = "default"
	}
	return w.RootPath + "template" + string(filepath.Separator) + w.System.TemplateName
}

func GetDBWebsites(name, baseUrl string, page, pageSize int) ([]*model.Website, int64) {
	var sites []*model.Website
	db := GetDefaultDB()
	if db == nil {
		return nil, 0
	}
	var total int64
	offset := (page - 1) * pageSize
	tx := db.Model(&model.Website{}).Order("id asc")
	if name != "" {
		tx = tx.Where("`name` LIKE ?", "%"+name+"%")
	}
	if baseUrl != "" {
		ids := websites.Keys(baseUrl)
		tx = tx.Where("id in (?)", ids)
	}
	tx.Count(&total).Limit(pageSize).Offset(offset).Find(&sites)
	// 逐行用运行时注册表补全 Language / BaseUrl。
	//
	// ⚠️ 这里曾写成 `sites[0].Mysql = config.Server.Mysql`，是**位置判断**而不是
	// 身份判断：它命中每一页的第一行。翻到第 2 页时，第 21 号站点会被改写成
	// 「用的是默认库 anqicms / use_default=false」，而它真实配置是 aitool（实测）。
	// 站点 1 的库信息由 handler 按 `site.Id == 1` 补，不需要在这里做。
	for i := range sites {
		applySiteRuntimeInfo(sites[i], GetWebsite(sites[i].Id))
	}

	return sites, total
}

// applySiteRuntimeInfo 用运行时注册表里的站点补全数据库行的展示字段。
//
// 为什么必须容错：websites 注册表里可能存在**半初始化**的站点。
// InitWebsite 先 `websites.Set(&w)`，之后才 `w.InitSetting()`，而
// InitSetting 在 `w.DB == nil` 时直接 return —— 于是「库连不上」的站点
// 会以 `System == nil` 的状态长期留在注册表里。
//
// 直接 `currentSite.System.Language` 取字段就是空指针 panic，被 recover
// 兜成 500「服务器内部错误」。因为 panic 只发生在包含该行的分页上，
// 表现是「第 1 页正常、翻到某一页必 500」——极难从现象反推到数据。
// 这里的语义：拿不到运行时配置就退化成数据库里的原始值（Status 归 0 表示不可用）。
func applySiteRuntimeInfo(site *model.Website, currentSite *Website) {
	if site == nil {
		return
	}
	if currentSite == nil {
		// 未注册（如直接写库造出来的行）：保留库里读到的字段，只标记不可用。
		site.Status = 0
		return
	}
	if currentSite.System != nil {
		site.Language = currentSite.System.Language
		site.BaseUrl = currentSite.System.BaseUrl
	}
	if !currentSite.Initialed {
		site.Status = 0
	}
}

func GetDBWebsiteInfo(id uint) (*model.Website, error) {
	db := GetDefaultDB()
	if db == nil {
		return nil, errors.New("please initialize the database first")
	}
	var website model.Website
	err := db.Where("`id` = ?", id).Take(&website).Error
	if err != nil {
		return nil, err
	}
	// 与列表走同一套降级逻辑：`System` 可能为 nil（半初始化站点），
	// 裸取 currentSite.System.Language 会让详情接口同样 500。
	applySiteRuntimeInfo(&website, GetWebsite(website.Id))

	return &website, nil
}

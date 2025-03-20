package provider

import (
	"context"
	"errors"
	"net/url"
	"os"
	"strconv"
	"strings"
	"sync"

	"github.com/esap/wechat"
	"github.com/kataras/iris/v12"
	"github.com/kataras/iris/v12/i18n"
	"github.com/medivhzhan/weapp/v3"
	"gorm.io/gorm"
	"kandaoni.com/anqicms/config"
	"kandaoni.com/anqicms/library"
	"kandaoni.com/anqicms/model"
	"kandaoni.com/anqicms/provider/fulltext"
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
	Host                    string
	BaseURI                 string
	RootPath                string
	DataPath                string
	CachePath               string
	PublicPath              string
	DB                      *gorm.DB
	StatisticLog            *StatisticLog
	Storage                 *BucketStorage
	CacheStorage            *BucketStorage
	parsedPatten            *RewritePatten
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

	System  *config.SystemConfig
	Content *config.ContentConfig
	Index   *config.IndexConfig
	Contact *config.ContactConfig
	Safe    *config.SafeConfig
	Banner  *config.BannerConfig
	//plugin
	PluginPush         *config.PluginPushConfig
	PluginSitemap      *config.PluginSitemapConfig
	PluginRewrite      *config.PluginRewriteConfig
	PluginAnchor       *config.PluginAnchorConfig
	PluginGuestbook    *config.PluginGuestbookConfig
	PluginUploadFiles  []config.PluginUploadFile
	PluginSendmail     *config.PluginSendmail
	PluginImportApi    *config.PluginImportApiConfig
	PluginStorage      *config.PluginStorageConfig
	PluginPay          *config.PluginPayConfig
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
}

type StoreTemplates struct {
	Templates map[string]int64
	mu        sync.Mutex
}

func (w *Website) SetTemplates(templates map[string]int64) {
	if w.Template == nil {
		return
	}
	w.Template.mu.Lock()
	defer w.Template.mu.Unlock()
	w.Template.Templates = templates
}

func (w *Website) TemplateExist(tplPaths ...string) (string, bool) {
	if len(tplPaths) == 0 {
		return "", false
	}
	if w.Template == nil {
		return tplPaths[0], false
	}
	w.Template.mu.Lock()
	defer w.Template.mu.Unlock()
	for _, tplPath := range tplPaths {
		if tplPath == "" {
			continue
		}
		if _, ok := w.Template.Templates[tplPath]; ok {
			return tplPath, true
		}
	}

	return tplPaths[0], false
}

func (w *Website) Ctx() context.Context {
	if w.ctx == nil {
		return context.TODO()
	}
	return w.ctx.Request().Context()
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

	values := make([]*Website, len(ow.keys))
	for i, id := range ow.keys {
		values[i] = ow.data[id]
	}
	return values
}

var websites = NewWebsites()

func InitWebsites() {
	// 先需要获得 default db
	db := GetDefaultDB()
	if db == nil {
		return
	}
	// 如果是从旧版升级上来的，需要先创建website表，并注入相关信息
	if !db.Migrator().HasTable("websites") {
		db.AutoMigrate(&model.Website{})
	}
	defaultSite := model.Website{
		Model:    model.Model{Id: 1},
		RootPath: config.ExecPath,
		Name:     "Default Website",
		Status:   1,
	}
	db.Where("`id` = 1").FirstOrCreate(&defaultSite)
	var sites []*model.Website
	db.Order("`id` asc").Find(&sites)
	for _, v := range sites {
		InitWebsite(v)
	}
	// 检查多语言站点
	values := websites.Values()
	for _, w := range values {
		if w.MultiLanguage.Open {
			if w.MultiLanguage.SiteType == config.MultiLangSiteTypeMulti {
				// 读取子站点
				multiLangSites := w.GetMultiLangSites(w.Id, false)
				w.MultiLanguage.SubSites = make([]config.MultiLangSite, 0, len(multiLangSites)*2)
				mainBaseUrl := w.System.BaseUrl
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
		db, err = InitDB(&mw.Mysql)
	}
	if !strings.HasSuffix(mw.RootPath, "/") {
		mw.RootPath = mw.RootPath + "/"
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
		BaseURI:      "/",
		RootPath:     mw.RootPath,
		CachePath:    mw.RootPath + "cache/",
		DataPath:     mw.RootPath + "data/",
		PublicPath:   mw.RootPath + "public/",
		backLanguage: lang,
		Template: &StoreTemplates{
			Templates: make(map[string]int64),
			mu:        sync.Mutex{},
		},
		cachedTodayArticleCount: &response.CacheArticleCount{},
	}
	if db != nil && mw.Status == 1 {
		// 读取真正的 TokenSecret

		w.Initialed = true
	}
	if db == nil {
		w.ErrorMsg = w.Tr("DatabaseConnectionFailed")
		if err != nil {
			w.ErrorMsg = "：" + err.Error()
		}
	}
	// 先判断目录是否存在。对于迁移的站点，这个地方可能是会出错的
	_, err = os.Stat(mw.RootPath)
	if err != nil {
		w.Initialed = false
		w.ErrorMsg = w.Tr("SitePathError:") + err.Error()
	}
	if mw.Id == 1 {
		w.Mysql = &config.Server.Mysql
	}
	websites.Set(&w)
	if db != nil {
		var lastVersion string
		db.Model(&model.Setting{}).Where("`key` = ?", LastRunVersionKey).Pluck("value", &lastVersion)
		if lastVersion != "" {
			go AutoMigrateDB(db, false)
		} else {
			_ = AutoMigrateDB(db, false)
		}
		w.InitSetting()
		w.InitModelData()
		// fix BaseUri
		parsed, err := url.Parse(w.System.BaseUrl)
		if err == nil {
			if parsed.RequestURI() != "/" {
				w.BaseURI = parsed.RequestURI()
			}
			w.Host = parsed.Host
		}
	}
	if w.Initialed {
		w.GetRewritePatten(true)
		// 启动限流器
		w.InitLimiter()
		w.InitStatistic()
		w.InitBucket()
		w.InitCacheBucket()
		w.InitCache()
		// 初始化索引,异步处理
		go w.InitFulltext(false)
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
	return cloneWithContext(websites.MustGet(1), ctx)
}

// 创建默认网站配置
func createDefaultWebsite(ctx iris.Context) *Website {
	site := &Website{
		Id:         0,
		Initialed:  false,
		BaseURI:    "/",
		RootPath:   config.ExecPath,
		CachePath:  config.ExecPath + "cache/",
		DataPath:   config.ExecPath + "data/",
		PublicPath: config.ExecPath + "public/",
		System: &config.SystemConfig{
			SiteName:     "AnQiCMS",
			TemplateName: "default",
		},
		ctx: ctx,
		Template: &StoreTemplates{
			Templates: make(map[string]int64),
			mu:        sync.Mutex{},
		},
	}
	if ctx != nil {
		site.backLanguage = ctx.GetLocale().Language()
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
		cloned.backLanguage = ctx.GetLocale().Language()
	}
	return &cloned
}

// URI和主机匹配逻辑
func matchByURIAndHost(sites []*Website, uri, host string, ctx iris.Context) *Website {
	for _, w := range sites {
		// 检查所有相关URL配置
		for _, urlToCheck := range []string{w.System.BaseUrl, w.System.MobileUrl, w.System.AdminUrl} {
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
		for _, urlToCheck := range []string{w.System.BaseUrl, w.System.MobileUrl, w.System.AdminUrl} {
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

			if isSubdomainMatch(parsed.Hostname(), host) {
				return cloneWithContext(w, ctx)
			}
		}
	}
	return nil
}

// 处理多语言配置
func handleMultiLanguage(w *Website, host, uri string, ctx iris.Context) *Website {
	// 多语言只根据 baseUrl 来处理
	parsed, err := url.Parse(w.System.BaseUrl)
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
	return strings.HasSuffix(parsedHost, "."+currentHost)
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

func CurrentSite2(ctx iris.Context) *Website {
	if websites.Len() == 0 {
		cur := &Website{
			Id:         0,
			Initialed:  false,
			BaseURI:    "/",
			RootPath:   config.ExecPath,
			CachePath:  config.ExecPath + "cache/",
			DataPath:   config.ExecPath + "data/",
			PublicPath: config.ExecPath + "public/",
			System: &config.SystemConfig{
				SiteName:     "AnQiCMS",
				TemplateName: "default",
			},
			Template: &StoreTemplates{
				Templates: make(map[string]int64),
				mu:        sync.Mutex{},
			},
		}
		if ctx != nil {
			cur.backLanguage = ctx.GetLocale().Language()
		}
		cur.ctx = ctx
		return cur
	}
	if ctx != nil {
		var tmpSite Website
		if siteId, err := ctx.Values().GetUint("siteId"); err == nil {
			w, ok := websites.Get(siteId)
			if ok {
				tmpSite = *w
				tmpSite.backLanguage = ctx.GetLocale().Language()
				tmpSite.ctx = ctx
				return &tmpSite
			}
		}
		// 获取到当前website
		uri := ctx.RequestPath(false)
		host := library.GetHost(ctx)
		// check exist uri first
		values := websites.Values()
		if uri != "/" {
			for _, w := range values {
				parsed, err := url.Parse(w.System.BaseUrl)
				if err != nil {
					continue
				}
				// 多语言站点处理，先确定主域名
				if parsed.Hostname() == host {
					// 得到了siteId
					mainSite := w.GetMainWebsite()
					// 匹配并解析语言
					if mainSite.MultiLanguage.Open {
						// 采用目录形式
						var lang string
						if mainSite.MultiLanguage.Type == config.MultiLangTypeDirectory {
							lang = strings.SplitN(strings.TrimPrefix(uri, "/"), "/", 2)[0]
							if strings.Contains(lang, ".") {
								// 忽略
								lang = ""
							}
						} else if mainSite.MultiLanguage.Type == config.MultiLangTypeSame {
							// url不变
							lang = ctx.GetCookie("lang")
							if lang == "" {
								lang = ctx.URLParam("lang")
							}
						}
						langSite := mainSite.MultiLanguage.GetSite(lang)
						if langSite != nil {
							ctx.SetLanguage(lang)
							ctx.Values().Set("siteId", langSite.Id)
							tmpSite1, _ := websites.Get(langSite.Id)
							tmpSite = *tmpSite1
							tmpSite.backLanguage = ctx.GetLocale().Language()
							tmpSite.ctx = ctx
							return &tmpSite
						}
					}
					ctx.Values().Set("siteId", w.Id)
					tmpSite = *w
					tmpSite.backLanguage = ctx.GetLocale().Language()
					tmpSite.ctx = ctx
					return &tmpSite
				}
				// end
				if parsed.RequestURI() != "/" {
					if parsed.Hostname() == host && strings.HasPrefix(uri, parsed.RequestURI()) {
						w.BaseURI = parsed.RequestURI()
						ctx.Values().Set("siteId", w.Id)
						tmpSite = *w
						tmpSite.backLanguage = ctx.GetLocale().Language()
						tmpSite.ctx = ctx
						return &tmpSite
					}
				}
				if w.System.MobileUrl != "" {
					parsed, err = url.Parse(w.System.MobileUrl)
					if err == nil {
						if parsed.RequestURI() != "/" {
							if parsed.Hostname() == host && strings.HasPrefix(uri, parsed.RequestURI()) {
								w.BaseURI = parsed.RequestURI()
								ctx.Values().Set("siteId", w.Id)
								tmpSite = *w
								tmpSite.backLanguage = ctx.GetLocale().Language()
								tmpSite.ctx = ctx
								return &tmpSite
							}
						}
					}
				}
				if w.System.AdminUrl != "" {
					parsed, err = url.Parse(w.System.AdminUrl)
					if err == nil {
						if parsed.Hostname() == host && strings.HasPrefix(uri, parsed.RequestURI()) {
							w.BaseURI = parsed.RequestURI()
							ctx.Values().Set("siteId", w.Id)
							tmpSite = *w
							tmpSite.backLanguage = ctx.GetLocale().Language()
							tmpSite.ctx = ctx
							return &tmpSite
						}
					}
				}
			}
		}
		for _, w := range values {
			// 判断内容，base_url,mobile_url,admin_url
			parsed, err := url.Parse(w.System.BaseUrl)
			if err == nil {
				if parsed.RequestURI() != "/" {
					continue
				}
				if parsed.Hostname() == host {
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
							ctx.SetLanguage(lang)
							ctx.Values().Set("siteId", langSite.Id)
							tmpSite1, _ := websites.Get(langSite.Id)
							tmpSite = *tmpSite1
							tmpSite.backLanguage = ctx.GetLocale().Language()
							tmpSite.ctx = ctx
							return &tmpSite
						}
					}

					ctx.Values().Set("siteId", w.Id)
					tmpSite = *w
					tmpSite.backLanguage = ctx.GetLocale().Language()
					tmpSite.ctx = ctx
					return &tmpSite
				}
				// 顶级域名
				if strings.HasSuffix(parsed.Hostname(), "."+host) {
					ctx.Values().Set("siteId", w.Id)
					tmpSite = *w
					tmpSite.backLanguage = ctx.GetLocale().Language()
					tmpSite.ctx = ctx
					return &tmpSite
				}
			}
			if w.System.MobileUrl != "" {
				parsed, err = url.Parse(w.System.MobileUrl)
				if err == nil {
					if parsed.RequestURI() != "/" {
						continue
					}
					if parsed.Hostname() == host {
						ctx.Values().Set("siteId", w.Id)
						tmpSite = *w
						tmpSite.backLanguage = ctx.GetLocale().Language()
						tmpSite.ctx = ctx
						return &tmpSite
					}
				}
			}
			if w.System.AdminUrl != "" {
				parsed, err = url.Parse(w.System.AdminUrl)
				if err == nil {
					if parsed.RequestURI() != "/" {
						continue
					}
					if parsed.Hostname() == host {
						ctx.Values().Set("siteId", w.Id)
						tmpSite = *w
						tmpSite.backLanguage = ctx.GetLocale().Language()
						tmpSite.ctx = ctx
						return &tmpSite
					}
				}
			}
		}
	}

	// return default 1
	defaultWebsite, _ := websites.Get(1)
	if ctx != nil {
		tmpSite := *defaultWebsite
		tmpSite.backLanguage = ctx.GetLocale().Language()
		tmpSite.ctx = ctx
		return &tmpSite
	}
	return defaultWebsite
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
	websites.Delete(siteId)
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
		return config.ExecPath + "template/default"
	}
	if len(w.System.TemplateName) == 0 {
		w.System.TemplateName = "default"
	}
	return w.RootPath + "template/" + w.System.TemplateName
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
	if len(sites) > 0 {
		sites[0].Mysql = config.Server.Mysql
		for i := range sites {
			currentSite := GetWebsite(sites[i].Id)
			if currentSite != nil {
				sites[i].Language = currentSite.System.Language
				sites[i].BaseUrl = currentSite.System.BaseUrl
				if !currentSite.Initialed {
					sites[i].Status = 0
				}
			} else {
				sites[i].Status = 0
			}
		}
	}

	return sites, total
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
	currentSite := GetWebsite(website.Id)
	if currentSite != nil {
		website.Language = currentSite.System.Language
		website.BaseUrl = currentSite.System.BaseUrl
		if !currentSite.Initialed {
			website.Status = 0
		}
	} else {
		website.Status = 0
	}

	return &website, nil
}

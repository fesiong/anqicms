package provider

import (
	"errors"
	"github.com/esap/wechat"
	"github.com/huichen/wukong/engine"
	"github.com/kataras/iris/v12"
	"github.com/kataras/iris/v12/i18n"
	"github.com/medivhzhan/weapp/v3"
	"gorm.io/gorm"
	"kandaoni.com/anqicms/config"
	"kandaoni.com/anqicms/library"
	"kandaoni.com/anqicms/model"
	"kandaoni.com/anqicms/response"
	"net/url"
	"os"
	"strconv"
	"strings"
	"sync"
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
	searcher                *engine.Engine
	fulltextStatus          int // 0 未启用，1初始化中，2 初始化完成
	cachedTodayArticleCount response.CacheArticleCount
	transferWebsite         *TransferWebsite
	weappClient             *weapp.Client
	wechatServer            *wechat.Server
	Cache                   library.Cache
	HtmlCacheStatus         *HtmlCacheStatus
	HtmlCachePushStatus     *HtmlCacheStatus
	quickImportStatus       *QuickImportArchive

	System  config.SystemConfig
	Content config.ContentConfig
	Index   config.IndexConfig
	Contact config.ContactConfig
	Safe    config.SafeConfig
	Banner  config.BannerConfig
	//plugin
	PluginPush         config.PluginPushConfig
	PluginSitemap      config.PluginSitemapConfig
	PluginRewrite      config.PluginRewriteConfig
	PluginAnchor       config.PluginAnchorConfig
	PluginGuestbook    config.PluginGuestbookConfig
	PluginUploadFiles  []config.PluginUploadFile
	PluginSendmail     config.PluginSendmail
	PluginImportApi    config.PluginImportApiConfig
	PluginStorage      config.PluginStorageConfig
	PluginPay          config.PluginPayConfig
	PluginWeapp        config.PluginWeappConfig
	PluginWechat       config.PluginWeappConfig
	PluginRetailer     config.PluginRetailerConfig
	PluginUser         config.PluginUserConfig
	PluginOrder        config.PluginOrderConfig
	PluginFulltext     config.PluginFulltextConfig
	PluginTitleImage   config.PluginTitleImageConfig
	PluginWatermark    config.PluginWatermark
	PluginHtmlCache    config.PluginHtmlCache
	SensitiveWords     []string
	AiGenerateConfig   config.AiGenerateConfig
	PluginInterference config.PluginInterference
	PluginTimeFactor   config.PluginTimeFactor
	MultiLanguage      config.PluginMultiLangConfig
	PluginTranslate    config.PluginTranslateConfig
	PluginJsonLd       config.PluginJsonLdConfig

	CollectorConfig config.CollectorJson
	KeywordConfig   config.KeywordJson
	Proxy           *ProxyIPs // 代理

	FindPasswordInfo *response.FindPasswordInfo
	Limiter          *Limiter
	TplI18n          *i18n.I18n
	// 一些缓存内容
	languages    map[string]string
	backLanguage string
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
			w.MultiLanguage.SubSites = map[string]uint{}
			// 读取子站点
			multiLangSites := w.GetMultiLangSites(w.Id, false)
			mainBaseUrl := w.System.BaseUrl
			for _, v := range multiLangSites {
				w.MultiLanguage.SubSites[v.Language] = v.Id
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
						if curSite.PluginStorage.StorageType == config.StorageTypeLocal {
							curSite.PluginStorage.StorageUrl = baseUrl
						}
						curSite.System.BaseUrl = baseUrl
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
		// 启动限流器
		w.InitLimiter()
		w.InitStatistic()
		w.InitBucket()
		w.InitCacheBucket()
		w.InitCache()
		// 初始化索引,异步处理
		go w.InitFulltext()
	}
}

func GetWebsites() []*Website {
	return websites.Values()
}

func CurrentSite(ctx iris.Context) *Website {
	if websites.Len() == 0 {
		cur := &Website{
			Id:         0,
			Initialed:  false,
			BaseURI:    "/",
			RootPath:   config.ExecPath,
			CachePath:  config.ExecPath + "cache/",
			DataPath:   config.ExecPath + "data/",
			PublicPath: config.ExecPath + "public/",
			System: config.SystemConfig{
				SiteName:     "AnQiCMS",
				TemplateName: "default",
			},
		}
		if ctx != nil {
			cur.backLanguage = ctx.GetLocale().Language()
		}
		return cur
	}
	if ctx != nil {
		if siteId, err := ctx.Values().GetUint("siteId"); err == nil {
			w, ok := websites.Get(siteId)
			if ok {
				if w.backLanguage == "" {
					w.backLanguage = ctx.GetLocale().Language()
				}
				return w
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
						if tmpId, ok := mainSite.MultiLanguage.SubSites[lang]; ok {
							ctx.SetLanguage(lang)
							ctx.Values().Set("siteId", tmpId)
							tmpSite, _ := websites.Get(tmpId)
							tmpSite.backLanguage = ctx.GetLocale().Language()
							return tmpSite
						}
					}
					ctx.Values().Set("siteId", w.Id)
					if w.backLanguage == "" {
						w.backLanguage = ctx.GetLocale().Language()
					}
					return w
				}
				// end
				if parsed.RequestURI() != "/" {
					if parsed.Hostname() == host && strings.HasPrefix(uri, parsed.RequestURI()) {
						w.BaseURI = parsed.RequestURI()
						ctx.Values().Set("siteId", w.Id)
						if w.backLanguage == "" {
							w.backLanguage = ctx.GetLocale().Language()
						}
						return w
					}
				}
				if w.System.MobileUrl != "" {
					parsed, err = url.Parse(w.System.MobileUrl)
					if err == nil {
						if parsed.RequestURI() != "/" {
							if parsed.Hostname() == host && strings.HasPrefix(uri, parsed.RequestURI()) {
								w.BaseURI = parsed.RequestURI()
								ctx.Values().Set("siteId", w.Id)
								if w.backLanguage == "" {
									w.backLanguage = ctx.GetLocale().Language()
								}
								return w
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
							if w.backLanguage == "" {
								w.backLanguage = ctx.GetLocale().Language()
							}
							return w
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
						if tmpId, ok := mainSite.MultiLanguage.SubSites[lang]; ok {
							ctx.SetLanguage(lang)
							ctx.Values().Set("siteId", tmpId)
							tmpSite, _ := websites.Get(tmpId)
							tmpSite.backLanguage = ctx.GetLocale().Language()
							return tmpSite
						}
					}

					ctx.Values().Set("siteId", w.Id)
					if w.backLanguage == "" {
						w.backLanguage = ctx.GetLocale().Language()
					}
					return w
				}
				// 顶级域名
				if strings.HasSuffix(parsed.Hostname(), "."+host) {
					ctx.Values().Set("siteId", w.Id)
					if w.backLanguage == "" {
						w.backLanguage = ctx.GetLocale().Language()
					}
					return w
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
						if w.backLanguage == "" {
							w.backLanguage = ctx.GetLocale().Language()
						}
						return w
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
						if w.backLanguage == "" {
							w.backLanguage = ctx.GetLocale().Language()
						}
						return w
					}
				}
			}
		}
	}

	// return default 1
	defaultWebsite, _ := websites.Get(1)
	if ctx != nil {
		defaultWebsite.backLanguage = ctx.GetLocale().Language()
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
				for _, subId := range currentSite.MultiLanguage.SubSites {
					if subId == uint(siteId) {
						// 存在这样的子站点
						currentSite = GetWebsite(subId)
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

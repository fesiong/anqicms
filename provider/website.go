package provider

import (
	"errors"
	"github.com/esap/wechat"
	"github.com/huichen/wukong/engine"
	"github.com/kataras/iris/v12"
	"github.com/medivhzhan/weapp/v3"
	"gorm.io/gorm"
	"kandaoni.com/anqicms/config"
	"kandaoni.com/anqicms/library"
	"kandaoni.com/anqicms/model"
	"kandaoni.com/anqicms/response"
	"net/url"
	"os"
	"strings"
)

type Website struct {
	Id                      uint
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
	Storage                 *BucketStorage
	CacheStorage            *BucketStorage
	parsedPatten            *RewritePatten
	searcher                *engine.Engine
	fulltextStatus          int // 0 未启用，1初始化中，2 初始化完成
	cachedTodayArticleCount response.CacheArticleCount
	transferWebsite         *TransferWebsite
	weappClient             *weapp.Client
	wechatServer            *wechat.Server
	CachedStatistics        *response.Statistics
	AdminLoginError         response.LoginError
	Cache                   library.Cache
	HtmlCacheStatus         *HtmlCacheStatus
	HtmlCachePushStatus     *HtmlCacheStatus

	System  config.SystemConfig  `json:"system"`
	Content config.ContentConfig `json:"content"`
	Index   config.IndexConfig   `json:"index"`
	Contact config.ContactConfig `json:"contact"`
	Safe    config.SafeConfig    `json:"safe"`
	Banner  config.BannerConfig  `json:"banner"`
	//plugin
	PluginPush         config.PluginPushConfig       `json:"plugin_push"`
	PluginSitemap      config.PluginSitemapConfig    `json:"plugin_sitemap"`
	PluginRewrite      config.PluginRewriteConfig    `json:"plugin_rewrite"`
	PluginAnchor       config.PluginAnchorConfig     `json:"plugin_anchor"`
	PluginGuestbook    config.PluginGuestbookConfig  `json:"plugin_guestbook"`
	PluginUploadFiles  []config.PluginUploadFile     `json:"plugin_upload_file"`
	PluginSendmail     config.PluginSendmail         `json:"plugin_sendmail"`
	PluginImportApi    config.PluginImportApiConfig  `json:"plugin_import_api"`
	PluginStorage      config.PluginStorageConfig    `json:"plugin_storage"`
	PluginPay          config.PluginPayConfig        `json:"plugin_pay"`
	PluginWeapp        config.PluginWeappConfig      `json:"plugin_weapp"`
	PluginWechat       config.PluginWeappConfig      `json:"plugin_wechat"`
	PluginRetailer     config.PluginRetailerConfig   `json:"plugin_retailer"`
	PluginUser         config.PluginUserConfig       `json:"plugin_user"`
	PluginOrder        config.PluginOrderConfig      `json:"plugin_order"`
	PluginFulltext     config.PluginFulltextConfig   `json:"plugin_fulltext"`
	PluginTitleImage   config.PluginTitleImageConfig `json:"plugin_title_image"`
	PluginHtmlCache    config.PluginHtmlCache        `json:"plugin_html_cache"`
	SensitiveWords     []string                      `json:"sensitive_words"`
	AiGenerateConfig   config.AiGenerateConfig       `json:"ai_generate_config"`
	PluginInterference config.PluginInterference     `json:"plugin_interference"`

	CollectorConfig config.CollectorJson
	KeywordConfig   config.KeywordJson

	FindPasswordInfo *response.FindPasswordInfo

	// 一些缓存内容
	languages map[string]string
}

var websites = map[uint]*Website{}

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
		Name:     "默认站点",
		Status:   1,
	}
	db.Where("`id` = 1").FirstOrCreate(&defaultSite)
	var sites []*model.Website
	db.Order("`id` asc").Find(&sites)
	for _, v := range sites {
		InitWebsite(v)
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
	w := Website{
		Id:          mw.Id,
		TokenSecret: config.GenerateRandString(32),
		Mysql:       &mw.Mysql,
		DB:          db,
		BaseURI:     "/",
		RootPath:    mw.RootPath,
		CachePath:   mw.RootPath + "cache/",
		DataPath:    mw.RootPath + "data/",
		PublicPath:  mw.RootPath + "public/",
	}
	if db != nil && mw.Status == 1 {
		w.Initialed = true
	}
	if db == nil {
		w.ErrorMsg = "数据库连接失败"
		if err != nil {
			w.ErrorMsg = "：" + err.Error()
		}
	}
	// 先判断目录是否存在。对于迁移的站点，这个地方可能是会出错的
	_, err = os.Stat(mw.RootPath)
	if err != nil {
		w.Initialed = false
		w.ErrorMsg = "站点路径错误：" + err.Error()
	}
	if mw.Id == 1 {
		w.Mysql = &config.Server.Mysql
	}
	websites[mw.Id] = &w
	if db != nil {
		_ = AutoMigrateDB(db, false)
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
		w.InitBucket()
		w.InitCacheBucket()
		w.InitCache()
		// 初始化索引,异步处理
		go w.InitFulltext()
	}
}

func GetWebsites() map[uint]*Website {
	return websites
}

func CurrentSite(ctx iris.Context) *Website {
	if len(websites) == 0 {
		return &Website{
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
	}
	if ctx != nil {
		if siteId, err := ctx.Values().GetUint("siteId"); err == nil {
			w, ok := websites[siteId]
			if ok {
				return w
			}
		}
		// 获取到当前website
		uri := ctx.RequestPath(false)
		host := library.GetHost(ctx)
		// check exist uri first
		if uri != "/" {
			for _, w := range websites {
				parsed, err := url.Parse(w.System.BaseUrl)
				if err != nil {
					continue
				}
				if parsed.RequestURI() != "/" {
					if parsed.Hostname() == host && strings.HasPrefix(uri, parsed.RequestURI()) {
						w.BaseURI = parsed.RequestURI()
						ctx.Values().Set("siteId", w.Id)
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
							return w
						}
					}
				}
			}
		}
		for _, w := range websites {
			// 判断内容，base_url,mobile_url,admin_url
			parsed, err := url.Parse(w.System.BaseUrl)
			if parsed.RequestURI() != "/" {
				continue
			}
			if err == nil && parsed.Hostname() == host {
				ctx.Values().Set("siteId", w.Id)
				return w
			}
			if w.System.MobileUrl != "" {
				parsed, err = url.Parse(w.System.MobileUrl)
				if err == nil {
					if parsed.RequestURI() != "/" {
						continue
					}
					if parsed.Hostname() == host {
						ctx.Values().Set("siteId", w.Id)
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
						return w
					}
				}
			}
		}
	}

	// return default 1
	return websites[1]
}

// GetWebsite default 1
func GetWebsite(siteId uint) *Website {
	website, _ := websites[siteId]

	return website
}

func RemoveWebsite(siteId uint, removeFile bool) {
	defer delete(websites, siteId)
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

func GetDBWebsites(page, pageSize int) ([]*model.Website, int64) {
	var sites []*model.Website
	db := GetDefaultDB()
	if db == nil {
		return nil, 0
	}
	var total int64
	offset := (page - 1) * pageSize
	tx := db.Model(&model.Website{}).Order("id asc")
	tx.Count(&total).Limit(pageSize).Offset(offset).Find(&sites)
	if len(sites) > 0 {
		sites[0].Mysql = config.Server.Mysql
		for i := range sites {
			currentSite := GetWebsite(sites[i].Id)
			if currentSite != nil {
				sites[i].BaseUrl = currentSite.System.BaseUrl
				sites[i].ErrorMsg = currentSite.ErrorMsg
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
		return nil, errors.New("未安装数据库")
	}
	var website model.Website
	err := db.Where("`id` = ?", id).Take(&website).Error
	if err != nil {
		return nil, err
	}

	return &website, nil
}

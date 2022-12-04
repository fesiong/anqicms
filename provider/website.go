package provider

import (
	"errors"
	"github.com/esap/wechat"
	"github.com/huichen/wukong/engine"
	"github.com/kataras/iris/v12"
	"github.com/medivhzhan/weapp/v3"
	"gorm.io/gorm"
	"kandaoni.com/anqicms/config"
	"kandaoni.com/anqicms/model"
	"kandaoni.com/anqicms/response"
	"net/url"
)

type Website struct {
	Id                      uint
	Mysql                   *config.MysqlConfig
	Initialed               bool
	RootPath                string
	DataPath                string
	CachePath               string
	PublicPath              string
	DB                      *gorm.DB
	Storage                 *BucketStorage
	parsedPatten            *RewritePatten
	searcher                *engine.Engine
	fulltextStatus          int // 0 未启用，1初始化中，2 初始化完成
	cachedTodayArticleCount response.CacheArticleCount
	transferWebsite         *TransferWebsite
	weappClient             *weapp.Client
	wechatServer            *wechat.Server
	CachedStatistics        *response.Statistics
	AdminLoginError         response.LoginError
	MemCache                *memCache

	System  config.SystemConfig  `json:"system"`
	Content config.ContentConfig `json:"content"`
	Index   config.IndexConfig   `json:"index"`
	Contact config.ContactConfig `json:"contact"`
	Safe    config.SafeConfig    `json:"safe"`
	//plugin
	PluginPush        config.PluginPushConfig      `json:"plugin_push"`
	PluginSitemap     config.PluginSitemapConfig   `json:"plugin_sitemap"`
	PluginRewrite     config.PluginRewriteConfig   `json:"plugin_rewrite"`
	PluginAnchor      config.PluginAnchorConfig    `json:"plugin_anchor"`
	PluginGuestbook   config.PluginGuestbookConfig `json:"plugin_guestbook"`
	PluginUploadFiles []config.PluginUploadFile    `json:"plugin_upload_file"`
	PluginSendmail    config.PluginSendmail        `json:"plugin_sendmail"`
	PluginImportApi   config.PluginImportApiConfig `json:"plugin_import_api"`
	PluginStorage     config.PluginStorageConfig   `json:"plugin_storage"`
	PluginPay         config.PluginPayConfig       `json:"plugin_pay"`
	PluginWeapp       config.PluginWeappConfig     `json:"plugin_weapp"`
	PluginWechat      config.PluginWeappConfig     `json:"plugin_wechat"`
	PluginRetailer    config.PluginRetailerConfig  `json:"plugin_retailer"`
	PluginUser        config.PluginUserConfig      `json:"plugin_user"`
	PluginOrder       config.PluginOrderConfig     `json:"plugin_order"`
	PluginFulltext    config.PluginFulltextConfig  `json:"plugin_fulltext"`

	CollectorConfig config.CollectorJson
	KeywordConfig   config.KeywordJson

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
	initialed := false
	var db *gorm.DB
	if mw.Id == 1 {
		// 站点 1的数据库信息使用 defaultDB
		db = defaultDB
	} else {
		if mw.Mysql.UseDefault {
			mw.Mysql.User = config.Server.Mysql.User
			mw.Mysql.Password = config.Server.Mysql.Password
			mw.Mysql.Host = config.Server.Mysql.Host
			mw.Mysql.Port = config.Server.Mysql.Port
		}
		db, _ = InitDB(&mw.Mysql)
	}
	if db != nil && mw.Status == 1 {
		initialed = true
	}
	w := Website{
		Id:         mw.Id,
		Mysql:      &mw.Mysql,
		Initialed:  initialed,
		DB:         db,
		RootPath:   mw.RootPath,
		CachePath:  mw.RootPath + "cache/",
		DataPath:   mw.RootPath + "data/",
		PublicPath: mw.RootPath + "public/",
	}
	if mw.Id == 1 {
		w.Mysql = &config.Server.Mysql
	}
	websites[mw.Id] = &w
	if db != nil {
		_ = AutoMigrateDB(db)
		w.InitSetting()
		w.InitModelData()
		w.InitBucket()
		w.InitMemCache()
		// 初始化索引,异步处理
		go w.InitFulltext()
	}
}

func GetWebsites() map[uint]*Website {
	return websites
}

func CurrentSite(ctx iris.Context) *Website {
	if len(websites) == 0 {
		return nil
	}
	if ctx != nil {
		// 获取到当前website
		host := ctx.Host()
		for _, w := range websites {
			// 判断内容，base_url,mobile_url,admin_url
			parsed, err := url.Parse(w.System.BaseUrl)
			if err == nil && parsed.Host == host {
				return w
			}
			if w.System.MobileUrl != "" {
				parsed, err = url.Parse(w.System.MobileUrl)
				if err == nil && parsed.Host == host {
					return w
				}
			}
			if w.System.AdminUrl != "" {
				parsed, err = url.Parse(w.System.AdminUrl)
				if err == nil && parsed.Host == host {
					return w
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

func RemoveWebsite(siteId uint) {
	delete(websites, siteId)
}

func (w *Website) GetTemplateDir() string {
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

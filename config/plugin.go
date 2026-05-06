package config

import (
	"strings"
	"sync"
)

type CodeItem struct {
	Name  string `json:"name"`
	Value string `json:"value"`
}

type PluginPushConfig struct {
	BaiduApi   string     `json:"baidu_api"`
	BingApi    string     `json:"bing_api"`
	GoogleJson string     `json:"google_json"`
	JsCode     string     `json:"js_code"`
	JsCodes    []CodeItem `json:"js_codes"`
}

type PluginSitemapConfig struct {
	AutoBuild   int    `json:"auto_build"`
	Type        string `json:"type"`
	UpdatedTime int64  `json:"updated_time"`
	SitemapURL  string `json:"sitemap_url"`

	ExcludeTag         bool   `json:"exclude_tag"`
	ExcludeModuleIds   []uint `json:"exclude_module_ids"`
	ExcludeCategoryIds []uint `json:"exclude_category_ids"`
	ExcludePageIds     []uint `json:"exclude_page_ids"`
}

type PluginAnchorConfig struct {
	AnchorDensity int `json:"anchor_density"`
	ReplaceWay    int `json:"replace_way"` // 0 = 不替换 1 = 入库替换，2 = 渲染替换
	KeywordWay    int `json:"keyword_way"`
	NoStrongTag   int `json:"no_strong_tag"` // 0 = 加粗 1 = 不加粗
}

type PluginGuestbookConfig struct {
	ReturnMessage string         `json:"return_message"`
	Fields        []*CustomField `json:"fields"`
}

type CustomField struct {
	Name        string      `json:"name"`
	FieldName   string      `json:"field_name,omitempty"`
	Type        string      `json:"type,omitempty"`
	Value       interface{} `json:"value"`
	Default     interface{} `json:"default"`
	Remark      string      `json:"remark,omitempty"`
	Required    bool        `json:"required,omitempty"`
	IsSystem    bool        `json:"is_system,omitempty"`
	IsFilter    bool        `json:"is_filter,omitempty"`
	FollowLevel bool        `json:"follow_level"`
	Content     string      `json:"content,omitempty"`
	Items       []string    `json:"-"`
}

type CustomFieldTexts struct {
	Key    string   `json:"key"`
	Value  string   `json:"value"`
	Values []string `json:"values"` // 更多的字段
}

type TimelineField struct {
	Title   string            `json:"title"`
	Content string            `json:"content"`
	Status  string            `json:"status"`
	Images  []string          `json:"images"`
	Extra   map[string]string `json:"extra"`
	Items   []TimelineField   `json:"items,omitempty"`
}

type PluginUploadFile struct {
	Hash        string `json:"hash"`
	FileName    string `json:"file_name"`
	CreatedTime int64  `json:"created_time"`
	Link        string `json:"link"`
}

type PluginSendmail struct {
	Server    string `json:"server"`
	UseSSL    int    `json:"use_ssl"`
	Port      int    `json:"port"`
	Account   string `json:"account"`
	Password  string `json:"password"`
	Recipient string `json:"recipient"`

	AutoReply    bool   `json:"auto_reply"`
	ReplySubject string `json:"reply_subject"`
	ReplyMessage string `json:"reply_message"` // 自动回复内容
	SendType     []int  `json:"send_type"`

	SignupVerify  bool   `json:"signup_verify"` // 注册邮件验证
	VerifySubject string `json:"verify_subject"`
	VerifyMessage string `json:"verify_message"`
}

type PluginImportApiConfig struct {
	Token     string `json:"token"`      // 文档导入token
	LinkToken string `json:"link_token"` // 友情链接token
}

type PluginStorageConfig struct {
	StorageUrl  string `json:"storage_url"`
	StorageType string `json:"storage_type"`
	KeepLocal   bool   `json:"keep_local"`

	AliyunEndpoint        string `json:"aliyun_endpoint"`
	AliyunAccessKeyId     string `json:"aliyun_access_key_id"`
	AliyunAccessKeySecret string `json:"aliyun_access_key_secret"`
	AliyunBucketName      string `json:"aliyun_bucket_name"`

	TencentSecretId  string `json:"tencent_secret_id"`
	TencentSecretKey string `json:"tencent_secret_key"`
	TencentBucketUrl string `json:"tencent_bucket_url"`

	QiniuAccessKey string `json:"qiniu_access_key"`
	QiniuSecretKey string `json:"qiniu_secret_key"`
	QiniuBucket    string `json:"qiniu_bucket"`
	QiniuRegion    string `json:"qiniu_region"`

	UpyunBucket   string `json:"upyun_bucket"`
	UpyunOperator string `json:"upyun_operator"`
	UpyunPassword string `json:"upyun_password"`

	FTPHost     string `json:"ftp_host"`
	FTPPort     int    `json:"ftp_port"`
	FTPUsername string `json:"ftp_username"`
	FTPPassword string `json:"ftp_password"`
	FTPWebroot  string `json:"ftp_webroot"`

	SSHHost       string `json:"ssh_host"`
	SSHPort       int    `json:"ssh_port"`
	SSHUsername   string `json:"ssh_username"`
	SSHPassword   string `json:"ssh_password"`
	SSHPrivateKey string `json:"ssh_private_key"` // 私钥文件名
	SSHWebroot    string `json:"ssh_webroot"`

	GoogleProjectId       string `json:"google_project_id"`
	GoogleCredentialsJson string `json:"google_credentials_json"`
	GoogleBucketName      string `json:"google_bucket_name"`

	S3Region    string `json:"s3_region"`
	S3Bucket    string `json:"s3_bucket"`
	S3AccessKey string `json:"s3_access_key"`
	S3SecretKey string `json:"s3_secret_key"`
	S3Endpoint  string `json:"s3_endpoint"`
}

type PluginFulltextConfig struct {
	Open        bool   `json:"open"`
	UseContent  bool   `json:"use_content"`  // 是否索引内容
	UseCategory bool   `json:"use_category"` // 是否索引分类
	UseTag      bool   `json:"use_tag"`      // 是否索引标签
	Modules     []uint `json:"modules"`
	Initialed   bool   `json:"initialed"` //是否已经生成过索引

	Engine        string `json:"engine"` // 支持的搜索引擎：default(wukong)|Elasticsearch|ZincSearch|Meilisearch
	EngineUrl     string `json:"engine_url"`
	EngineUser    string `json:"engine_user"`
	EnginePass    string `json:"engine_pass"`
	RankingScore  int    `json:"ranking_score"`  // 可设置评分 0-100分，默认 0分 高于这个评分的结果才显示
	ContainLength int    `json:"contain_length"` // 可设置搜索词包含长度，默认 0，低于x个需要全包含，高于x个则至少包含x个字符
}

type PluginTitleImageConfig struct {
	Open        bool     `json:"open"`
	DrawSub     bool     `json:"draw_sub"`
	BgImages    []string `json:"bg_images"`
	FontPath    string   `json:"font_path"`
	FontSize    int      `json:"font_size"`
	FontColor   string   `json:"font_color"`
	FontBgColor string   `json:"font_bg_color"` // 文字背景色
	Width       int      `json:"width"`
	Height      int      `json:"height"`
	Noise       bool     `json:"noise"`
}

type PluginHtmlCache struct {
	Open          bool   `json:"open"`
	IndexCache    int64  `json:"index_cache"`     // 首页缓存时间
	ListCache     int64  `json:"category_cache"`  // 列表页缓存时间
	DetailCache   int64  `json:"detail_cache"`    // 详情页缓存时间
	LastBuildTime int64  `json:"last_build_time"` // 上一次手动生成时间
	LastPushTime  int64  `json:"last_push_time"`  // 上一次手动推送时间
	ErrorMsg      string `json:"error_msg"`
	PluginStorageConfig
}

type PluginTimeFactor struct {
	Open        bool     `json:"open"`
	ModuleIds   []int64  `json:"module_ids"`
	Types       []string `json:"types"`
	StartDay    int      `json:"start_day"`
	EndDay      int      `json:"end_day"`
	CategoryIds []int64  `json:"category_ids"`
	DoPublish   bool     `json:"do_publish"`
	ReleaseOpen bool     `json:"release_open"`
	DailyLimit  int      `json:"daily_limit"` // 自动发布用
	StartTime   int      `json:"start_time"`
	EndTime     int      `json:"end_time"`
	TodayCount  int      `json:"today_count"` // 当天发布了多少
	LastSent    int64    `json:"last_sent"`
	DailyUpdate int      `json:"daily_update"` // 自动更新用
	TodayUpdate int      `json:"today_update"` // 当天更新了多少
	LastUpdate  int64    `json:"last_update"`  // 最后更新时间
	Random      bool     `json:"random"`

	UpdateRunning bool `json:"-"`
}

type PluginInterference struct {
	Open              bool `json:"open"`
	Mode              int  `json:"mode"`
	DisableSelection  bool `json:"disable_selection"`
	DisableCopy       bool `json:"disable_copy"`
	DisableRightClick bool `json:"disable_right_click"`
}

type PluginWatermark struct {
	Open      bool   `json:"open"`
	Type      int    `json:"type"` // 0 image, 1 text
	ImagePath string `json:"image_path"`
	Text      string `json:"text,omitempty"`
	FontPath  string `json:"font_path"`
	Size      int    `json:"size"`
	Color     string `json:"color"`
	Position  int    `json:"position"` // 5 居中，1 左上角，3 右上角 7 左下角 9 右下角
	Opacity   int    `json:"opacity"`
	MinSize   int    `json:"min_size"`
}

type PluginLimiter struct {
	Open          bool     `json:"open"`
	WhiteIPs      []string `json:"white_ips"`
	BlackIPs      []string `json:"black_ips"`
	MaxRequests   int      `json:"max_requests"`
	BlockHours    int      `json:"block_hours"`
	BlockAgents   []string `json:"block_agents"`
	AllowPrefixes []string `json:"allow_prefixes"`
	IsAllowSpider bool     `json:"is_allow_spider"`
	BanEmptyRefer bool     `json:"ban_empty_refer"` // 只限制图片，js之类
	BanEmptyAgent bool     `json:"ban_empty_agent"` // 限制 curl 等
	MemLimit      bool     `json:"mem_limit"`
	MemPercent    int      `json:"mem_percent"`
}

type MultiLangSite struct {
	Id            uint   `json:"id"`
	RootPath      string `json:"root_path,omitempty"`
	Name          string `json:"name,omitempty"`
	Status        bool   `json:"status,omitempty"`
	ParentId      uint   `json:"parent_id,omitempty"`
	SyncTime      int64  `json:"sync_time,omitempty"`
	LanguageIcon  string `json:"language_icon"` // 图标
	LanguageEmoji string `json:"language_emoji,omitempty"`
	LanguageName  string `json:"language_name,omitempty"`
	Language      string `json:"language"`
	IsMain        bool   `json:"is_main"`
	IsCurrent     bool   `json:"is_current,omitempty"`
	Link          string `json:"link,omitempty"`
	BaseUrl       string `json:"base_url,omitempty"`
	ErrorMsg      string `json:"error_msg,omitempty"`
}

type PluginMultiLangConfig struct {
	mu              *sync.Mutex     `json:"-"`
	Open            bool            `json:"open"`
	Type            string          `json:"type"`
	DefaultLanguage string          `json:"default_language"` // 该语言只是调用系统的设置
	AutoTranslate   bool            `json:"auto_translate"`
	SiteType        string          `json:"site_type"`     // multi|single
	ShowMainDir     bool            `json:"show_main_dir"` // 显示主站目录
	SubSites        []MultiLangSite `json:"sub_sites"`
}

type PluginAkismetConfig struct {
	Open      bool   `json:"open"`
	ApiKey    string `json:"api_key"`
	CheckType []int  `json:"check_type"`
}

func (pm *PluginMultiLangConfig) GetUrl(oriUrl string, baseUrl string, langSite *MultiLangSite) string {
	if pm.SiteType == MultiLangSiteTypeSingle {
		if pm.Type == MultiLangTypeDomain {
			oriUrl = strings.Replace(oriUrl, baseUrl, langSite.BaseUrl, 1)
		} else if pm.Type == MultiLangTypeDirectory {
			// 替换目录
			if strings.HasPrefix(oriUrl, baseUrl+"/"+pm.DefaultLanguage) {
				oriUrl = strings.Replace(oriUrl, baseUrl+"/"+pm.DefaultLanguage, baseUrl, 1)
			}
			if langSite.IsMain && pm.ShowMainDir == false {
				// 无需处理
			} else {
				oriUrl = strings.Replace(oriUrl, baseUrl, baseUrl+"/"+langSite.Language, 1)
			}
		} else if pm.Type == MultiLangTypeSame {
			// 相同
			if strings.Contains(oriUrl, "?") {
				oriUrl = oriUrl + "&lang=" + langSite.Language
			} else {
				oriUrl += "?lang=" + langSite.Language
			}
		}
	}

	// 返回默认值
	return oriUrl
}

func (pm *PluginMultiLangConfig) GetSite(lang string) *MultiLangSite {
	if lang == "" {
		return nil
	}
	for i := range pm.SubSites {
		if pm.SubSites[i].Language == lang {
			return &pm.SubSites[i]
		}
	}
	// 如果没匹配的话，则尝试匹配前缀
	if strings.Contains(lang, "-") {
		lang = strings.Split(lang, "-")[0]
		for i := range pm.SubSites {
			if pm.SubSites[i].Language == lang {
				return &pm.SubSites[i]
			}
		}
	}
	return nil
}

func (pm *PluginMultiLangConfig) GetSiteByBaseUrl(baseUrl string) *MultiLangSite {
	for i := range pm.SubSites {
		if strings.Contains(pm.SubSites[i].BaseUrl, baseUrl) {
			return &pm.SubSites[i]
		}
	}
	return nil
}

func (pm *PluginMultiLangConfig) RemoveSite(id uint, lang string) {
	for i := range pm.SubSites {
		if id > 0 && pm.SubSites[i].Id == id {
			pm.SubSites = append(pm.SubSites[:i], pm.SubSites[i+1:]...)
			break
		} else if lang != "" && pm.SubSites[i].Language == lang {
			pm.SubSites = append(pm.SubSites[:i], pm.SubSites[i+1:]...)
			break
		}
	}
}

func (pm *PluginMultiLangConfig) SaveSite(site MultiLangSite) {
	var exist = false
	for i := range pm.SubSites {
		if site.Id > 0 && pm.SubSites[i].Id == site.Id {
			exist = true
			// 已存在，更新
			pm.SubSites[i] = site
			break
		} else if pm.SubSites[i].Language == site.Language {
			exist = true
			// 已存在，更新
			pm.SubSites[i] = site
			break
		}
	}
	if !exist {
		pm.SubSites = append(pm.SubSites, site)
	}
}

type PluginTranslateConfig struct {
	Engine          string `json:"engine"`            // 使用的翻译引擎，默认为官方接口，可选有：baidu,youdao,ai
	BaiduAppId      string `json:"baidu_app_id"`      // 百度翻译
	BaiduAppSecret  string `json:"baidu_app_secret"`  // 百度翻译
	YoudaoAppKey    string `json:"youdao_app_key"`    // 有道翻译
	YoudaoAppSecret string `json:"youdao_app_secret"` // 有道翻译
	DeeplAuthKey    string `json:"deepl_auth_key"`    // Deepl
}

type DataSchemaType struct {
	Id         uint   `json:"id"`          // 模型/分类ID
	ListType   string `json:"list_type"`   // 列表页类型： CollectionPage, DetailedItemList, ItemList
	SchemaType string `json:"schema_type"` // 结构化数据类型： Article, Product, ScholarlyArticle, BlogPosting, NewsArticle, AnalysisNewsArticle, AskPublicNewsArticle, BackgroundNewsArticle, OpinionNewsArticle, ReportageNewsArticle, ReviewNewsArticle, WebPage, ItemPage, Recipe, Course, FAQPage, HowTo, Event, Person, Place
}

type PluginJsonLdConfig struct {
	Open                  bool             `json:"open"`                    // 是否开启数据结构化输出
	AboutPageId           uint             `json:"about_page_id"`           // 关于页id，用于生成合适的结构化数据
	ContactPageId         uint             `json:"contact_page_id"`         // 联系页id，用于生成合适的结构化数据
	IncludeHomepage       bool             `json:"include_homepage"`        // 是否生成首页的结构化数据
	IncludeSearch         bool             `json:"include_search"`          // 是否包含搜索链接
	IncludeAuthor         bool             `json:"include_author"`          // 是否包含作者
	Author                string           `json:"author"`                  // 作者
	AuthorUrl             string           `json:"author_url"`              // 作者url
	IncludeBreadcrumb     bool             `json:"include_breadcrumb"`      // 是否包含面包屑导航
	IncludeComments       bool             `json:"include_comments"`        // 是否包含评论, 商品页面包含review
	Module                []DataSchemaType `json:"module"`                  // 各个模型定义的结构化数据类型
	Category              []DataSchemaType `json:"category"`                // 自定义分类的结构化数据类型，没定义的继承模型的结构化数据类型
	DefaultBrand          string           `json:"default_brand"`           // 默认品牌
	DefaultImage          string           `json:"default_image"`           // 默认图片
	DataType              int              `json:"data_type"`               // 1 = Organization, 2 = Person
	OrganizationType      string           `json:"organization_type"`       // LocalBusiness, Airline, Consortium, Corporation, EducationalOrganization, School, GovernmentOrganization, LibrarySystem, MedicalOrganization, NewsMediaOrganization, NGO, PerformingGroup, SportsOrganization, WorkersUnion
	OrganizationName      string           `json:"organization_name"`       // 组织名称
	OrganizationLegalName string           `json:"organization_legal_name"` // 组织法律名称
	OrganizationUrl       string           `json:"organization_url"`        // 组织url, 默认为网站url
	PersonName            string           `json:"person_name"`             // 个人名称
	PersonJobTitle        string           `json:"person_job_title"`        // 个人职务
	PersonImage           string           `json:"person_image"`            // 个人图片
	ContactType           string           `json:"contact_type"`            // 联系类型: general, customer support, technical support, billing support, bill payment, sales, reservations, credit card support, emergency, baggage tracking, roadside assistance, package tracking
	ContactNumber         string           `json:"contact_number"`          // 联系电话
	ContactUrl            string           `json:"contact_url"`             // 联系url
	LogoImage             string           `json:"logo_image"`              // logo图片
	SocialProfiles        []string         `json:"social_profiles"`         // 社交账号列表
	OpeningDayOfWeek      []string         `json:"opening_day_of_week"`     // 开业时间列表
	OpeningStartTime      string           `json:"opening_start_time"`      // 开业时间
	OpeningEndTime        string           `json:"opening_end_time"`        // 关闭时间
	PriceRange            string           `json:"price_range"`             // 价格范围
	GeoLatitude           string           `json:"geo_latitude"`            // 纬度
	GeoLongitude          string           `json:"geo_longitude"`           // 经度
	StreetAddress         string           `json:"street_address"`          // 街道地址
	AddressLocality       string           `json:"address_locality"`        // 城市
	AddressRegion         string           `json:"address_region"`          // 州/省
	PostalCode            string           `json:"postal_code"`             // 邮政编码
	AddressCountry        string           `json:"address_country"`         // 国家/地区
}

type PluginLLMsConfig struct {
	Open                 bool   `json:"open"`
	UpdateFrequency      int    `json:"update_frequency"`       // Update Frequency 0 = 立即更新，1 = 每天更新一次，2 = 每周更新一次，3 = 不自动更新
	LastUpdate           int64  `json:"last_update"`            // 最后生成时间
	FileUrl              string `json:"file_url"`               // 文件URL
	FileStatus           bool   `json:"file_status"`            // true = 已生成， false = 未生成
	MaxPostPerType       int    `json:"max_post_per_type"`      // 每个类型的最大文章数
	MaxWords             int    `json:"max_words"`              // 每篇文章的最大字数
	IncludeMetadata      bool   `json:"include_metadata"`       // 是否包含元数据
	IncludeDescription   bool   `json:"include_description"`    // 是否包含描述
	IncludeCategory      bool   `json:"include_category"`       // 是否包含分类
	IncludeTag           bool   `json:"include_tag"`            // 是否包含标签
	IncludeExtra         bool   `json:"include_extra"`          // 是否包含额外字段
	ExcludeModuleIds     []uint `json:"exclude_module_ids"`     // 排除的模块id
	ExcludeCategoryIds   []uint `json:"exclude_category_ids"`   // 排除的分类id
	ExcludePageIds       []uint `json:"exclude_page_ids"`       // 排除的页面id
	LLMSTitle            string `json:"llms_title"`             // LLMS.txt 标题, 为你的LLMs.txt文件设置一个自定义标题。该标题将出现在生成的文件顶部，位于所有列出的网址之前。
	LLMSDescrption       string `json:"llms_description"`       // LLMs.txt 描述, 在URL列表之前添加了可选的介绍文本。使用此文本解释LLMs.txt文件的用途或结构。
	LLMSAfterDescription string `json:"llms_after_description"` // 在链接或内容条目列表之前插入的可选文本。您可以在网址开始之前使用它来添加额外的注释、上下文或数据使用信息。
	LLMSEndDescription   string `json:"llms_end_description"`   // 附加在LLMs.txt文件底部的结尾文本（例如页脚、联系方式或免责声明信息）。
}

func (g *CustomField) SplitContent() []string {
	var items []string
	contents := strings.Split(g.Content, "\n")
	for _, v := range contents {
		v = strings.TrimSpace(v)
		if v != "" {
			items = append(items, v)
		}
	}

	g.Items = items

	return items
}

// CheckSetFilter 支付允许筛选
func (g *CustomField) CheckSetFilter() bool {
	if g.Type != CustomFieldTypeRadio && g.Type != CustomFieldTypeCheckbox && g.Type != CustomFieldTypeSelect {
		g.IsFilter = false
		return false
	}
	if g.FollowLevel {
		g.IsFilter = false
		return false
	}

	return true
}

func (g *CustomField) GetFieldColumn() string {
	column := "`" + g.FieldName + "`"

	switch g.Type {
	case CustomFieldTypeNumber, CustomFieldTypeCategory:
		column += " int(10)"
	case CustomFieldTypeTextarea, CustomFieldTypeEditor, CustomFieldTypeImages, CustomFieldTypeTexts, CustomFieldTypeArchive:
		column += " text"
	case CustomFieldTypeTimeline:
		column += " longtext"
	case CustomFieldTypeCheckbox:
		column += " varchar(500)"
	default:
		// mysql 5.6 下，utf8mb4 索引只能用190
		column += " varchar(190)"
	}

	//if g.Required {
	//	column += " NOT NULL"
	//} else {
	//	column += " DEFAULT NULL"
	//}
	// 因为是后插值，因此这里默认都是null
	column += " DEFAULT NULL"

	return column
}

package config

import (
	"net/url"
	"strings"
	"sync"
)

type CodeItem struct {
	Name  string `json:"name"`  // 配置名称
	Value string `json:"value"` // JS
}

type PluginPushConfig struct {
	BaiduApi   string     `json:"baidu_api"`   // 百度推送地址
	BingApi    string     `json:"bing_api"`    // Bing 推送地址
	GoogleJson string     `json:"google_json"` // Google 推送所需JSON配置
	JsCode     string     `json:"js_code" ast:"-"`
	JsCodes    []CodeItem `json:"js_codes"` // 其它搜索引擎没提供APi，需要通过JS来推送的JS配置
}

type PluginSitemapConfig struct {
	AutoBuild   int    `json:"auto_build"` // 是否在文档发布时字段更新 Sitemap
	Type        string `json:"type"`       // Sitemap 格式：txt|xml
	UpdatedTime int64  `json:"updated_time" ast:"-"`
	SitemapURL  string `json:"sitemap_url" ast:"-"`
	PageSize    int    `json:"page_size"` // 每个 Sitemap 包含的链接数量，默认 20000

	ExcludeTag         bool   `json:"exclude_tag"`          // 是否排除 文档标签
	ExcludeModuleIds   []uint `json:"exclude_module_ids"`   // 需要排除的模型 ID 列表
	ExcludeCategoryIds []uint `json:"exclude_category_ids"` // 需要排除的分类 ID 列表
	ExcludePageIds     []uint `json:"exclude_page_ids"`     // 需要排除的单页面 ID 列表
}

type PluginAnchorConfig struct {
	AnchorDensity int `json:"anchor_density"` // 锚文本密度，小于10时默认为100。
	ReplaceWay    int `json:"replace_way"`    // 文档保存时是否自动替换锚文本：0 = 不替换 1 = 替换
	KeywordWay    int `json:"keyword_way"`    // 文档保存时是否自动提取锚文本：0 = 不提取 1 = 从关键词中提取
	NoStrongTag   int `json:"no_strong_tag"`  // 重复关键词是否不加strong标签：0 = 添加 1 = 不添加
}

type PluginGuestbookConfig struct {
	ReturnMessage string         `json:"return_message"` // 留言提交成功后返回的提示信息
	PushWay       int            `json:"push_way"`       // 0=email|1=站点|2=API接口
	SiteId        uint           `json:"site_id"`        // 站点ID，push_way=1时必填
	ApiMethod     string         `json:"api_method"`     // API推送数据格式：json|formdata|query
	ApiURL        string         `json:"api_url"`        // 推送接口地址
	HeaderKey     string         `json:"header_key"`     // 推送接口需要的请求头名称
	HeaderValue   string         `json:"header_value"`   // 推送接口需要的请求头值
	Fields        []*CustomField `json:"fields"`         // 留言字段列表
}

type CustomField struct {
	Name        string      `json:"name"`                 // 字段名称
	FieldName   string      `json:"field_name,omitempty"` // 字段标识
	Group       string      `json:"group"`                // 模块名称
	Type        string      `json:"type,omitempty"`       // 字段类型：text|number|textarea|editor|radio|checkbox|select|image|file|images|texts|archive|category|date|time|datetime|color|timeline
	Value       interface{} `json:"value"`                // 字段值
	Default     interface{} `json:"default"`              // 字段默认值
	Remark      string      `json:"remark,omitempty"`     // 字段描述
	Required    bool        `json:"required,omitempty"`   // 是否必填
	IsSystem    bool        `json:"is_system,omitempty"`  // 是否系统字段
	IsFilter    bool        `json:"is_filter,omitempty"`  // 是否支持筛选
	FollowLevel bool        `json:"follow_level"`         // 是否跟随文档的阅读 level（文档扩展字段使用）
	Content     string      `json:"content,omitempty"`    // 自动默认内容/radio|select|checkbox 的选项配置
	Items       []string    `json:"-"`                    // radio|select|checkbox 的选项配置解析值
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
	Server    string `json:"server"`    // SMTP服务器，如：smtp.qq.com
	UseSSL    int    `json:"use_ssl"`   // 使用SSL/TLS：0=不使用，1=SSL，2=TLS
	Port      int    `json:"port"`      // 端口， 默认 25，SSL 默认 465, TLS 默认 587
	Account   string `json:"account"`   // SMTP帐户
	Password  string `json:"password"`  // SMTP密码
	Recipient string `json:"recipient"` // 默认为SMTP账户，多个收件人请使用英文逗号`,`分开

	Templates []EmailTemplate `json:"templates"` // 启用的模板列表
}

type PluginImportApiConfig struct {
	Token     string `json:"token"`      // 文档导入token
	LinkToken string `json:"link_token"` // 友情链接token
}

type PluginStorageConfig struct {
	StorageUrl  string `json:"storage_url"`  // 静态资源地址
	StorageType string `json:"storage_type"` // 存储类型，默认本地：local=本地，aliyun=阿里云存储桶，tencent=腾讯云对象存储，qiniu=七牛云，upyun=又拍云，google=Google存储，awss3=Amazon S3，r2=Cloudflare R2，ftp=FTP，ssh=SFTP(SSH)通道
	KeepLocal   bool   `json:"keep_local"`   // 非本地存储时，是否在本地保存副本

	AliyunEndpoint        string `json:"aliyun_endpoint"`          // 阿里云节点
	AliyunAccessKeyId     string `json:"aliyun_access_key_id"`     // 阿里云 AccessKeyId
	AliyunAccessKeySecret string `json:"aliyun_access_key_secret"` // 阿里云 AccessKeySecret
	AliyunBucketName      string `json:"aliyun_bucket_name"`       // 阿里云存储桶名称

	TencentSecretId  string `json:"tencent_secret_id"`  // 腾讯云 SecretId
	TencentSecretKey string `json:"tencent_secret_key"` // 腾讯云 SecretKey
	TencentBucketUrl string `json:"tencent_bucket_url"` // 腾讯云存储桶地址

	QiniuAccessKey string `json:"qiniu_access_key"` // 七牛云 AccessKey
	QiniuSecretKey string `json:"qiniu_secret_key"` // 七牛云 SecretKey
	QiniuBucket    string `json:"qiniu_bucket"`     // 七牛云存储桶名称
	QiniuRegion    string `json:"qiniu_region"`     // 七牛云存储区域：z0=华东，z1=华北，z2=华南，na0=北美，as0=东南亚，cn-east-2=华东-浙江2，fog-cn-east-1=雾存储华东区

	UpyunBucket   string `json:"upyun_bucket"`   // 又拍云存储服务名称
	UpyunOperator string `json:"upyun_operator"` // 又拍云操作员
	UpyunPassword string `json:"upyun_password"` // 又拍云操作员密码

	FTPHost     string `json:"ftp_host"`     // FTP IP地址
	FTPPort     int    `json:"ftp_port"`     // FTP 端口
	FTPUsername string `json:"ftp_username"` // FTP 用户名
	FTPPassword string `json:"ftp_password"` // FTP 密码
	FTPWebroot  string `json:"ftp_webroot"`  // FTP 上传根目录

	SSHHost       string `json:"ssh_host"`        // SSH IP地址
	SSHPort       int    `json:"ssh_port"`        // SSH 端口
	SSHUsername   string `json:"ssh_username"`    // SSH 用户名
	SSHPassword   string `json:"ssh_password"`    // SSH 密码
	SSHPrivateKey string `json:"ssh_private_key"` // SSH 私钥文件名（通过密钥上传接口上传）
	SSHWebroot    string `json:"ssh_webroot"`     // SSH 上传根目录

	GoogleProjectId       string `json:"google_project_id"`       // 谷歌云项目ID
	GoogleCredentialsJson string `json:"google_credentials_json"` // 谷歌云密钥文件Json
	GoogleBucketName      string `json:"google_bucket_name"`      // 谷歌云存储桶名称

	S3Region    string `json:"s3_region"`     // S3区域（亚马逊/Cloudflare R2共用配置）
	S3Bucket    string `json:"s3_bucket"`     // S3存储桶名称
	S3AccessKey string `json:"s3_access_key"` // S3 SecretKey
	S3SecretKey string `json:"s3_secret_key"` // S3 SecretKey
	S3Endpoint  string `json:"s3_endpoint"`   // Cloudflare R2 Endpoint
}

type PluginFulltextConfig struct {
	Open        bool   `json:"open"`              // 是否开启全文索引
	UseContent  bool   `json:"use_content"`       // 是否索引内容
	UseCategory bool   `json:"use_category"`      // 是否索引分类
	UseTag      bool   `json:"use_tag"`           // 是否索引标签
	Modules     []uint `json:"modules"`           // 索引的模型
	Initialed   bool   `json:"initialed" ast:"-"` //是否已经生成过索引

	Engine        string `json:"engine"`         // 支持的索引引擎：default(wukong)，elasticsearch=Elasticsearch，zincsearch=ZincSearch，meilisearch=Meilisearch
	EngineUrl     string `json:"engine_url"`     // 引擎地址
	EngineUser    string `json:"engine_user"`    // 引擎用户名
	EnginePass    string `json:"engine_pass"`    // 引擎密码
	RankingScore  int    `json:"ranking_score"`  // 可设置评分 0-100分，默认 0分 高于这个评分的结果才显示
	ContainLength int    `json:"contain_length"` // 可设置搜索词包含长度，默认 0，低于x个需要全包含，高于x个则至少包含x个字符
}

type PluginTitleImageConfig struct {
	Open        bool     `json:"open"`          // 是否启用标题生成图片，启用后，如果文档没有图片，则根据标题自动生成一张图片
	DrawSub     bool     `json:"draw_sub"`      // 是否为二级标题生成图片
	BgImages    []string `json:"bg_images"`     // 背景图片地址列表
	FontPath    string   `json:"font_path"`     // 字体路径（通过字体上传接口上传）
	FontSize    int      `json:"font_size"`     // 文字大小，默认 32
	FontColor   string   `json:"font_color"`    // 文字颜色
	FontBgColor string   `json:"font_bg_color"` // 文字背景色
	Width       int      `json:"width"`         // 图片宽度，默认 800
	Height      int      `json:"height"`        // 图片高度，默认 600
	Noise       bool     `json:"noise"`         // 是否生成干扰斑点
}

type PluginHtmlCache struct {
	Open                bool   `json:"open"`                    // 是否开启静态缓存。
	IndexCache          int64  `json:"index_cache"`             // 首页缓存时间
	ListCache           int64  `json:"category_cache"`          // 列表页缓存时间
	DetailCache         int64  `json:"detail_cache"`            // 详情页缓存时间
	LastBuildTime       int64  `json:"last_build_time" ast:"-"` // 上一次手动生成时间
	LastPushTime        int64  `json:"last_push_time" ast:"-"`  // 上一次手动推送时间
	ErrorMsg            string `json:"error_msg" ast:"-"`
	PluginStorageConfig        // 静态缓存的存储配置
}

type PluginTimeFactor struct {
	Open        bool     `json:"open"`                 // 是否启用旧文档时间更新
	ModuleIds   []int64  `json:"module_ids"`           // 启用的模型 ID 列表
	Types       []string `json:"types"`                // 更新的字段，可多选，至少选1个：created_time|updated_time
	StartDay    int      `json:"start_day"`            // 更新x天前的文档
	EndDay      int      `json:"end_day"`              // 更新到x天内的时间
	DailyUpdate int      `json:"daily_update"`         // 每天最多更新x篇
	CategoryIds []int64  `json:"category_ids"`         // 不参与更新的分类 ID 列表
	DoPublish   bool     `json:"do_publish"`           // 是否将更新的文档重新推送给搜索引擎
	ReleaseOpen bool     `json:"release_open"`         // 是否启用草稿箱文档自动发布
	DailyLimit  int      `json:"daily_limit"`          // 自动发布用，每天发布数量
	StartTime   int      `json:"start_time"`           // 自动发布用，每天发布开始时间：0-23
	EndTime     int      `json:"end_time"`             // 自动发布用，每天结束时间：0-23
	Random      bool     `json:"random"`               // 是发ID随机发布
	TodayCount  int      `json:"today_count" ast:"-"`  // 当天发布了多少
	LastSent    int64    `json:"last_sent" ast:"-"`    // 最好推送时间
	TodayUpdate int      `json:"today_update" ast:"-"` // 当天更新了多少
	LastUpdate  int64    `json:"last_update" ast:"-"`  // 最后更新时间

	UpdateRunning bool `json:"-"`
}

type PluginInterference struct {
	Open              bool `json:"open"`                // 是否开启
	Mode              int  `json:"mode"`                // 干扰模式：0=添加随机class，1=添加随机隐藏文字
	DisableSelection  bool `json:"disable_selection"`   // 是否禁止选择
	DisableCopy       bool `json:"disable_copy"`        // 是否禁止复制
	DisableRightClick bool `json:"disable_right_click"` // 是否禁止右键
}

type PluginWatermark struct {
	Open      bool   `json:"open"`           // 是否开启图片水印
	Type      int    `json:"type"`           // 水印类型：0=image, 1=text
	ImagePath string `json:"image_path"`     // 水印图片地址,type=image 时需要
	Text      string `json:"text,omitempty"` // 水印文字，type=text 时需要
	FontPath  string `json:"font_path"`      // 字体路径，type=text 时需要，通过字体上传接口上传
	Size      int    `json:"size"`           // 文字大小，默认：20
	Color     string `json:"color"`          // 文字颜色，默认：#ffffff
	Position  int    `json:"position"`       // 水印位置：5 居中，1 左上角，3 右上角 7 左下角 9 右下角
	Opacity   int    `json:"opacity"`        // 水印透明度：1-100
	MinSize   int    `json:"min_size"`       // 图片宽度最小x像素才加水印
}

type PluginLimiter struct {
	Open          bool     `json:"open"`            // 是否开启
	WhiteIPs      []string `json:"white_ips"`       // 白名单IP列表，支持IP段
	BlackIPs      []string `json:"black_ips"`       // 黑名单IP列表，支持IP段
	MaxRequests   int      `json:"max_requests"`    // 最大请求数
	BlockHours    int      `json:"block_hours"`     // 封禁时间（小时）
	BlockAgents   []string `json:"block_agents"`    // 封禁的UserAgent关键词列表
	AllowPrefixes []string `json:"allow_prefixes"`  // 允许的IP前缀
	IsAllowSpider bool     `json:"is_allow_spider"` // 是否允许爬虫
	BanEmptyRefer bool     `json:"ban_empty_refer"` // 只限制图片，js之类
	BanEmptyAgent bool     `json:"ban_empty_agent"` // 限制 curl 等
	MemLimit      bool     `json:"mem_limit"`       // 是否限制内存使用
	MemPercent    int      `json:"mem_percent"`     // 内存使用限制百分比
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
	Open            bool            `json:"open"`                     // 是否启用
	Type            string          `json:"type"`                     // 多语言路由类型：domain=子域名，directory=子目录，same=相同链接，按浏览器Cookie/Session识别语言
	DefaultLanguage string          `json:"default_language" ast:"-"` // 该语言只是调用系统的设置
	AutoTranslate   bool            `json:"auto_translate"`           // 是否自动翻译
	SiteType        string          `json:"site_type"`                // 站点数据类型：multi=每个语言独立站点数据，single=共用主站数据，页面直接按语言翻译展示
	ShowMainDir     bool            `json:"show_main_dir"`            // 显示主站语言目录，type=directory 时有效
	SubSites        []MultiLangSite `json:"sub_sites"`                // 多语言站点列表
}

type PluginAkismetConfig struct {
	Open      bool   `json:"open"`       // 是否启用 Akismet
	ApiKey    string `json:"api_key"`    // akismet api key
	CheckType []int  `json:"check_type"` // 检测类型：1=留言，2=评论
	// reCAPTCHA
	RecaptchaOpen       bool   `json:"recaptcha_open"`        // 是否启用 reCAPTCHA
	RecaptchaSiteKey    string `json:"recaptcha_site_key"`    // 网站密钥
	RecaptchaPrivateKey string `json:"recaptcha_private_key"` // 通信密钥
}

const EmailTypeSystem = "system"
const EmailTypeUser = "user"

type EmailTemplate struct {
	Open        bool   `json:"open"`        // 是否启用
	Type        string `json:"type"`        // 模板类型：system=系统模板，user=用户模板
	Key         string `json:"key"`         // 模板标识
	Name        string `json:"name"`        // 模板名称
	Description string `json:"description"` // 模板说明
	Readonly    bool   `json:"readonly"`    // 邮件内容是否只读，只读的话，邮件内容不可编辑
	Delay       int64  `json:"delay"`       // 延迟多少秒发送
	Subject     string `json:"subject"`     // 邮件标题
	Content     string `json:"content"`     // 邮件内容
}

type CommunicationConfig struct {
	SummitedTemplate EmailTemplate  `json:"summited_template"` // 提交成功模板
	QuoteTemplate    EmailTemplate  `json:"quote_template"`    // 报价模板
	ReplyTemplate    EmailTemplate  `json:"reply_template"`    // 消息回复模板
	OrderFields      []*CustomField `json:"order_fields"`      // 需要哪些自定义字段
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
		subUrl, err := url.Parse(pm.SubSites[i].BaseUrl)
		if err != nil {
			continue
		}
		if subUrl.Host == baseUrl {
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
		}
		if lang != "" && pm.SubSites[i].Language == lang {
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
	Engine          string `json:"engine"`            // 使用的翻译引擎，默认为官方接口，可选有：baidu,youdao,ai,deepl
	BaiduAppId      string `json:"baidu_app_id"`      // 百度翻译 APPID
	BaiduAppSecret  string `json:"baidu_app_secret"`  // 百度翻译 AppSecret
	YoudaoAppKey    string `json:"youdao_app_key"`    // 有道翻译 AppKey
	YoudaoAppSecret string `json:"youdao_app_secret"` // 有道翻译 AppSecret
	DeeplAuthKey    string `json:"deepl_auth_key"`    // Deepl Auth Key
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
	Open                 bool   `json:"open"`                   // 是否开启
	UpdateFrequency      int    `json:"update_frequency"`       // Update Frequency: 0 = 立即更新，1 = 每天更新一次，2 = 每周更新一次，3 = 不自动更新
	LastUpdate           int64  `json:"last_update"`            // 最后生成时间
	FileUrl              string `json:"file_url"`               // 文件URL
	FileStatus           bool   `json:"file_status"`            // 文件状态：true = 已生成， false = 未生成
	MaxPostPerType       int    `json:"max_post_per_type"`      // 每个类型的最大文章数
	MaxWords             int    `json:"max_words"`              // 每篇文章的最大字数
	IncludeMetadata      bool   `json:"include_metadata"`       // 是否包含元数据
	IncludeDescription   bool   `json:"include_description"`    // 是否包含描述
	IncludeCategory      bool   `json:"include_category"`       // 是否包含分类
	IncludeTag           bool   `json:"include_tag"`            // 是否包含标签
	IncludeExtra         bool   `json:"include_extra"`          // 是否包含额外字段
	ExcludeModuleIds     []uint `json:"exclude_module_ids"`     // 排除的模型id
	ExcludeCategoryIds   []uint `json:"exclude_category_ids"`   // 排除的分类id
	ExcludePageIds       []uint `json:"exclude_page_ids"`       // 排除的页面id
	LLMSTitle            string `json:"llms_title"`             // LLMs.txt 标题, 为你的LLMs.txt文件设置一个自定义标题。该标题将出现在生成的文件顶部，位于所有列出的网址之前。
	LLMSDescrption       string `json:"llms_description"`       // LLMs.txt 描述, 在URL列表之前添加了可选的介绍文本。使用此文本解释LLMs.txt文件的用途或结构。
	LLMSAfterDescription string `json:"llms_after_description"` // 在链接或内容条目列表之前插入的可选文本。您可以在网址开始之前使用它来添加额外的注释、上下文或数据使用信息。
	LLMSEndDescription   string `json:"llms_end_description"`   // 附加在LLMs.txt文件底部的结尾文本（例如页脚、联系方式或免责声明信息）。
}

type PluginPlaceConfig struct {
	Open        bool          `json:"open"`         // 是否启用城市分站
	UrlType     string        `json:"url_type"`     // URL形式，subdomain|directory,默认：directory
	ContentType string        `json:"content_type"` // 默认=内容需要绑定分站，full=全站复用
	Fields      []CustomField `json:"fields"`       // 城市站自定义字段
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
	case CustomFieldTypeTextarea, CustomFieldTypeImages, CustomFieldTypeTexts, CustomFieldTypeArchive:
		column += " text"
	case CustomFieldTypeEditor, CustomFieldTypeTimeline:
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

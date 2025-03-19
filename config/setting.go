package config

const (
	//自适应
	TemplateTypeAuto = 0
	//代码适配
	TemplateTypeAdapt = 1
	//电脑+手机
	TemplateTypeSeparate = 2
)

type SystemConfig struct {
	SiteName      string       `json:"site_name"`
	SiteLogo      string       `json:"site_logo"`
	SiteIcp       string       `json:"site_icp"`
	SiteCopyright string       `json:"site_copyright"`
	BaseUrl       string       `json:"base_url"`
	MobileUrl     string       `json:"mobile_url"`
	AdminUrl      string       `json:"admin_url"`
	SiteClose     int          `json:"site_close"`
	SiteCloseTips string       `json:"site_close_tips"`
	BanSpider     int          `json:"ban_spider"`
	TemplateName  string       `json:"template_name"`
	TemplateType  int          `json:"template_type"`
	TemplateUrl   string       `json:"template_url"` // template 的静态文件目录
	Language      string       `json:"language"`     // 语言包引用
	ExtraFields   []ExtraField `json:"extra_fields"` // 用户自定义字段
	Favicon       string       `json:"favicon"`
	DefaultSite   bool         `json:"default_site"` // 是否是默认站点，每次读取的时候会检查
}

type ContentConfig struct {
	RemoteDownload int    `json:"remote_download"`
	FilterOutlink  int    `json:"filter_outlink"`
	UrlTokenType   int    `json:"url_token_type"`
	UseWebp        int    `json:"use_webp"`
	ConvertGif     int    `json:"convert_gif"` // 在转换成webp的时候，是否转换gif，1=true
	Quality        int    `json:"quality"`
	ResizeImage    int    `json:"resize_image"`
	ResizeWidth    int    `json:"resize_width"`
	ThumbCrop      int    `json:"thumb_crop"`
	ThumbWidth     int    `json:"thumb_width"`
	ThumbHeight    int    `json:"thumb_height"`
	DefaultThumb   string `json:"default_thumb"`
	MultiCategory  int    `json:"multi_category"` // 是否启用多分类支持
	Editor         string `json:"editor"`         // 使用的editor，默认为空，支持 空值|default|markdown
	UseSort        int    `json:"use_sort"`       // 启用文档排序
	MaxPage        int    `json:"max_page"`       // 最大显示页码
	MaxLimit       int    `json:"max_limit"`      // 最大显示条数
}

type IndexConfig struct {
	SeoTitle       string `json:"seo_title"`
	SeoKeywords    string `json:"seo_keywords"`
	SeoDescription string `json:"seo_description"`
}

type ContactConfig struct {
	UserName    string       `json:"user_name"`
	Cellphone   string       `json:"cellphone"`
	Address     string       `json:"address"`
	Email       string       `json:"email"`
	Wechat      string       `json:"wechat"`
	QQ          string       `json:"qq"`
	WhatsApp    string       `json:"whats_app"`
	Facebook    string       `json:"facebook"`
	Twitter     string       `json:"twitter"`
	Tiktok      string       `json:"tiktok"`
	Pinterest   string       `json:"pinterest"`
	Linkedin    string       `json:"linkedin"`
	Instagram   string       `json:"instagram"`
	Youtube     string       `json:"youtube"`
	Qrcode      string       `json:"qrcode"`
	ExtraFields []ExtraField `json:"extra_fields"` // 用户自定义字段
}

type SafeConfig struct {
	Captcha          int    `json:"captcha"`
	DailyLimit       int    `json:"daily_limit"`
	ContentLimit     int    `json:"content_limit"`
	IntervalLimit    int    `json:"interval_limit"`
	ContentForbidden string `json:"content_forbidden"`
	IPForbidden      string `json:"ip_forbidden"`
	UAForbidden      string `json:"ua_forbidden"`
	APIOpen          int    `json:"api_open"`
	APIPublish       int    `json:"api_publish"`
	AdminCaptchaOff  int    `json:"admin_captcha_off"`
}

type ExtraField struct {
	Name    string `json:"name"`
	Type    string `json:"type"`
	Value   string `json:"value"`
	Remark  string `json:"remark"`
	Content string `json:"content"`
}

type BannerItem struct {
	Logo        string `json:"logo"`
	Id          int    `json:"id"`
	Link        string `json:"link"`
	Alt         string `json:"alt"`
	Description string `json:"description"`
	Type        string `json:"type"` // 增加类型
}

type Banner struct {
	Type string       `json:"type"`
	List []BannerItem `json:"list"`
}

type BannerConfig struct {
	Banners []Banner `json:"banner"`
}

type CacheConfig struct {
	CacheType string `json:"cache_type"`
	Update    bool   `json:"update"`
}

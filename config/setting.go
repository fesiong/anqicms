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
	TemplateName  string       `json:"template_name"`
	TemplateType  int          `json:"template_type"`
	TemplateUrl   string       `json:"template_url"` // template 的静态文件目录
	Language      string       `json:"language"`     // 语言包引用
	ExtraFields   []ExtraField `json:"extra_fields"` // 用户自定义字段
}

type ContentConfig struct {
	RemoteDownload int    `json:"remote_download"`
	FilterOutlink  int    `json:"filter_outlink"`
	UrlTokenType   int    `json:"url_token_type"`
	UseWebp        int    `json:"use_webp"`
	Quality        int    `json:"quality"`
	ResizeImage    int    `json:"resize_image"`
	ResizeWidth    int    `json:"resize_width"`
	ThumbCrop      int    `json:"thumb_crop"`
	ThumbWidth     int    `json:"thumb_width"`
	ThumbHeight    int    `json:"thumb_height"`
	DefaultThumb   string `json:"default_thumb"`
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
}

type ExtraField struct {
	Name   string `json:"name"`
	Value  string `json:"value"`
	Remark string `json:"remark"`
}

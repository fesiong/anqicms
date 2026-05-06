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
	SiteName      string        `json:"site_name"`
	SiteLogo      string        `json:"site_logo"`
	SiteIcp       string        `json:"site_icp"`
	SiteCopyright string        `json:"site_copyright"`
	BaseUrl       string        `json:"base_url"`
	FrontUrl      string        `json:"front_url"`
	MobileUrl     string        `json:"mobile_url"`
	AdminUrl      string        `json:"admin_url"`
	SiteClose     int           `json:"site_close"`
	SiteCloseTips string        `json:"site_close_tips"`
	BanSpider     int           `json:"ban_spider"`
	TemplateName  string        `json:"template_name"`
	TemplateType  int           `json:"template_type"`
	TemplateUrl   string        `json:"template_url"` // template 的静态文件目录
	Language      string        `json:"language"`     // 语言包引用
	ExtraFields   []CustomField `json:"extra_fields"` // 用户自定义字段
	Favicon       string        `json:"favicon"`
	DefaultSite   bool          `json:"default_site"` // 是否是默认站点，每次读取的时候会检查
}

type ContentConfig struct {
	RemoteDownload   int      `json:"remote_download"`
	FilterOutlink    int      `json:"filter_outlink"`
	UrlTokenType     int      `json:"url_token_type"`
	UseWebp          int      `json:"use_webp"`
	ConvertGif       int      `json:"convert_gif"` // 在转换成webp的时候，是否转换gif，1=true
	Quality          int      `json:"quality"`
	ResizeImage      int      `json:"resize_image"`
	ResizeWidth      int      `json:"resize_width"`
	ThumbCrop        int      `json:"thumb_crop"`
	ThumbWidth       int      `json:"thumb_width"`
	ThumbHeight      int      `json:"thumb_height"`
	DefaultThumbType int      `json:"default_thumb_type"` // 0 = default, 3 = category id
	DefaultThumb     string   `json:"default_thumb"`
	DefaultThumbs    []string `json:"default_thumbs"`
	ThumbCategoryId  int      `json:"thumb_category_id"`
	MultiCategory    int      `json:"multi_category"` // 是否启用多分类支持
	Editor           string   `json:"editor"`         // 使用的editor，默认为空，支持 空值|default|markdown
	UseSort          int      `json:"use_sort"`       // 启用文档排序
	MaxPage          int      `json:"max_page"`       // 最大显示页码
	MaxLimit         int      `json:"max_limit"`      // 最大显示条数
}

type IndexConfig struct {
	SeoTitle       string `json:"seo_title"`
	SeoKeywords    string `json:"seo_keywords"`
	SeoDescription string `json:"seo_description"`
	Sep            string `json:"sep"` // 分隔符
}

type ContactConfig struct {
	UserName    string        `json:"user_name"`
	Cellphone   string        `json:"cellphone"`
	Address     string        `json:"address"`
	Email       string        `json:"email"`
	Wechat      string        `json:"wechat"`
	QQ          string        `json:"qq"`
	WhatsApp    string        `json:"whats_app"`
	Facebook    string        `json:"facebook"`
	Twitter     string        `json:"twitter"`
	Tiktok      string        `json:"tiktok"`
	Pinterest   string        `json:"pinterest"`
	Linkedin    string        `json:"linkedin"`
	Instagram   string        `json:"instagram"`
	Youtube     string        `json:"youtube"`
	Qrcode      string        `json:"qrcode"`
	ExtraFields []CustomField `json:"extra_fields"` // 用户自定义字段
	// shop info
	ShopName           string `json:"shop_name"`
	ShopLogo           string `json:"shop_logo"`
	ShopDescription    string `json:"shop_description"`
	BusinessHours      string `json:"business_hours"`
	ShippingTime       int    `json:"shipping_time"`
	ShippingTimeRemark string `json:"shipping_time_remark"`
}

// ShippingUnitItem 计量单位项
type ShippingUnitItem struct {
	Name       string  `json:"name"`        // 长度单位名称，如 cm
	VolumeName string  `json:"volume_name"` // 体积单位名，如 cm³
	Ratio      float64 `json:"ratio"`       // 换算比例
}

type PlaceItem struct {
	CountryCode  string `json:"country_code"`
	ProvinceCode string `json:"province_code"`
	Country      string `json:"country"`
	Province     string `json:"province"`
	City         string `json:"city"`
	Address      string `json:"address"` // 详细地址
	Latitude     string `json:"latitude"`
	Longitude    string `json:"longitude"`
}

type FreeShippingRule struct {
	Type      int     `json:"type"`      // 0=不启用，1=金额满n包邮，2=重量数满n包邮，3=体积数满n包邮， 4=按数量计费满n件包邮
	Threshold float64 `json:"threshold"` // 满额阈值，为了兼容其它单位，这里满额的时候，保留元，不转换为分
}

type ShippingRule struct {
	Id           int64       `json:"id,omitempty"`
	FirstUnit    float64     `json:"first_unit"`
	FirstFee     int64       `json:"first_fee"` // 首单位运费，单位为分
	ContinueUnit float64     `json:"continue_unit"`
	ContinueFee  int64       `json:"continue_fee"` // 续重费用，单位为分
	Regions      []PlaceItem `json:"regions"`
}

// ShippingConfig 物流配置
type ShippingConfig struct {
	Units             []ShippingUnitItem `json:"units"`              // 计量单位列表
	VolumetricDivisor int                `json:"volumetric_divisor"` // 体积单位换算比例，如 cm³/5000 = 1kg，5000 = 1kg
	WeightUnits       []ShippingUnitItem `json:"weight_units"`       // 重量单位列表
	Type              int                `json:"type"`               // 1 = 固定运费， 2 = 按重量计费， 3 = 按体积计费，4 = 按数量计费， 5 = 按金额计费
	// 全局是否免运费
	IsFreeShipping bool `json:"is_free_shipping"` // 是否免费配送 false=自定义运费 true=包邮, 如果设置了包邮，则下面的配置将被忽略。
	// 发货地
	DeliveryPlace PlaceItem `json:"delivery_place"` // 发货地址
	// 满额免运费规则
	FreeShippingRule FreeShippingRule `json:"free_shipping_rule"` // 满额免运费规则
	// 默认运费规则，当不匹配任何地区时，使用默认运费规则
	DefaultRule ShippingRule `json:"default_rule"` // 默认运费规则
	// 自定义地区运费规则
	RegionRules []ShippingRule `json:"region_rules"` // 自定义地区运费规则
}

func (s *ShippingConfig) GetLengthUnit(name string) ShippingUnitItem {
	for _, item := range s.Units {
		if item.Name == name {
			return item
		}
	}
	return ShippingUnitItem{
		Name:       "m",
		VolumeName: "m³",
		Ratio:      1,
	}
}

func (s *ShippingConfig) GetWeightUnit(name string) ShippingUnitItem {
	for _, item := range s.WeightUnits {
		if item.Name == name {
			return item
		}
	}
	return ShippingUnitItem{
		Name:  "kg",
		Ratio: 1,
	}
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

// CurrencyConfig 货币配置
type CurrencyConfig struct {
	Name     string `json:"name"`     // 货币名称
	Currency string `json:"currency"` // 货币代码
	Symbol   string `json:"symbol"`   // 货币符号
}

type ExtraField struct {
	Name    string      `json:"name"`
	Type    string      `json:"type"`
	Value   interface{} `json:"value"`
	Remark  string      `json:"remark"`
	Content string      `json:"content"`
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

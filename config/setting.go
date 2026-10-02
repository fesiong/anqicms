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
	SiteName      string        `json:"site_name"`            // 网站名称
	SiteLogo      string        `json:"site_logo"`            // 网站 Logo。
	SiteIcp       string        `json:"site_icp"`             // 网站 ICP 备案号。
	SiteCopyright string        `json:"site_copyright"`       // 网站版权信息。
	BaseUrl       string        `json:"base_url"`             // 网站网址。
	FrontUrl      string        `json:"front_url"`            // 前台网址，前后端分离的站可设置独立的前台网址。
	MobileUrl     string        `json:"mobile_url"`           // 手机端网址，模板是 PC+手机 类型的需要设置
	AdminUrl      string        `json:"admin_url"`            // 后台网址。
	SiteClose     int           `json:"site_close"`           // 是否关闭网站：0=否，1=是
	SiteCloseTips string        `json:"site_close_tips"`      // 关闭网站页面上提示的内容。
	BanSpider     int           `json:"ban_spider"`           // 是否禁止搜索引擎抓取。
	TemplateName  string        `json:"template_name"`        // 模板包名
	TemplateType  int           `json:"template_type"`        // 模板类型：0=自适应，1=代码适配，2=PC+手机独立模板
	TemplateUrl   string        `json:"template_url" ast:"-"` // template 的静态文件目录
	Language      string        `json:"language"`             // 站点语言
	ExtraFields   []CustomField `json:"extra_fields" ast:"-"` // 用户自定义字段
	Favicon       string        `json:"favicon" ast:"-"`
	DefaultSite   bool          `json:"default_site" ast:"-"` // 是否是默认站点，每次读取的时候会检查
}

type ContentConfig struct {
	RemoteDownload   int      `json:"remote_download"`       // 下载远程图片：0=否，1=是
	FilterOutlink    int      `json:"filter_outlink"`        // 外部链接处理：0=保留链接，1=移除链接，2=添加 Nofollow
	UrlTokenType     int      `json:"url_token_type"`        // URL 别名生成格式：0=全拼音/单词，1=拼音/单词首字母
	UseWebp          int      `json:"use_webp"`              // 是否启用webp格式：0=否，1=是
	MatchTag         int      `json:"match_tag"`             // 是否在文章创建时，自动匹配标签：0=否，1=是
	ConvertGif       int      `json:"convert_gif"`           // 在转换成webp的时候，是否转换gif：0=否，1=是
	Quality          int      `json:"quality"`               // 图片质量：10-100
	ResizeImage      int      `json:"resize_image"`          // 是否启用大图片压缩：0=否，1=是
	ResizeWidth      int      `json:"resize_width"`          // 图片压缩宽度：默认1200
	ThumbCrop        int      `json:"thumb_crop"`            // 缩略图裁剪方式：0 = 按最长边等比缩放, 1 = 按最长边补白，2 = 按最短边裁剪
	ThumbWidth       int      `json:"thumb_width"`           // 缩略图宽度：默认250
	ThumbHeight      int      `json:"thumb_height"`          // 缩略图高度：默认250
	DefaultThumbType int      `json:"default_thumb_type"`    // 如果文档没有缩略图，则可以使用默认缩略图：0 = 从选定的图片中随机选择, 3 = 从指定图片分类中随机选择
	DefaultThumb     string   `json:"default_thumb" ast:"-"` // 默认缩略图
	DefaultThumbs    []string `json:"default_thumbs"`        // 默认缩略图列表
	ThumbCategoryId  int      `json:"thumb_category_id"`     // 默认缩略图分类ID
	MultiCategory    int      `json:"multi_category"`        // 是否启用多分类支持：0=否，1=是
	Editor           string   `json:"editor"`                // 使用的editor，默认为空，支持 空值|default=富文本编辑器，markdown=Markdown编辑器，simple=简易编辑器
	UseSort          int      `json:"use_sort"`              // 启用文档排序：0=否，1=是
	MaxPage          int      `json:"max_page"`              // 最大显示页码：默认1000
	MaxLimit         int      `json:"max_limit"`             // 每页最大显示条数：默认100
}

type IndexConfig struct {
	SeoTitle       string `json:"seo_title"`       // 首页 SEO 标题
	SeoKeywords    string `json:"seo_keywords"`    // 首页 SEO 关键词
	SeoDescription string `json:"seo_description"` // 首页 SEO 描述
	Sep            string `json:"sep"`             // 标题分隔符
}

type ContactConfig struct {
	UserName    string        `json:"user_name"`            // 联系人
	Cellphone   string        `json:"cellphone"`            // 联系电话
	Address     string        `json:"address"`              // 联系地址
	Email       string        `json:"email"`                // 联系邮箱
	Wechat      string        `json:"wechat"`               // 联系微信
	QQ          string        `json:"qq"`                   // 联系QQ
	WhatsApp    string        `json:"whats_app"`            // 联系WhatsApp
	Facebook    string        `json:"facebook"`             // 联系Facebook
	Twitter     string        `json:"twitter"`              // 联系Twitter
	Tiktok      string        `json:"tiktok"`               // 联系TikTok
	Pinterest   string        `json:"pinterest"`            // 联系Pinterest
	Linkedin    string        `json:"linkedin"`             // 联系LinkedIn
	Instagram   string        `json:"instagram"`            // 联系Instagram
	Youtube     string        `json:"youtube"`              // 联系YouTube
	Qrcode      string        `json:"qrcode"`               // 联系二维码，一般是微信二维码
	ExtraFields []CustomField `json:"extra_fields" ast:"-"` // 用户自定义字段
}

type SafeConfig struct {
	Captcha          int    `json:"captcha"`           // 是否启用登陆/留言/评论验证码：0=否，1=是
	DailyLimit       int    `json:"daily_limit"`       // 留言/评论同IP每日提交限制次数
	ContentLimit     int    `json:"content_limit"`     // 提交留言内容至少字数
	IntervalLimit    int    `json:"interval_limit"`    // 留言/评论间隔限制秒数
	ContentForbidden string `json:"content_forbidden"` // 留言敏感词过滤，包含这些敏感词将禁止提交
	IPForbidden      string `json:"ip_forbidden"`      // 这些IP访问的将会被拒绝，一行一个IP，支持IP段
	UAForbidden      string `json:"ua_forbidden"`      // 这些UA将会被拒绝，一行一个UA关键词
	APIOpen          int    `json:"api_open"`          // 是否启用API：0=否，1=是
	APIPublish       int    `json:"api_publish"`       // 是否启用API发布：0=否，1=是
	AdminCaptchaOff  int    `json:"admin_captcha_off"` // 是否关闭管理员验证码：0=否，1=是
}

type BannerItem struct {
	Logo        string `json:"logo"`          // Banner 图片地址
	Id          int    `json:"id"`            // Banner ID
	Link        string `json:"link"`          // Banner 链接地址
	Alt         string `json:"alt"`           // Banner Alt 文本
	Title       string `json:"title" ast:"-"` // 兼容Alt
	Description string `json:"description"`   // Banner 描述
	Type        string `json:"type"`          // Banner 分组，默认 default
}

type BannerItemDeleteRequest struct {
	Id   int    `json:"id"`   // Banner ID
	Type string `json:"type"` // Banner 分组，默认 default
}

type Banner struct {
	Type string       `json:"type"` // Banner 分组，默认 default
	List []BannerItem `json:"list"` // Banner 列表
}

type BannerConfig struct {
	Banners []Banner `json:"banner"`
}

// CacheConfig 缓存配置
type CacheConfig struct {
	CacheType string `json:"cache_type"` // 缓存类型：0=自动选择，1=使用文件缓存，2=使用内存缓存
	Update    bool   `json:"update"`     // 是否更新缓存类型，为 false 时执行清理缓存。
}

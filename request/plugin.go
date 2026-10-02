package request

import "kandaoni.com/anqicms/config"

type PluginRobotsConfig struct {
	Robots string `json:"robots"` // robots.txt 内容
}

type PluginLink struct {
	Id       uint   `json:"id"`        // 链接 ID，新增时不需要传
	Title    string `json:"title"`     // 链接名称
	Link     string `json:"link"`      // 链接地址
	BackLink string `json:"back_link"` // 对方放置本站链接的页面URL
	MyTitle  string `json:"my_title"`  // 我的关键词
	MyLink   string `json:"my_link"`   // 我的链接
	Contact  string `json:"contact"`   // 对方联系方式
	Remark   string `json:"remark"`    // 备注
	Nofollow uint   `json:"nofollow"`  // 是否添加 nofollow：0=否，1=是
	Sort     uint   `json:"sort"`      // 排序，数值越小越靠前
	Status   uint   `json:"status" ast:"-"`
	Logo     string `json:"logo"` // 站点图标
}

type PluginLinkDeleteRequest struct {
	Id uint `json:"id"` // 链接 ID
}

type PluginComment struct {
	Id        uint   `json:"id"`
	ArchiveId int64  `json:"archive_id"`
	UserId    uint   `json:"user_id"`
	UserName  string `json:"user_name"`
	Email     string `json:"email"`
	Ip        string `json:"ip"`
	VoteCount uint   `json:"vote_count"`
	Content   string `json:"content"`
	ParentId  uint   `json:"parent_id"`
	ToUid     uint   `json:"to_uid"`
	Status    uint   `json:"status"`
	CaptchaId string `json:"captcha_id"`
	Captcha   string `json:"captcha"`
}

type PluginCommentDeleteRequest struct {
	Id uint `json:"id"` // 评论 ID
}

type PluginCommentApprovalRequest struct {
	Id     uint   `json:"id"`     // 评论 ID
	Status uint   `json:"status"` // 状态：0=待审核，1=正常
	Ids    []uint `json:"ids"`    // 批量 ID，和 ID 二选一
}

type PluginCommentUpdateRequest struct {
	Id       uint   `json:"id"`        // 评论 ID
	Status   uint   `json:"status"`    // 状态：0=待审核，1=正常
	UserName string `json:"user_name"` // 用户名
	Content  string `json:"content"`   // 评论内容
	Ip       string `json:"ip"`        // 评论IP，为空时使用请求端地址。
}

type PluginAnchor struct {
	Id     uint   `json:"id"`     // 锚文本 ID，新增时不需要传
	Title  string `json:"title"`  // 锚文本名称，需唯一
	Link   string `json:"link"`   // 锚文本链接
	Weight int    `json:"weight"` // 锚文本权重
}

type PluginAnchorReplaceRequest struct {
	Id uint `json:"id"` // 锚文本 ID

}

type PluginAnchorDelete struct {
	Id  uint   `json:"id"`
	Ids []uint `json:"ids"`
}

type PluginAnchorAddFromTitle struct {
	Type string `json:"type"` // 导入类型：category=分类，archive=文档，tag=标签
	Ids  []uint `json:"ids"`  // 类型关联的 ID
}

type PluginGuestbookStatus struct {
	Id     uint   `json:"id"`
	Ids    []uint `json:"ids"`
	Status int    `json:"status"`
}

type PluginGuestbookDelete struct {
	Id  uint   `json:"id"`  // 留言 ID（删除单条时传）
	Ids []uint `json:"ids"` // 留言 ID 列表（批量删除时传）
}

type PluginKeyword struct {
	Id         uint   `json:"id"`
	Title      string `json:"title"`
	CategoryId uint   `json:"category_id"`
}

type PluginKeywordDelete struct {
	Id  uint   `json:"id"`
	Ids []uint `json:"ids"`
	All bool   `json:"all"`
}

type PluginFileUploadDelete struct {
	Hash string `json:"hash"` // 文件 hash
}

type PluginMaterial struct {
	Id         uint   `json:"id"`          // 素材 ID，新增时不需要传
	Title      string `json:"title"`       // 素材名称
	CategoryId uint   `json:"category_id"` // 素材分类 ID
	Content    string `json:"content"`     // 素材内容
	Status     uint   `json:"status"`      // 素材状态：0=禁用，1=启用
	AutoUpdate uint   `json:"auto_update"` // 素材更新时，是否自动更新引用该素材的文档
}

type PluginMaterialDelete struct {
	Id uint `json:"id"` // 素材 ID
}

type PluginMaterialCategory struct {
	Id    uint   `json:"id"`    // 分类 ID，新增时不需要传
	Title string `json:"title"` // 分类名称
}

type PluginMaterialCategoryDelete struct {
	Id uint `json:"id"` // 分类 ID
}

type PluginMaterialImportRequest struct {
	Materials []*PluginMaterial `json:"materials"` // 素材列表
}

type PluginTag struct {
	Id          uint                   `json:"id"`             // 标签 ID
	Title       string                 `json:"title"`          // 标签名称
	CategoryId  uint                   `json:"category_id"`    // 分类 ID
	UrlToken    string                 `json:"url_token"`      // URL 别名
	SeoTitle    string                 `json:"seo_title"`      // SEO 标题
	Keywords    string                 `json:"keywords"`       // 标签关键词
	Description string                 `json:"description"`    // 描述
	FirstLetter string                 `json:"first_letter"`   // 索引字母，A-Z
	Content     string                 `json:"content"`        // 标签内容
	Logo        string                 `json:"logo"`           // 标签图标地址
	Template    string                 `json:"template"`       // 标签自定义模板
	Status      uint                   `json:"status" ast:"-"` // 状态：1=正常，0=隐藏
	Extra       map[string]interface{} `json:"extra"`          // 标签的自定义字段内容
	UpdateAll   bool                   `json:"update_all" ast:"-"`
}

type PluginTagDeleteRequest struct {
	Id uint `json:"id"` // 标签 ID
}

type PluginRedirectRequest struct {
	Id      uint   `json:"id"`       // 链接ID，新增的时候不用传
	FromUrl string `json:"from_url"` // 访问地址（URI）
	ToUrl   string `json:"to_url"`   // 跳转地址
}

type PluginRedirectDeleteRequest struct {
	Id uint `json:"id"` // 链接ID
}

type PluginRedirectsRequest struct {
	Urls []PluginRedirectRequest `json:"urls"`
}

type PluginBackupRequest struct {
	Name   string `json:"name"`   // 备份文件名称
	Remark string `json:"remark"` // 备份文件备注
}

type PluginCleanupRequest struct {
	CleanUploads bool `json:"clean_uploads"` // 是否清理上传文件
}

type PluginRestoreRequest struct {
	Name string `json:"name"` // 备份文件名称
}

type PluginReplaceRequest struct {
	ReplaceTag bool                    `json:"replace_tag"` // 是否替换HTML标签内容
	Places     []string                `json:"places"`      // 替换类型：setting|archive|category|tag|anchor|keyword|comment|attachment|nav|link|redirect|place|guestbook|template
	Keywords   []config.ReplaceKeyword `json:"keywords"`    // 替换关键词组，[{"from": "A","to":"B"}]
}

type PluginHtmlCachePushRequest struct {
	All   bool     `json:"all"`
	Paths []string `json:"paths"`
}

type PluginTestSendmailRequest struct {
	Recipient string `json:"recipient"` // 收件人邮箱地址
	Subject   string `json:"subject"`   // 邮件主题
	Message   string `json:"message"`   // 邮件正文内容
}

type PluginMultiLangSiteRequest struct {
	Id           uint   `json:"id"`            // 多语言站点 ID（站点数据类型为共用主站数据时不用传）
	ParentId     uint   `json:"parent_id"`     // 父级多语言站点 ID
	Language     string `json:"language"`      // 语言
	LanguageIcon string `json:"language_icon"` // 语言图标
	BaseUrl      string `json:"base_url"`      // 站点根 URL（多语言路由类型为 domain 的时候需要填写）
}

type PluginMultiLangSiteSyncRequest struct {
	Id       uint `json:"id"`
	ParentId uint `json:"parent_id" ast:"-"`
	Focus    bool `json:"focus" ast:"-"`
}

type PluginMultiLangSiteDeleteRequest struct {
	Id       uint   `json:"id"`
	Language string `json:"language"`
}

type PluginLimiterRemoveIPRequest struct {
	Ip string `json:"ip"` // 要解除访问限制的 IP 地址
}

type PluginMultiLangCacheRemoveRequest struct {
	Uris []string `json:"uris"` // 缓存 URI列表
	All  bool     `json:"all"`  // 是否删除所有缓存
}

type SubscriberRequest struct {
	Id         int64  `json:"id"`          // 订阅用户ID，大于 0 时为更新。
	Email      string `json:"email"`       // 订阅用户 Email 地址
	Remark     string `json:"remark"`      // 备注信息
	CategoryId int64  `json:"category_id"` // 订阅用户分类 ID
	Status     int    `json:"status"`      // 用户状态：1=有效，0=无效
}

type SubscriberDeleteRequest struct {
	Id int64 `json:"id"` // 订阅用户ID
}

type SubscriberMailRequest struct {
	Type       string   `json:"type"`        // 发送类别：all=向全部订阅用户发送，category=向指定分类发送，email=向指定 Email 列表发送
	CategoryId int64    `json:"category_id"` // 分类 ID，type=category 时提供
	Emails     []string `json:"emails"`      // Email 列表，type=email 时提供
	Subject    string   `json:"subject"`     // 邮件标题
	Content    string   `json:"content"`     // 邮件内容，支持使用 anqicms 标签
}

type SubscriberCategoryRequest struct {
	Id    int64  `json:"id"`    // 订阅分类 ID
	Title string `json:"title"` // 订阅分类名称
}

type PluginPushUrlsRequest struct {
	Urls []string `json:"urls"` // 本站的文档链接
}

type TranslateTextLogDeleteRequest struct {
	Id  uint `json:"id"`            // 记录 ID
	All bool `json:"all,omitempty"` // 是否全部
}

type TranslateTextLog struct {
	Id         uint   `json:"id"`          // 记录 ID
	Md5        string `json:"md5" ast:"-"` // md5 的来源是 language-to_language-Text
	Language   string `json:"language"`    // 原语言，默认取主站语言
	ToLanguage string `json:"to_language"` // 译文语言，对应多语言站点语言
	Text       string `json:"text"`        // 原文
	Translated string `json:"translated"`  // 译文
}

type SensitiveWordsRequest []string

type SensitiveWordsCheckRequest struct {
	Title   string `json:"title"`   // 待检查的标题
	Content string `json:"content"` // 待检查的内容
}

package request

import "kandaoni.com/anqicms/config"

type PluginRobotsConfig struct {
	Robots string `json:"robots"`
}

type PluginLink struct {
	Id       uint   `json:"id"`
	Title    string `json:"title"`
	Link     string `json:"link"`
	BackLink string `json:"back_link"`
	MyTitle  string `json:"my_title"`
	MyLink   string `json:"my_link"`
	Contact  string `json:"contact"`
	Remark   string `json:"remark"`
	Nofollow uint   `json:"nofollow"`
	Sort     uint   `json:"sort"`
	Status   uint   `json:"status"`
}

type PluginComment struct {
	Id        uint   `json:"id"`
	ArchiveId int64  `json:"archive_id"`
	UserId    uint   `json:"user_id"`
	UserName  string `json:"user_name"`
	Ip        string `json:"ip"`
	VoteCount uint   `json:"vote_count"`
	Content   string `json:"content"`
	ParentId  uint   `json:"parent_id"`
	ToUid     uint   `json:"to_uid"`
	Status    uint   `json:"status"`
	CaptchaId string `json:"captcha_id"`
	Captcha   string `json:"captcha"`

	// 批量更新
	Ids []uint `json:"ids"`
}

type PluginAnchor struct {
	Id     uint   `json:"id"`
	Title  string `json:"title"`
	Link   string `json:"link"`
	Weight int    `json:"weight"`
}

type PluginAnchorDelete struct {
	Id  uint   `json:"id"`
	Ids []uint `json:"ids"`
}

type PluginAnchorAddFromTitle struct {
	Type string `json:"type"`
	Ids  []uint `json:"ids"`
}

type PluginGuestbookDelete struct {
	Id  uint   `json:"id"`
	Ids []uint `json:"ids"`
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
	Hash string `json:"hash"`
}

type PluginMaterial struct {
	Id         uint   `json:"id"`
	Title      string `json:"title"`
	CategoryId uint   `json:"category_id"`
	Content    string `json:"content"`
	Status     uint   `json:"status"`
	AutoUpdate uint   `json:"auto_update"`
}

type PluginMaterialCategory struct {
	Id    uint   `json:"id"`
	Title string `json:"title"`
}

type PluginMaterialImportRequest struct {
	Materials []*PluginMaterial `json:"materials"`
}

type PluginTag struct {
	Id          uint   `json:"id"`
	Title       string `json:"title"`
	CategoryId  uint   `json:"category_id"`
	UrlToken    string `json:"url_token"`
	SeoTitle    string `json:"seo_title"`
	Keywords    string `json:"keywords"`
	Description string `json:"description"`
	FirstLetter string `json:"first_letter"`
	Content     string `json:"content"`
	Logo        string `json:"logo"`
	Template    string `json:"template"`
	Status      uint   `json:"status"`
}

type PluginRedirectRequest struct {
	Id      uint   `json:"id"`
	FromUrl string `json:"from_url"`
	ToUrl   string `json:"to_url"`
}

type PluginRedirectsRequest struct {
	Urls []PluginRedirectRequest `json:"urls"`
}

type PluginBackupRequest struct {
	Name         string `json:"name"`
	CleanUploads bool   `json:"clean_uploads"`
}

type PluginReplaceRequest struct {
	ReplaceTag bool                    `json:"replace_tag"`
	Places     []string                `json:"places"`
	Keywords   []config.ReplaceKeyword `json:"keywords"`
}

type PluginHtmlCachePushRequest struct {
	All   bool     `json:"all"`
	Paths []string `json:"paths"`
}

type PluginTestSendmailRequest struct {
	Recipient string `json:"recipient"`
	Subject   string `json:"subject"`
	Message   string `json:"message"`
}

type PluginMultiLangSiteRequest struct {
	Id           uint   `json:"id"`
	ParentId     uint   `json:"parent_id"`
	Language     string `json:"language"`
	LanguageIcon string `json:"language_icon"`
	BaseUrl      string `json:"base_url"`
	Focus        bool   `json:"focus"`
}

type PluginLimiterRemoveIPRequest struct {
	Ip string `json:"ip"`
}

type PluginMultiLangCacheRemoveRequest struct {
	Uris []string `json:"uris"`
	All  bool     `json:"all"`
}

package request

import "kandaoni.com/anqicms/config"

type SystemConfig struct {
	SiteName      string              `json:"site_name"`
	SiteLogo      string              `json:"site_logo"`
	SiteIcp       string              `json:"site_icp"`
	SiteCopyright string              `json:"site_copyright"`
	AdminUrl      string              `json:"admin_url"`
	SiteClose     int                 `json:"site_close"`
	SiteCloseTips string              `json:"site_close_tips"`
	TemplateName  string              `json:"template_name"`
	BaseUrl       string              `json:"base_url"`
	MobileUrl     string              `json:"mobile_url"`
	TemplateType  int                 `json:"template_type"`
	Language      string              `json:"language"`
	ExtraFields   []config.ExtraField `json:"extra_fields"` // 用户自定义字段
}

type ContentConfig struct {
	RemoteDownload int    `json:"remote_download"`
	FilterOutlink  int    `json:"filter_outlink"`
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

type NavConfig struct {
	Id          uint   `json:"id"`
	Title       string `json:"title"`
	SubTitle    string `json:"sub_title"`
	Description string `json:"description"`
	ParentId    uint   `json:"parent_id"`
	NavType     uint   `json:"nav_type"`
	PageId      uint   `json:"page_id"`
	TypeId      uint   `json:"type_id"`
	Link        string `json:"link"`
	Sort        uint   `json:"sort"`
	Status      uint   `json:"status"`
}

type NavTypeRequest struct {
	Id    uint   `json:"id"`
	Title string `json:"title"`
}

type ContactConfig struct {
	UserName    string              `json:"user_name"`
	Cellphone   string              `json:"cellphone"`
	Address     string              `json:"address"`
	Email       string              `json:"email"`
	Wechat      string              `json:"wechat"`
	Qrcode      string              `json:"qrcode"`
	ExtraFields []config.ExtraField `json:"extra_fields"` // 用户自定义字段
}

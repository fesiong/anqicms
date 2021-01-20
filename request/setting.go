package request

type SystemConfig struct {
	SiteName      string `json:"site_name"`
	SiteLogo      string `json:"site_logo"`
	SiteIcp       string `json:"site_icp"`
	SiteCopyright string `json:"site_copyright"`
	AdminUri      string `json:"admin_uri"`
	SiteClose     int    `json:"site_close"`
	SiteCloseTips string `json:"site_close_tips"`
}

type ContentConfig struct {
	RemoteDownload int    `json:"remote_download"`
	FilterOutlink  int    `json:"filter_outlink"`
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
	Link        string `json:"link"`
	Sort        uint   `json:"sort"`
	Status      uint   `json:"status"`
}

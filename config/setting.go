package config

type systemConfig struct {
	SiteName      string `json:"site_name"`
	SiteLogo      string `json:"site_logo"`
	SiteIcp       string `json:"site_icp"`
	SiteCopyright string `json:"site_copyright"`
	BaseUrl       string `json:"base_url"`
	AdminUri      string `json:"admin_uri"`
	SiteClose     int    `json:"site_close"`
	SiteCloseTips string `json:"site_close_tips"`
}

type contentConfig struct {
	RemoteDownload int    `json:"remote_download"`
	FilterOutlink  int    `json:"filter_outlink"`
	ResizeImage    int    `json:"resize_image"`
	ResizeWidth    int   `json:"resize_width"`
	ThumbCrop      int    `json:"thumb_crop"`
	ThumbWidth     int   `json:"thumb_width"`
	ThumbHeight    int   `json:"thumb_height"`
	DefaultThumb   string `json:"default_thumb"`
}

type indexConfig struct {
	SeoTitle       string `json:"seo_title"`
	SeoKeywords    string `json:"seo_keywords"`
	SeoDescription string `json:"seo_description"`
}

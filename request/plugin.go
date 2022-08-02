package request

import "kandaoni.com/anqicms/config"

type PluginPushConfig struct {
	BaiduApi string            `json:"baidu_api"`
	BingApi  string            `json:"bing_api"`
	JsCode   string            `json:"js_code"`
	JsCodes  []config.CodeItem `json:"js_codes"`
}

type PluginRobotsConfig struct {
	Robots string `json:"robots"`
}

type PluginSitemapConfig struct {
	AutoBuild int    `json:"auto_build"`
	Type      string `json:"type"`
}

type PluginRewriteConfig struct {
	Mode   int    `json:"mode"`
	Patten string `json:"patten"`
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
	ArchiveId uint   `json:"archive_id"`
	UserId    uint   `json:"user_id"`
	UserName  string `json:"user_name"`
	Ip        string `json:"ip"`
	VoteCount uint   `json:"vote_count"`
	Content   string `json:"content"`
	ParentId  uint   `json:"parent_id"`
	ToUid     uint   `json:"to_uid"`
	Status    uint   `json:"status"`
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

type PluginAnchorSetting struct {
	AnchorDensity int `json:"anchor_density"`
	ReplaceWay    int `json:"replace_way"`
	KeywordWay    int `json:"keyword_way"`
}

type PluginGuestbookSetting struct {
	ReturnMessage string                `json:"return_message"`
	Fields        []*config.CustomField `json:"fields"`
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

type PluginSendmail struct {
	Server    string `json:"server"`
	UseSSL    int    `json:"use_ssl"`
	Port      int    `json:"port"`
	Account   string `json:"account"`
	Password  string `json:"password"`
	Recipient string `json:"recipient"`
}

type PluginMaterialImportRequest struct {
	Materials []*PluginMaterial `json:"materials"`
}

type PluginTag struct {
	Id          uint   `json:"id"`
	Title       string `json:"title"`
	UrlToken    string `json:"url_token"`
	SeoTitle    string `json:"seo_title"`
	Keywords    string `json:"keywords"`
	Description string `json:"description"`
	FirstLetter string `json:"first_letter"`
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

type PluginStorageConfigRequest struct {
	StorageUrl  string `json:"storage_url"`
	StorageType string `json:"storage_type"`
	KeepLocal   bool   `json:"keep_local"`

	AliyunEndpoint        string `json:"aliyun_endpoint"`
	AliyunAccessKeyId     string `json:"aliyun_access_key_id"`
	AliyunAccessKeySecret string `json:"aliyun_access_key_secret"`
	AliyunBucketName      string `json:"aliyun_bucket_name"`

	TencentSecretId  string `json:"tencent_secret_id"`
	TencentSecretKey string `json:"tencent_secret_key"`
	TencentBucketUrl string `json:"tencent_bucket_url"`

	QiniuAccessKey string `json:"qiniu_access_key"`
	QiniuSecretKey string `json:"qiniu_secret_key"`
	QiniuBucket    string `json:"qiniu_bucket"`
	QiniuRegion    string `json:"qiniu_region"`
}

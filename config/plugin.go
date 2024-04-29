package config

import (
	"fmt"
	"strings"
)

type CodeItem struct {
	Name  string `json:"name"`
	Value string `json:"value"`
}

type PluginPushConfig struct {
	BaiduApi   string     `json:"baidu_api"`
	BingApi    string     `json:"bing_api"`
	GoogleJson string     `json:"google_json"`
	JsCode     string     `json:"js_code"`
	JsCodes    []CodeItem `json:"js_codes"`
}

type PluginSitemapConfig struct {
	AutoBuild   int    `json:"auto_build"`
	Type        string `json:"type"`
	UpdatedTime int64  `json:"updated_time"`
	SitemapURL  string `json:"sitemap_url"`
}

type PluginAnchorConfig struct {
	AnchorDensity int `json:"anchor_density"`
	ReplaceWay    int `json:"replace_way"`
	KeywordWay    int `json:"keyword_way"`
}

type PluginGuestbookConfig struct {
	ReturnMessage string         `json:"return_message"`
	Fields        []*CustomField `json:"fields"`
}

type CustomField struct {
	Name        string   `json:"name"`
	FieldName   string   `json:"field_name"`
	Type        string   `json:"type"`
	Required    bool     `json:"required"`
	IsSystem    bool     `json:"is_system"`
	IsFilter    bool     `json:"is_filter"`
	FollowLevel bool     `json:"follow_level"`
	Content     string   `json:"content"`
	Items       []string `json:"-"`
}

type PluginUploadFile struct {
	Hash        string `json:"hash"`
	FileName    string `json:"file_name"`
	CreatedTime int64  `json:"created_time"`
	Link        string `json:"link"`
}

type PluginSendmail struct {
	Server    string `json:"server"`
	UseSSL    int    `json:"use_ssl"`
	Port      int    `json:"port"`
	Account   string `json:"account"`
	Password  string `json:"password"`
	Recipient string `json:"recipient"`

	AutoReply    bool   `json:"auto_reply"`
	ReplySubject string `json:"reply_subject"`
	ReplyMessage string `json:"reply_message"` // 自动回复内容
	SendType     []int  `json:"send_type"`
}

type PluginImportApiConfig struct {
	Token     string `json:"token"`      // 文档导入token
	LinkToken string `json:"link_token"` // 友情链接token
}

type PluginStorageConfig struct {
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

	UpyunBucket   string `json:"upyun_bucket"`
	UpyunOperator string `json:"upyun_operator"`
	UpyunPassword string `json:"upyun_password"`

	FTPHost     string `json:"ftp_host"`
	FTPPort     int    `json:"ftp_port"`
	FTPUsername string `json:"ftp_username"`
	FTPPassword string `json:"ftp_password"`
	FTPWebroot  string `json:"ftp_webroot"`

	SSHHost       string `json:"ssh_host"`
	SSHPort       int    `json:"ssh_port"`
	SSHUsername   string `json:"ssh_username"`
	SSHPassword   string `json:"ssh_password"`
	SSHPrivateKey string `json:"ssh_private_key"` // 私钥文件名
	SSHWebroot    string `json:"ssh_webroot"`
}

type PluginFulltextConfig struct {
	Open       bool   `json:"open"`
	UseContent bool   `json:"use_content"` // 是否索引内容
	Modules    []uint `json:"modules"`
}

type PluginTitleImageConfig struct {
	Open      bool   `json:"open"`
	DrawSub   bool   `json:"draw_sub"`
	BgImage   string `json:"bg_image"`
	FontPath  string `json:"font_path"`
	FontSize  int    `json:"font_size"`
	FontColor string `json:"font_color"`
	Width     int    `json:"width"`
	Height    int    `json:"height"`
	Noise     bool   `json:"noise"`
}

type PluginHtmlCache struct {
	Open          bool   `json:"open"`
	IndexCache    int64  `json:"index_cache"`     // 首页缓存时间
	ListCache     int64  `json:"category_cache"`  // 列表页缓存时间
	DetailCache   int64  `json:"detail_cache"`    // 详情页缓存时间
	LastBuildTime int64  `json:"last_build_time"` // 上一次手动生成时间
	LastPushTime  int64  `json:"last_push_time"`  // 上一次手动推送时间
	ErrorMsg      string `json:"error_msg"`
	PluginStorageConfig
}

type PluginTimeFactor struct {
	Open        bool     `json:"open"`
	ModuleIds   []int64  `json:"module_ids"`
	Types       []string `json:"types"`
	StartDay    int      `json:"start_day"`
	EndDay      int      `json:"end_day"`
	CategoryIds []int64  `json:"category_ids"`
	DoPublish   bool     `json:"do_publish"`
	ReleaseOpen bool     `json:"release_open"`
	DailyLimit  int      `json:"daily_limit"`
	StartTime   int      `json:"start_time"`
	EndTime     int      `json:"end_time"`
	TodayCount  int      `json:"today_count"`
	LastSent    int64    `json:"last_sent"`
}

type PluginInterference struct {
	Open              bool `json:"open"`
	Mode              int  `json:"mode"`
	DisableSelection  bool `json:"disable_selection"`
	DisableCopy       bool `json:"disable_copy"`
	DisableRightClick bool `json:"disable_right_click"`
}

type PluginWatermark struct {
	Open      bool   `json:"open"`
	Type      int    `json:"type"` // 0 image, 1 text
	ImagePath string `json:"image_path"`
	Text      string `json:"text,omitempty"`
	FontPath  string `json:"font_path"`
	Size      int    `json:"size"`
	Color     string `json:"color"`
	Position  int    `json:"position"` // 5 居中，1 左上角，3 右上角 7 左下角 9 右下角
	Opacity   int    `json:"opacity"`
	MinSize   int    `json:"min_size"`
}

func (g *CustomField) SplitContent() []string {
	var items []string
	contents := strings.Split(g.Content, "\n")
	for _, v := range contents {
		v = strings.TrimSpace(v)
		if v != "" {
			items = append(items, v)
		}
	}

	g.Items = items

	return items
}

// CheckSetFilter 支付允许筛选
func (g *CustomField) CheckSetFilter() bool {
	if g.Type != CustomFieldTypeRadio && g.Type != CustomFieldTypeCheckbox && g.Type != CustomFieldTypeSelect {
		g.IsFilter = false
		return false
	}
	if g.FollowLevel {
		g.IsFilter = false
		return false
	}

	return true
}

func (g *CustomField) GetFieldColumn() string {
	column := fmt.Sprintf("`%s`", g.FieldName)

	if g.Type == CustomFieldTypeNumber {
		column += " int(10)"
	} else if g.Type == CustomFieldTypeTextarea || g.Type == CustomFieldTypeEditor {
		column += " text"
	} else {
		// mysql 5.6 下，utf8mb4 索引只能用190
		column += " varchar(190)"
	}

	//if g.Required {
	//	column += " NOT NULL"
	//} else {
	//	column += " DEFAULT NULL"
	//}
	// 因为是后插值，因此这里默认都是null
	column += " DEFAULT NULL"

	return column
}

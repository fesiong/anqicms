package config

type pluginPushConfig struct {
	BaiduApi string `json:"baidu_api"`
	BingApi  string `json:"bing_api"`
	JsCode   string `json:"js_code"`
}

type pluginSitemapConfig struct {
	AutoBuild   int   `json:"auto_build"`
	UpdatedTime int64 `json:"updated_time"`
}

type pluginAnchorConfig struct {
	AnchorDensity int `json:"anchor_density"`
	ReplaceWay    int `json:"replace_way"`
	KeywordWay    int `json:"keyword_way"`
}

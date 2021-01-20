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

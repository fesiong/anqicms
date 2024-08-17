package config

type PluginWeappConfig struct {
	AppID     string `json:"app_id"`
	AppSecret string `json:"app_secret"`
	//公众号存在的部分
	Token          string `json:"token"`
	EncodingAESKey string `json:"encoding_aes_key"`
	VerifyKey      string `json:"verify_key"`
	VerifyMsg      string `json:"verify_msg"`
	ServerUrl      string `json:"server_url"`
}

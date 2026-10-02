package config

type PluginWeappConfig struct {
	AppID          string `json:"app_id"`             // APP ID
	AppSecret      string `json:"app_secret"`         // APP Secret
	Token          string `json:"token"`              // 服务号Token，消息推送用
	EncodingAESKey string `json:"encoding_aes_key"`   // 服务号EncodingAESKey，消息推送用
	VerifyKey      string `json:"verify_key"`         // 验证码关键词，用公众号下发验证码用
	VerifyMsg      string `json:"verify_msg"`         // 验证码信息模板，模板需要包含 `{code}`
	ServerUrl      string `json:"server_url" ast:"-"` // 服务器地址
}

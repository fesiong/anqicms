package config

const (
	SmsProviderAliyun  = "aliyun"
	SmsProviderTencent = "tencent"
	SmsProviderTwilio  = "twilio"

	SmsRegionDomestic      = "domestic"
	SmsRegionInternational = "international"
)

type PluginSmsConfig struct {
	Provider string `json:"provider"` // "" = 关闭；aliyun / tencent / twilio

	Aliyun  SmsAliyunConfig  `json:"aliyun"`
	Tencent SmsTencentConfig `json:"tencent"`
	Twilio  SmsTwilioConfig  `json:"twilio"`

	// 验证码参数（决策 15：全部后台可配）
	CodeLength         int `json:"code_length"`           // 4..8，默认 6
	CodeTTL            int `json:"code_ttl"`              // 秒，默认 300
	ResendInterval     int `json:"resend_interval"`       // 秒，默认 60
	MaxVerifyAttempts  int `json:"max_verify_attempts"`   // 默认 5
	DailyLimitPerPhone int `json:"daily_limit_per_phone"` // 默认 10
	HourlyLimitPerIp   int `json:"hourly_limit_per_ip"`   // 默认 10
	DailyLimitPerIp    int `json:"daily_limit_per_ip"`    // 默认 30
	LogRetentionDays   int `json:"log_retention_days"`    // 默认 90

	Scenes []string `json:"scenes"` // 启用的场景码，空 = 全部关闭
}

type SmsAliyunConfig struct {
	Domestic      SmsAliyunRegion `json:"domestic"`
	International SmsAliyunRegion `json:"international"`
}

type SmsAliyunRegion struct {
	AccessKeyId     string            `json:"access_key_id"`
	AccessKeySecret string            `json:"access_key_secret"`
	Endpoint        string            `json:"endpoint"` // 空则用区域默认值
	SignName        string            `json:"sign_name"`
	ParamName       string            `json:"param_name"` // 模板变量名，空则默认 "code"，见 §3.6
	Templates       map[string]string `json:"templates"`  // 场景码 -> TemplateCode
}

type SmsTencentConfig struct {
	Domestic      SmsTencentRegion `json:"domestic"`
	International SmsTencentRegion `json:"international"`
}

type SmsTencentRegion struct {
	SecretId    string            `json:"secret_id"`
	SecretKey   string            `json:"secret_key"`
	SmsSdkAppId string            `json:"sms_sdk_app_id"`
	Region      string            `json:"region"` // 空则 ap-guangzhou
	SignName    string            `json:"sign_name"`
	Templates   map[string]string `json:"templates"` // 场景码 -> TemplateId
}

// Twilio 只有国际 profile（决策 17）。
type SmsTwilioConfig struct {
	International SmsTwilioRegion `json:"international"`
}

type SmsTwilioRegion struct {
	// AccountSid 恒必填（它在 URL 路径里）。
	// Basic 凭据对二选一（决策 18）：(AccountSid, AuthToken) 或 (ApiKeySid, ApiKeySecret)；
	// 都填时优先 API Key，但路径里仍用 AccountSid。
	AccountSid   string `json:"account_sid"`
	AuthToken    string `json:"auth_token"`
	ApiKeySid    string `json:"api_key_sid"`
	ApiKeySecret string `json:"api_key_secret"`

	// 两种发件方二选一（决策 19）：都填时优先 MessagingServiceSid。
	From                string `json:"from"`
	MessagingServiceSid string `json:"messaging_service_sid"`

	Texts map[string]string `json:"texts"` // 场景码 -> 本地文案，占位符 {{code}}
}

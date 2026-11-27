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

	// TrustedProxies 是可信反向代理 / CDN 回源网段（IP 或 CIDR，如 "1.2.3.4"、
	// "2408:4001::/32"）。只有**连接的对端落在这些网段里**时，才采信
	// X-Forwarded-For 里的客户端地址；空（默认）= 不信任任何代理。
	//
	// 为什么必须有这一项：站点挂 CDN 后，TCP 对端永远是 CDN 回源节点，真实客户端
	// 地址只能靠 HTTP 头传递；而不配这一项时我们无法区分「CDN 在转发」与
	// 「客户端自己塞了个头」（两者报文完全一样），只能按「不信任」处理。
	// 详见 provider.SmsClientIP 的注释与 CHANGELOG 的升级须知。
	TrustedProxies []string `json:"trusted_proxies"`
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

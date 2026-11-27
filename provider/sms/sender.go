// Package sms 是短信渠道的纯函数平台层：不碰 DB、不 import provider 包，
// 只允许 provider/sms → config 的单向依赖。
package sms

import "context"

// 场景码。本期只定义验证码场景与后台测试场景；通知场景统一用 "notify." 前缀，
// 本期不定义具体值（规格 §2.4 的预留边界：只留 Sender 接口与场景命名空间，
// 不建投递规则表／模板表／重试队列）。
const (
	SceneLogin    = "login"
	SceneRegister = "register"
	SceneReset    = "reset"
	SceneBindOld  = "bind_old"
	SceneBindNew  = "bind_new"
	SceneTest     = "test" // 后台测试发送专用
)

// Profile 用扁平值类型而不是三个嵌套结构：适配器只读自己关心的字段，
// 测试构造成本低，且不含任何 DB 或站点依赖。
type Profile struct {
	Provider string
	Region   string
	Endpoint string

	// 阿里云 / 腾讯云
	AccessKeyId     string
	AccessKeySecret string
	SecretId        string
	SecretKey       string
	SmsSdkAppId     string
	CloudRegion     string // 腾讯云 X-TC-Region
	SignName        string
	TemplateId      string
	ParamName       string

	// Twilio
	AccountSid          string
	AuthToken           string
	ApiKeySid           string
	ApiKeySecret        string
	From                string
	MessagingServiceSid string
}

type SendRequest struct {
	To    string // E.164，必须带 +，如 +8613900001111
	Scene string
	Code  string // 验证码，唯一模板变量
	Text  string // Twilio 本地文案（已渲染，含验证码）
	OutId string // 关联 SmsLog.Id，透传给平台做对账
}

type SendResult struct {
	BizId   string // 阿里云 BizId / 腾讯云 SerialNo / Twilio sid
	RawCode string // 平台业务码原值
	RawMsg  string
}

// Sender 是平台无关的发送契约。Send 的错误一律包成 *SendError，
// 带 Class（permanent／transient）、Code、Message。业务失败不得返回 nil error——
// HTTP 200 但业务码非成功也是失败。
type Sender interface {
	Name() string
	Send(ctx context.Context, p Profile, req SendRequest) (SendResult, error)
}

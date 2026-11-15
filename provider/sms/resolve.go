package sms

import (
	"fmt"
	"net/url"
	"strings"

	"kandaoni.com/anqicms/config"
)

// 区域默认接入点与默认模板变量名（规格 §3.2／§3.6）。
// 阿里云国际 endpoint 可配：站长可能开在别的地域，写死会让他配不出来。
const (
	aliyunDomesticEndpoint      = "dysmsapi.aliyuncs.com"
	aliyunInternationalEndpoint = "dysmsapi.ap-southeast-1.aliyuncs.com"
	tencentEndpoint             = "sms.tencentcloudapi.com"
	tencentDefaultRegion        = "ap-guangzhou"
	aliyunDefaultParamName      = "code"
	twilioEndpointFormat        = "https://api.twilio.com/2010-04-01/Accounts/%s/Messages.json"
)

// 验证码参数默认值（全局约束逐字取值）。CodeLength 合法区间 4..8。
const (
	defaultCodeLength         = 6
	minCodeLength             = 4
	maxCodeLength             = 8
	defaultCodeTTL            = 300
	defaultResendInterval     = 60
	defaultMaxVerifyAttempts  = 5
	defaultDailyLimitPerPhone = 10
	defaultHourlyLimitPerIp   = 10
	defaultDailyLimitPerIp    = 30
	defaultLogRetentionDays   = 90
)

// ApplyDefaults 就地把未配置的字段填成默认值：8 个验证码参数 + 阿里云两槽的
// endpoint／ParamName + 腾讯云两槽的 Region。非正数按「未配置」处理；
// CodeLength 的正数越界值双向钳到 4..8（传 3 收敛到 4，传 9 收敛到 8），
// 非正数则回默认 6——钳到 4 会凭空把验证码强度降到 4 位。
//
// 只写标量字段，绝不触碰 Templates／Texts 这两个 map，也不改写 Provider／Scenes。
// 调用方若持有站点内存里的全局配置对象（Website.PluginSms），必须先值拷贝再调用，
// 否则一次解析就把默认值写回了站点配置——Resolve 内部就是这么做的。
func ApplyDefaults(cfg *config.PluginSmsConfig) {
	if cfg == nil {
		return
	}
	switch {
	case cfg.CodeLength <= 0:
		cfg.CodeLength = defaultCodeLength
	case cfg.CodeLength < minCodeLength:
		cfg.CodeLength = minCodeLength
	case cfg.CodeLength > maxCodeLength:
		cfg.CodeLength = maxCodeLength
	}
	if cfg.CodeTTL <= 0 {
		cfg.CodeTTL = defaultCodeTTL
	}
	if cfg.ResendInterval <= 0 {
		cfg.ResendInterval = defaultResendInterval
	}
	if cfg.MaxVerifyAttempts <= 0 {
		cfg.MaxVerifyAttempts = defaultMaxVerifyAttempts
	}
	if cfg.DailyLimitPerPhone <= 0 {
		cfg.DailyLimitPerPhone = defaultDailyLimitPerPhone
	}
	if cfg.HourlyLimitPerIp <= 0 {
		cfg.HourlyLimitPerIp = defaultHourlyLimitPerIp
	}
	if cfg.DailyLimitPerIp <= 0 {
		cfg.DailyLimitPerIp = defaultDailyLimitPerIp
	}
	if cfg.LogRetentionDays <= 0 {
		cfg.LogRetentionDays = defaultLogRetentionDays
	}

	if cfg.Aliyun.Domestic.Endpoint == "" {
		cfg.Aliyun.Domestic.Endpoint = aliyunDomesticEndpoint
	}
	if cfg.Aliyun.International.Endpoint == "" {
		cfg.Aliyun.International.Endpoint = aliyunInternationalEndpoint
	}
	// 防御性归一化（裁决 R36）：库里既有值可能是站长带 scheme 填进来的，
	// 而适配器裸拼 "https://"+endpoint+"/?" → 会拼出 https://https//… 的畸形 URL，
	// 直到发请求才炸（DNS 失败 → transient → 站点误以为平台抖动）。
	// 只在 ApplyDefaults 里归一化还不够：ApplyDefaults 跑在副本上，
	// 站点内存里的原值不变；写入口（后台保存）另有一份归一化，见 provider 侧调用。
	cfg.Aliyun.Domestic.Endpoint = NormalizeEndpointHost(cfg.Aliyun.Domestic.Endpoint)
	cfg.Aliyun.International.Endpoint = NormalizeEndpointHost(cfg.Aliyun.International.Endpoint)
	if cfg.Aliyun.Domestic.ParamName == "" {
		cfg.Aliyun.Domestic.ParamName = aliyunDefaultParamName
	}
	if cfg.Aliyun.International.ParamName == "" {
		cfg.Aliyun.International.ParamName = aliyunDefaultParamName
	}
	if cfg.Tencent.Domestic.Region == "" {
		cfg.Tencent.Domestic.Region = tencentDefaultRegion
	}
	if cfg.Tencent.International.Region == "" {
		cfg.Tencent.International.Region = tencentDefaultRegion
	}
}

// NormalizeEndpointHost 归一化阿里云 endpoint：剥 scheme、剥尾斜杠、剥首尾空白。
//
// 裁决 R36：站长最常见的填法是「https://dysmsapi.aliyuncs.com」（带 scheme），
// 而 aliyun.go 裸拼 "https://"+endpoint+"/?" → 拼出 https://https//… 的畸形 URL，
// 要到发请求才暴露（DNS 失败被判成 transient + retryable，站点以为是平台抖动）。
//
// 只剥 scheme 与尾斜杠，**不剥路径**：R36 已核过腾讯云/Twilio 的接入点要么是纯
// host、要么是代码常量里的完整 URL，带路径的 endpoint 归一化不得削掉它。
// 腾讯云与 Twilio 的 endpoint 不是用户可填字段，故只对阿里云两槽调用本函数。
func NormalizeEndpointHost(raw string) string {
	s := strings.TrimSpace(raw)
	if s == "" {
		return ""
	}
	if i := strings.Index(s, "://"); i >= 0 {
		s = s[i+3:]
	}
	s = strings.TrimRight(s, "/")
	return strings.TrimSpace(s)
}

// Resolve 是「配置 → 可用 Profile」的唯一入口（规格 §3.3）。
// 必须带 scene：模板按场景配，不带场景就无法判定「该场景模板为空」并提前拒发。
//
// 顺序：未启用判定 → 值拷贝 + ApplyDefaults（不写回入参）→ 按 provider 选槽
// → 区域支持判定 → 凭据继承（只继承凭据）→ 必填校验 → 填 Profile。
// 任何失败路径都返回 (nil, 哨兵错误)，绝不返回半成品 Profile 或 (nil, nil)。
//
// 场景是否启用（Scenes）不在这里判：那是发码流程的事（规格 §6.3），
// Resolve 只回答「这套配置能不能给该区域该场景发出一条短信」。
func Resolve(cfg *config.PluginSmsConfig, region, scene string) (*Profile, error) {
	if cfg == nil || cfg.Provider == "" {
		return nil, ErrSmsServiceNotEnabled
	}
	// cfg 是站点内存里的全局对象，ApplyDefaults 只能跑在副本上。
	// 这是浅拷贝：Templates／Texts 仍与原件共享，下面只读不写，因此安全。
	snapshot := *cfg
	ApplyDefaults(&snapshot)

	switch snapshot.Provider {
	case config.SmsProviderAliyun:
		return resolveAliyun(&snapshot, region, scene)
	case config.SmsProviderTencent:
		return resolveTencent(&snapshot, region, scene)
	case config.SmsProviderTwilio:
		return resolveTwilio(&snapshot, region, scene)
	default:
		// 未知服务商按未启用处理，fail closed。
		return nil, ErrSmsServiceNotEnabled
	}
}

// resolveAliyun 解析阿里云槽。SignName 两槽都必填（平台请求参数），
// 且永不继承——国内签名过审不代表国际签名过审。
func resolveAliyun(cfg *config.PluginSmsConfig, region, scene string) (*Profile, error) {
	var slot config.SmsAliyunRegion
	switch region {
	case config.SmsRegionDomestic:
		slot = cfg.Aliyun.Domestic
	case config.SmsRegionInternational:
		slot = cfg.Aliyun.International
		// 只继承凭据字段，逐个判空（规格 §3.3 规则 4）。
		// SignName／Templates／Endpoint／ParamName 一律不沿用国内槽。
		if slot.AccessKeyId == "" {
			slot.AccessKeyId = cfg.Aliyun.Domestic.AccessKeyId
		}
		if slot.AccessKeySecret == "" {
			slot.AccessKeySecret = cfg.Aliyun.Domestic.AccessKeySecret
		}
	default:
		return nil, ErrSmsRegionUnsupported
	}

	if slot.AccessKeyId == "" || slot.AccessKeySecret == "" {
		return nil, ErrSmsCredentialMissing
	}
	if slot.SignName == "" {
		return nil, ErrSmsSignMissing
	}
	templateId := slot.Templates[scene]
	if templateId == "" {
		return nil, ErrSmsTemplateMissing
	}
	return &Profile{
		Provider:        cfg.Provider,
		Region:          region,
		Endpoint:        slot.Endpoint,
		AccessKeyId:     slot.AccessKeyId,
		AccessKeySecret: slot.AccessKeySecret,
		SignName:        slot.SignName,
		TemplateId:      templateId,
		ParamName:       slot.ParamName,
	}, nil
}

// resolveTencent 解析腾讯云槽。SignName 国内必填、国际允许为空
// （规格 §5.3：国际短信可用 SenderId 或平台默认签名）。
// SmsSdkAppId 是请求体必填参数，但它不是凭据（规格 §3.4 的不掩码名单），
// 因此不继承，两槽各配各的。
func resolveTencent(cfg *config.PluginSmsConfig, region, scene string) (*Profile, error) {
	var slot config.SmsTencentRegion
	switch region {
	case config.SmsRegionDomestic:
		slot = cfg.Tencent.Domestic
	case config.SmsRegionInternational:
		slot = cfg.Tencent.International
		// 只继承凭据字段（规格 §3.3 规则 4）。
		if slot.SecretId == "" {
			slot.SecretId = cfg.Tencent.Domestic.SecretId
		}
		if slot.SecretKey == "" {
			slot.SecretKey = cfg.Tencent.Domestic.SecretKey
		}
	default:
		return nil, ErrSmsRegionUnsupported
	}

	if slot.SecretId == "" || slot.SecretKey == "" || slot.SmsSdkAppId == "" {
		return nil, ErrSmsCredentialMissing
	}
	if region == config.SmsRegionDomestic && slot.SignName == "" {
		return nil, ErrSmsSignMissing
	}
	templateId := slot.Templates[scene]
	if templateId == "" {
		return nil, ErrSmsTemplateMissing
	}
	return &Profile{
		Provider:    cfg.Provider,
		Region:      region,
		Endpoint:    tencentEndpoint,
		SecretId:    slot.SecretId,
		SecretKey:   slot.SecretKey,
		SmsSdkAppId: slot.SmsSdkAppId,
		CloudRegion: slot.Region,
		SignName:    slot.SignName,
		TemplateId:  templateId,
		// ParamName 留空：腾讯云 TemplateParamSet 是位置数组，没有变量名（规格 §3.6）。
	}, nil
}

// resolveTwilio 解析 Twilio 槽。只有国际 profile（决策 17），
// 国内号码在这里就被明确拒发，不静默降级、不走到适配器。
//
// AccountSid 恒必填（它在 URL 路径里）；Basic 凭据对二选一（决策 18）：
// (AccountSid, AuthToken) 或 (ApiKeySid, ApiKeySecret)，半对不算配齐。
// From 与 MessagingServiceSid 二选一（决策 19），都空则提前拒发（规格 §5.4）。
// 两组值都原样带进 Profile，优先级由适配器决定。
func resolveTwilio(cfg *config.PluginSmsConfig, region, scene string) (*Profile, error) {
	if region != config.SmsRegionInternational {
		return nil, ErrSmsRegionUnsupported
	}
	slot := cfg.Twilio.International
	if slot.AccountSid == "" {
		return nil, ErrSmsCredentialMissing
	}
	hasAuthToken := slot.AuthToken != ""
	hasApiKeyPair := slot.ApiKeySid != "" && slot.ApiKeySecret != ""
	if !hasAuthToken && !hasApiKeyPair {
		return nil, ErrSmsCredentialMissing
	}
	if slot.From == "" && slot.MessagingServiceSid == "" {
		return nil, ErrSmsCredentialMissing
	}
	text := slot.Texts[scene]
	if text == "" {
		return nil, ErrSmsTemplateMissing
	}
	// ⚠️ Twilio 没有模板 ID，短信正文就是站长自己写的这段文案，验证码靠
	// {{code}} 占位符注入。文案漏写占位符时，渲染结果是一段**不含验证码**的
	// 普通短信——用户收到短信却收不到码，而系统照样记「发送成功」。
	// 这必须在发送前拒掉，不能等到用户投诉（裁决 R151）。
	if !strings.Contains(text, "{{code}}") {
		return nil, ErrSmsTemplateInvalid
	}
	return &Profile{
		Provider:            cfg.Provider,
		Region:              region,
		Endpoint:            fmt.Sprintf(twilioEndpointFormat, url.PathEscape(slot.AccountSid)), // 裁决 R41①：转义防路径注入，AccountSid 原值仍留给 Basic 用户名侧
		AccountSid:          slot.AccountSid,
		AuthToken:           slot.AuthToken,
		ApiKeySid:           slot.ApiKeySid,
		ApiKeySecret:        slot.ApiKeySecret,
		From:                slot.From,
		MessagingServiceSid: slot.MessagingServiceSid,
		// Twilio 不预审模板，没有模板 ID：TemplateId 槽承载该场景的本地文案原文
		// （含 {{code}} 占位符，未渲染）。调用方渲染后放进 SendRequest.Text，
		// 适配器取 Body（规格 §3.3「Profile 含该场景模板 ID／文案」+ §5.4）。
		TemplateId: text,
	}, nil
}

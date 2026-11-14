package sms

// 反向对照已实跑（2026-11-14）：把 resolveAliyun／resolveTencent 国际分支的「只继承凭据」
// 改成「整槽拷贝」（slot = cfg.X.Domestic）后，TestResolveCredentialInheritance 10 个子测试里 7 个变红
// （签名／模板／endpoint／SdkAppId 沿用国内、国际自带凭据被覆盖、缺签名缺模板不再拒发），恢复后全绿。

import (
	"encoding/json"
	"errors"
	"reflect"
	"testing"

	"kandaoni.com/anqicms/config"
)

// 本文件全部使用假凭据（fake-*／AC_fake_*），不涉及任何真实 AK/SK。

// cloneConfig 做真深拷贝（含 Templates／Texts 这两个 map）。
// 直接用 `before := *cfg` 会让 map 与原件共享底层数据，
// 「入参被改写」的断言就会因为两边一起变而空转。
func cloneConfig(t *testing.T, cfg *config.PluginSmsConfig) *config.PluginSmsConfig {
	t.Helper()
	raw, err := json.Marshal(cfg)
	if err != nil {
		t.Fatalf("序列化配置失败：%v", err)
	}
	out := &config.PluginSmsConfig{}
	if err := json.Unmarshal(raw, out); err != nil {
		t.Fatalf("反序列化配置失败：%v", err)
	}
	return out
}

// assertNumericDefaults 逐个核对全局约束里的 8 个默认值。
func assertNumericDefaults(t *testing.T, cfg *config.PluginSmsConfig) {
	t.Helper()
	checks := []struct {
		name string
		got  int
		want int
	}{
		{"CodeLength", cfg.CodeLength, 6},
		{"CodeTTL", cfg.CodeTTL, 300},
		{"ResendInterval", cfg.ResendInterval, 60},
		{"MaxVerifyAttempts", cfg.MaxVerifyAttempts, 5},
		{"DailyLimitPerPhone", cfg.DailyLimitPerPhone, 10},
		{"HourlyLimitPerIp", cfg.HourlyLimitPerIp, 10},
		{"DailyLimitPerIp", cfg.DailyLimitPerIp, 30},
		{"LogRetentionDays", cfg.LogRetentionDays, 90},
	}
	for _, c := range checks {
		if c.got != c.want {
			t.Errorf("%s = %d, want %d", c.name, c.got, c.want)
		}
	}
}

// validConfig 造一份「两个区域都配齐」的合法配置：国内与国际凭据取不同值，
// 这样任何一处漏继承／误继承都能在断言里看出来，不会被相同取值掩盖。
func validConfig(provider string) *config.PluginSmsConfig {
	cfg := &config.PluginSmsConfig{Provider: provider}
	switch provider {
	case config.SmsProviderAliyun:
		cfg.Aliyun.Domestic = config.SmsAliyunRegion{
			AccessKeyId:     "fake-dom-id",
			AccessKeySecret: "fake-dom-secret",
			SignName:        "DomSign",
			Templates:       map[string]string{SceneLogin: "SMS_DOM_LOGIN", SceneRegister: "SMS_DOM_REGISTER"},
		}
		cfg.Aliyun.International = config.SmsAliyunRegion{
			AccessKeyId:     "fake-intl-id",
			AccessKeySecret: "fake-intl-secret",
			SignName:        "IntlSign",
			Templates:       map[string]string{SceneLogin: "SMS_INTL_LOGIN"},
		}
	case config.SmsProviderTencent:
		cfg.Tencent.Domestic = config.SmsTencentRegion{
			SecretId:    "fake-dom-id",
			SecretKey:   "fake-dom-key",
			SmsSdkAppId: "1400000001",
			SignName:    "DomSign",
			Templates:   map[string]string{SceneLogin: "1001"},
		}
		// 腾讯云国际签名允许为空（规格 §5.3），这里故意留空。
		cfg.Tencent.International = config.SmsTencentRegion{
			SecretId:    "fake-intl-id",
			SecretKey:   "fake-intl-key",
			SmsSdkAppId: "1400000002",
			Templates:   map[string]string{SceneLogin: "1002"},
		}
	case config.SmsProviderTwilio:
		cfg.Twilio.International = config.SmsTwilioRegion{
			AccountSid: "AC_fake_sid",
			AuthToken:  "fake_auth_token",
			From:       "+15005550006",
			Texts:      map[string]string{SceneLogin: "Your code is {{code}}"},
		}
	}
	return cfg
}

// TestResolveCredentialInheritance 钉住规格 §3.3 规则 4：国际槽只继承凭据字段，
// SignName 与 Templates／Texts 永不沿用；反向（国际→国内）绝不继承。
func TestResolveCredentialInheritance(t *testing.T) {
	t.Run("阿里云_只继承凭据_签名模板取国际槽", func(t *testing.T) {
		cfg := validConfig(config.SmsProviderAliyun)
		cfg.Aliyun.International.AccessKeyId = ""
		cfg.Aliyun.International.AccessKeySecret = ""

		p, err := Resolve(cfg, config.SmsRegionInternational, SceneLogin)
		if err != nil {
			t.Fatalf("国际槽凭据留空应继承国内，实际报错：%v", err)
		}
		if p.AccessKeyId != "fake-dom-id" || p.AccessKeySecret != "fake-dom-secret" {
			t.Errorf("凭据未继承国内值：id=%q secret=%q", p.AccessKeyId, p.AccessKeySecret)
		}
		if p.SignName != "IntlSign" {
			t.Errorf("SignName 必须取国际槽自己的值，绝不沿用国内：got %q", p.SignName)
		}
		if p.TemplateId != "SMS_INTL_LOGIN" {
			t.Errorf("TemplateId 必须取国际槽自己的模板，绝不沿用国内：got %q", p.TemplateId)
		}
		if p.Endpoint != "dysmsapi.ap-southeast-1.aliyuncs.com" {
			t.Errorf("国际 endpoint 不得沿用国内：%q", p.Endpoint)
		}
	})

	t.Run("阿里云_国际槽自带凭据则不继承", func(t *testing.T) {
		cfg := validConfig(config.SmsProviderAliyun)
		p, err := Resolve(cfg, config.SmsRegionInternational, SceneLogin)
		if err != nil {
			t.Fatalf("unexpected err: %v", err)
		}
		if p.AccessKeyId != "fake-intl-id" || p.AccessKeySecret != "fake-intl-secret" {
			t.Errorf("国际槽已填凭据却被国内覆盖：id=%q secret=%q", p.AccessKeyId, p.AccessKeySecret)
		}
	})

	t.Run("阿里云_逐字段判空_只补缺的那一个", func(t *testing.T) {
		cfg := validConfig(config.SmsProviderAliyun)
		cfg.Aliyun.International.AccessKeySecret = ""
		p, err := Resolve(cfg, config.SmsRegionInternational, SceneLogin)
		if err != nil {
			t.Fatalf("unexpected err: %v", err)
		}
		if p.AccessKeyId != "fake-intl-id" {
			t.Errorf("已填的 AccessKeyId 被改写了：%q", p.AccessKeyId)
		}
		if p.AccessKeySecret != "fake-dom-secret" {
			t.Errorf("留空的 AccessKeySecret 未继承国内：%q", p.AccessKeySecret)
		}
	})

	t.Run("阿里云_反向不继承_国内缺凭据直接拒发", func(t *testing.T) {
		cfg := validConfig(config.SmsProviderAliyun)
		cfg.Aliyun.Domestic.AccessKeyId = ""
		cfg.Aliyun.Domestic.AccessKeySecret = ""

		p, err := Resolve(cfg, config.SmsRegionDomestic, SceneLogin)
		if !errors.Is(err, ErrSmsCredentialMissing) {
			t.Fatalf("国内缺凭据必须报 ErrSmsCredentialMissing，不得静默借用国际凭据：err=%v", err)
		}
		if p != nil {
			t.Errorf("拒发时 profile 必须为 nil，实际：%+v", p)
		}
	})

	t.Run("阿里云_国际缺签名不沿用国内签名", func(t *testing.T) {
		cfg := validConfig(config.SmsProviderAliyun)
		cfg.Aliyun.International.SignName = ""
		if _, err := Resolve(cfg, config.SmsRegionInternational, SceneLogin); !errors.Is(err, ErrSmsSignMissing) {
			t.Fatalf("国际槽缺签名必须拒发（签名永不继承）：err=%v", err)
		}
	})

	t.Run("阿里云_国际缺模板不沿用国内模板", func(t *testing.T) {
		cfg := validConfig(config.SmsProviderAliyun)
		cfg.Aliyun.International.Templates = nil
		if _, err := Resolve(cfg, config.SmsRegionInternational, SceneLogin); !errors.Is(err, ErrSmsTemplateMissing) {
			t.Fatalf("国际槽缺模板必须拒发（模板永不继承）：err=%v", err)
		}
	})

	t.Run("腾讯云_只继承凭据_SdkAppId与签名模板不沿用", func(t *testing.T) {
		cfg := validConfig(config.SmsProviderTencent)
		cfg.Tencent.International.SecretId = ""
		cfg.Tencent.International.SecretKey = ""

		p, err := Resolve(cfg, config.SmsRegionInternational, SceneLogin)
		if err != nil {
			t.Fatalf("国际槽凭据留空应继承国内，实际报错：%v", err)
		}
		if p.SecretId != "fake-dom-id" || p.SecretKey != "fake-dom-key" {
			t.Errorf("凭据未继承国内值：id=%q key=%q", p.SecretId, p.SecretKey)
		}
		if p.SmsSdkAppId != "1400000002" {
			t.Errorf("SmsSdkAppId 不是凭据（规格 §3.4 不掩码名单），不得继承国内：got %q", p.SmsSdkAppId)
		}
		if p.SignName != "" {
			t.Errorf("腾讯云国际签名允许为空，但绝不能沿用国内签名：got %q", p.SignName)
		}
		if p.TemplateId != "1002" {
			t.Errorf("TemplateId 必须取国际槽自己的模板：got %q", p.TemplateId)
		}
	})

	t.Run("腾讯云_国际槽自带凭据则不继承", func(t *testing.T) {
		cfg := validConfig(config.SmsProviderTencent)
		p, err := Resolve(cfg, config.SmsRegionInternational, SceneLogin)
		if err != nil {
			t.Fatalf("unexpected err: %v", err)
		}
		if p.SecretId != "fake-intl-id" || p.SecretKey != "fake-intl-key" {
			t.Errorf("国际槽已填凭据却被国内覆盖：id=%q key=%q", p.SecretId, p.SecretKey)
		}
	})

	t.Run("腾讯云_反向不继承_国内缺凭据直接拒发", func(t *testing.T) {
		cfg := validConfig(config.SmsProviderTencent)
		cfg.Tencent.Domestic.SecretId = ""
		cfg.Tencent.Domestic.SecretKey = ""
		p, err := Resolve(cfg, config.SmsRegionDomestic, SceneLogin)
		if !errors.Is(err, ErrSmsCredentialMissing) {
			t.Fatalf("国内缺凭据必须报 ErrSmsCredentialMissing：err=%v", err)
		}
		if p != nil {
			t.Errorf("拒发时 profile 必须为 nil，实际：%+v", p)
		}
	})

	t.Run("腾讯云_国内缺SdkAppId不沿用国际槽", func(t *testing.T) {
		cfg := validConfig(config.SmsProviderTencent)
		cfg.Tencent.Domestic.SmsSdkAppId = ""
		if _, err := Resolve(cfg, config.SmsRegionDomestic, SceneLogin); !errors.Is(err, ErrSmsCredentialMissing) {
			t.Fatalf("SmsSdkAppId 是腾讯云必填参数，缺失必须拒发：err=%v", err)
		}
	})
}

// TestResolveRegionUnsupported 钉住规格 §3.3 规则 3：Twilio + 国内明确拒发，
// 不静默降级到国际槽，也不返回半成品 profile。
func TestResolveRegionUnsupported(t *testing.T) {
	t.Run("Twilio_国内拒发", func(t *testing.T) {
		cfg := validConfig(config.SmsProviderTwilio)
		p, err := Resolve(cfg, config.SmsRegionDomestic, SceneLogin)
		if !errors.Is(err, ErrSmsRegionUnsupported) {
			t.Fatalf("Twilio + domestic 必须报 ErrSmsRegionUnsupported：err=%v", err)
		}
		if p != nil {
			t.Errorf("拒发时 profile 必须为 nil，实际：%+v", p)
		}
	})

	t.Run("未知区域一律拒发", func(t *testing.T) {
		for _, provider := range []string{config.SmsProviderAliyun, config.SmsProviderTencent, config.SmsProviderTwilio} {
			cfg := validConfig(provider)
			p, err := Resolve(cfg, "mars", SceneLogin)
			if !errors.Is(err, ErrSmsRegionUnsupported) {
				t.Errorf("%s + 未知区域必须拒发，不得当成国际槽：err=%v", provider, err)
			}
			if p != nil {
				t.Errorf("%s：拒发时 profile 必须为 nil", provider)
			}
		}
	})
}

// TestResolveMissingPieces 表驱动覆盖规格 §3.3 规则 5 的三类缺件，
// 模板缺失对三家都跑（Twilio 查 Texts）。每条都断言 profile == nil。
func TestResolveMissingPieces(t *testing.T) {
	cases := []struct {
		name     string
		provider string
		region   string
		scene    string
		mutate   func(*config.PluginSmsConfig)
		want     error
	}{
		{
			name: "阿里云_国内缺AccessKeySecret", provider: config.SmsProviderAliyun, region: config.SmsRegionDomestic,
			mutate: func(c *config.PluginSmsConfig) { c.Aliyun.Domestic.AccessKeySecret = "" },
			want:   ErrSmsCredentialMissing,
		},
		{
			name: "阿里云_国内缺AccessKeyId", provider: config.SmsProviderAliyun, region: config.SmsRegionDomestic,
			mutate: func(c *config.PluginSmsConfig) { c.Aliyun.Domestic.AccessKeyId = "" },
			want:   ErrSmsCredentialMissing,
		},
		{
			name: "阿里云_两槽都缺凭据_国际也拒发", provider: config.SmsProviderAliyun, region: config.SmsRegionInternational,
			mutate: func(c *config.PluginSmsConfig) {
				c.Aliyun.Domestic.AccessKeySecret = ""
				c.Aliyun.International.AccessKeySecret = ""
			},
			want: ErrSmsCredentialMissing,
		},
		{
			name: "阿里云_国内缺签名", provider: config.SmsProviderAliyun, region: config.SmsRegionDomestic,
			mutate: func(c *config.PluginSmsConfig) { c.Aliyun.Domestic.SignName = "" },
			want:   ErrSmsSignMissing,
		},
		{
			name: "阿里云_国际缺签名", provider: config.SmsProviderAliyun, region: config.SmsRegionInternational,
			mutate: func(c *config.PluginSmsConfig) { c.Aliyun.International.SignName = "" },
			want:   ErrSmsSignMissing,
		},
		{
			name: "阿里云_国内模板值为空", provider: config.SmsProviderAliyun, region: config.SmsRegionDomestic,
			mutate: func(c *config.PluginSmsConfig) { c.Aliyun.Domestic.Templates[SceneLogin] = "" },
			want:   ErrSmsTemplateMissing,
		},
		{
			name: "阿里云_国内场景不在map里", provider: config.SmsProviderAliyun, region: config.SmsRegionDomestic,
			scene:  SceneReset,
			mutate: func(c *config.PluginSmsConfig) {},
			want:   ErrSmsTemplateMissing,
		},
		{
			name: "阿里云_国内模板map为nil", provider: config.SmsProviderAliyun, region: config.SmsRegionDomestic,
			mutate: func(c *config.PluginSmsConfig) { c.Aliyun.Domestic.Templates = nil },
			want:   ErrSmsTemplateMissing,
		},
		{
			name: "阿里云_国际模板值为空", provider: config.SmsProviderAliyun, region: config.SmsRegionInternational,
			mutate: func(c *config.PluginSmsConfig) { c.Aliyun.International.Templates[SceneLogin] = "" },
			want:   ErrSmsTemplateMissing,
		},
		{
			name: "腾讯云_国内缺SecretKey", provider: config.SmsProviderTencent, region: config.SmsRegionDomestic,
			mutate: func(c *config.PluginSmsConfig) { c.Tencent.Domestic.SecretKey = "" },
			want:   ErrSmsCredentialMissing,
		},
		{
			name: "腾讯云_国内缺SecretId", provider: config.SmsProviderTencent, region: config.SmsRegionDomestic,
			mutate: func(c *config.PluginSmsConfig) { c.Tencent.Domestic.SecretId = "" },
			want:   ErrSmsCredentialMissing,
		},
		{
			name: "腾讯云_国内缺SdkAppId", provider: config.SmsProviderTencent, region: config.SmsRegionDomestic,
			mutate: func(c *config.PluginSmsConfig) { c.Tencent.Domestic.SmsSdkAppId = "" },
			want:   ErrSmsCredentialMissing,
		},
		{
			name: "腾讯云_国内缺签名", provider: config.SmsProviderTencent, region: config.SmsRegionDomestic,
			mutate: func(c *config.PluginSmsConfig) { c.Tencent.Domestic.SignName = "" },
			want:   ErrSmsSignMissing,
		},
		{
			name: "腾讯云_国内模板值为空", provider: config.SmsProviderTencent, region: config.SmsRegionDomestic,
			mutate: func(c *config.PluginSmsConfig) { c.Tencent.Domestic.Templates[SceneLogin] = "" },
			want:   ErrSmsTemplateMissing,
		},
		{
			name: "腾讯云_国内场景不在map里", provider: config.SmsProviderTencent, region: config.SmsRegionDomestic,
			scene:  SceneBindNew,
			mutate: func(c *config.PluginSmsConfig) {},
			want:   ErrSmsTemplateMissing,
		},
		{
			name: "腾讯云_国际模板值为空", provider: config.SmsProviderTencent, region: config.SmsRegionInternational,
			mutate: func(c *config.PluginSmsConfig) { c.Tencent.International.Templates[SceneLogin] = "" },
			want:   ErrSmsTemplateMissing,
		},
		{
			name: "Twilio_缺AccountSid", provider: config.SmsProviderTwilio, region: config.SmsRegionInternational,
			mutate: func(c *config.PluginSmsConfig) { c.Twilio.International.AccountSid = "" },
			want:   ErrSmsCredentialMissing,
		},
		{
			name: "Twilio_两种Basic凭据对都缺", provider: config.SmsProviderTwilio, region: config.SmsRegionInternational,
			mutate: func(c *config.PluginSmsConfig) { c.Twilio.International.AuthToken = "" },
			want:   ErrSmsCredentialMissing,
		},
		{
			name: "Twilio_ApiKey只填一半不算凭据", provider: config.SmsProviderTwilio, region: config.SmsRegionInternational,
			mutate: func(c *config.PluginSmsConfig) {
				c.Twilio.International.AuthToken = ""
				c.Twilio.International.ApiKeySid = "SK_fake_sid"
			},
			want: ErrSmsCredentialMissing,
		},
		{
			name: "Twilio_发件方两者都空", provider: config.SmsProviderTwilio, region: config.SmsRegionInternational,
			mutate: func(c *config.PluginSmsConfig) { c.Twilio.International.From = "" },
			want:   ErrSmsCredentialMissing,
		},
		{
			name: "Twilio_文案值为空", provider: config.SmsProviderTwilio, region: config.SmsRegionInternational,
			mutate: func(c *config.PluginSmsConfig) { c.Twilio.International.Texts[SceneLogin] = "" },
			want:   ErrSmsTemplateMissing,
		},
		{
			name: "Twilio_场景不在Texts里", provider: config.SmsProviderTwilio, region: config.SmsRegionInternational,
			scene:  SceneReset,
			mutate: func(c *config.PluginSmsConfig) {},
			want:   ErrSmsTemplateMissing,
		},
		{
			name: "Twilio_Texts为nil", provider: config.SmsProviderTwilio, region: config.SmsRegionInternational,
			mutate: func(c *config.PluginSmsConfig) { c.Twilio.International.Texts = nil },
			want:   ErrSmsTemplateMissing,
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			cfg := validConfig(tc.provider)
			tc.mutate(cfg)
			scene := tc.scene
			if scene == "" {
				scene = SceneLogin
			}
			p, err := Resolve(cfg, tc.region, scene)
			if !errors.Is(err, tc.want) {
				t.Fatalf("want %v, got %v", tc.want, err)
			}
			if p != nil {
				t.Errorf("缺件拒发时 profile 必须为 nil，实际：%+v", p)
			}
		})
	}
}

// TestResolveServiceNotEnabled 钉住规格 §3.3 规则 1：未启用／未知服务商一律拒发，
// 绝不返回 (nil, nil) 让调用方拿着空 profile 去发送。
func TestResolveServiceNotEnabled(t *testing.T) {
	cases := []struct {
		name string
		cfg  *config.PluginSmsConfig
	}{
		{name: "Provider为空", cfg: &config.PluginSmsConfig{}},
		{name: "cfg为nil", cfg: nil},
		{name: "未知服务商", cfg: &config.PluginSmsConfig{Provider: "smsb"}},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			p, err := Resolve(tc.cfg, config.SmsRegionDomestic, SceneLogin)
			if !errors.Is(err, ErrSmsServiceNotEnabled) {
				t.Fatalf("want ErrSmsServiceNotEnabled, got %v", err)
			}
			if p != nil {
				t.Errorf("拒发时 profile 必须为 nil，实际：%+v", p)
			}
		})
	}
}

// TestResolveProfileSnapshot 钉住 Profile 各字段的落位（规格 §3.3／§5.1）：
// 适配器只吃 Profile，不回头读站点配置，因此这里少一个字段后面就发不出去。
func TestResolveProfileSnapshot(t *testing.T) {
	t.Run("阿里云国内", func(t *testing.T) {
		cfg := validConfig(config.SmsProviderAliyun)
		p, err := Resolve(cfg, config.SmsRegionDomestic, SceneLogin)
		if err != nil {
			t.Fatalf("unexpected err: %v", err)
		}
		want := &Profile{
			Provider:        config.SmsProviderAliyun,
			Region:          config.SmsRegionDomestic,
			Endpoint:        "dysmsapi.aliyuncs.com",
			AccessKeyId:     "fake-dom-id",
			AccessKeySecret: "fake-dom-secret",
			SignName:        "DomSign",
			TemplateId:      "SMS_DOM_LOGIN",
			ParamName:       "code",
		}
		if !reflect.DeepEqual(p, want) {
			t.Errorf("Profile 落位不符\n got: %+v\nwant: %+v", p, want)
		}
	})

	t.Run("阿里云国际endpoint可配", func(t *testing.T) {
		cfg := validConfig(config.SmsProviderAliyun)
		cfg.Aliyun.International.Endpoint = "dysmsapi.eu-central-1.aliyuncs.com"
		p, err := Resolve(cfg, config.SmsRegionInternational, SceneLogin)
		if err != nil {
			t.Fatalf("unexpected err: %v", err)
		}
		if p.Endpoint != "dysmsapi.eu-central-1.aliyuncs.com" {
			t.Errorf("已配置的国际 endpoint 被默认值覆盖：%q", p.Endpoint)
		}
	})

	t.Run("阿里云ParamName可配", func(t *testing.T) {
		cfg := validConfig(config.SmsProviderAliyun)
		cfg.Aliyun.Domestic.ParamName = "verify_code"
		p, err := Resolve(cfg, config.SmsRegionDomestic, SceneLogin)
		if err != nil {
			t.Fatalf("unexpected err: %v", err)
		}
		if p.ParamName != "verify_code" {
			t.Errorf("ParamName 应取配置值：%q", p.ParamName)
		}
	})

	t.Run("腾讯云国内_ParamName留空", func(t *testing.T) {
		cfg := validConfig(config.SmsProviderTencent)
		p, err := Resolve(cfg, config.SmsRegionDomestic, SceneLogin)
		if err != nil {
			t.Fatalf("unexpected err: %v", err)
		}
		want := &Profile{
			Provider:    config.SmsProviderTencent,
			Region:      config.SmsRegionDomestic,
			Endpoint:    "sms.tencentcloudapi.com",
			SecretId:    "fake-dom-id",
			SecretKey:   "fake-dom-key",
			SmsSdkAppId: "1400000001",
			CloudRegion: "ap-guangzhou",
			SignName:    "DomSign",
			TemplateId:  "1001",
			// ParamName 对腾讯云无意义：TemplateParamSet 是位置数组（规格 §3.6）。
		}
		if !reflect.DeepEqual(p, want) {
			t.Errorf("Profile 落位不符\n got: %+v\nwant: %+v", p, want)
		}
	})

	t.Run("腾讯云Region可配", func(t *testing.T) {
		cfg := validConfig(config.SmsProviderTencent)
		cfg.Tencent.Domestic.Region = "ap-beijing"
		p, err := Resolve(cfg, config.SmsRegionDomestic, SceneLogin)
		if err != nil {
			t.Fatalf("unexpected err: %v", err)
		}
		if p.CloudRegion != "ap-beijing" {
			t.Errorf("CloudRegion 应取配置值：%q", p.CloudRegion)
		}
	})

	t.Run("Twilio国际", func(t *testing.T) {
		cfg := validConfig(config.SmsProviderTwilio)
		p, err := Resolve(cfg, config.SmsRegionInternational, SceneLogin)
		if err != nil {
			t.Fatalf("unexpected err: %v", err)
		}
		want := &Profile{
			Provider:   config.SmsProviderTwilio,
			Region:     config.SmsRegionInternational,
			Endpoint:   "https://api.twilio.com/2010-04-01/Accounts/AC_fake_sid/Messages.json",
			AccountSid: "AC_fake_sid",
			AuthToken:  "fake_auth_token",
			From:       "+15005550006",
			// Twilio 不预审模板，TemplateId 槽承载本地文案原文（{{code}} 未渲染）。
			TemplateId: "Your code is {{code}}",
		}
		if !reflect.DeepEqual(p, want) {
			t.Errorf("Profile 落位不符\n got: %+v\nwant: %+v", p, want)
		}
	})

	t.Run("Twilio_APIKey与MessagingServiceSid都填时原样带出", func(t *testing.T) {
		cfg := validConfig(config.SmsProviderTwilio)
		cfg.Twilio.International.ApiKeySid = "SK_fake_sid"
		cfg.Twilio.International.ApiKeySecret = "fake_api_secret"
		cfg.Twilio.International.MessagingServiceSid = "MG_fake_sid"
		p, err := Resolve(cfg, config.SmsRegionInternational, SceneLogin)
		if err != nil {
			t.Fatalf("unexpected err: %v", err)
		}
		// 优先级由适配器决定（规格 §5.4），Resolve 只负责把两组值都快照出来。
		if p.ApiKeySid != "SK_fake_sid" || p.ApiKeySecret != "fake_api_secret" {
			t.Errorf("API Key 凭据未带出：%+v", p)
		}
		if p.MessagingServiceSid != "MG_fake_sid" || p.From != "+15005550006" {
			t.Errorf("两种发件方都应带出，由适配器择一：%+v", p)
		}
		if p.AuthToken != "fake_auth_token" {
			t.Errorf("AuthToken 应原样带出：%q", p.AuthToken)
		}
	})

	t.Run("场景之间模板严格隔离", func(t *testing.T) {
		cfg := validConfig(config.SmsProviderAliyun)
		login, err := Resolve(cfg, config.SmsRegionDomestic, SceneLogin)
		if err != nil {
			t.Fatalf("unexpected err: %v", err)
		}
		register, err := Resolve(cfg, config.SmsRegionDomestic, SceneRegister)
		if err != nil {
			t.Fatalf("unexpected err: %v", err)
		}
		if login.TemplateId != "SMS_DOM_LOGIN" || register.TemplateId != "SMS_DOM_REGISTER" {
			t.Errorf("场景模板串了：login=%q register=%q", login.TemplateId, register.TemplateId)
		}
	})
}

// TestResolveDoesNotMutateConfig 钉住简报步骤 3 的硬约束：cfg 是站点内存里的
// 全局对象，Resolve 只能在副本上跑 ApplyDefaults，绝不写回入参。
func TestResolveDoesNotMutateConfig(t *testing.T) {
	for _, provider := range []string{config.SmsProviderAliyun, config.SmsProviderTencent, config.SmsProviderTwilio} {
		t.Run(provider, func(t *testing.T) {
			cfg := validConfig(provider)
			cfg.CodeLength = 0
			cfg.Aliyun.Domestic.Endpoint = ""
			cfg.Aliyun.International.Endpoint = ""
			cfg.Aliyun.Domestic.ParamName = ""
			cfg.Tencent.Domestic.Region = ""
			before := cloneConfig(t, cfg)
			region := config.SmsRegionDomestic
			if provider == config.SmsProviderTwilio {
				region = config.SmsRegionInternational
			}
			if _, err := Resolve(cfg, region, SceneLogin); err != nil {
				t.Fatalf("unexpected err: %v", err)
			}
			if !reflect.DeepEqual(before, cfg) {
				t.Errorf("Resolve 改写了入参 cfg（站点全局对象）\nbefore: %+v\nafter:  %+v", before, cfg)
			}
			if cfg.CodeLength != 0 {
				t.Errorf("CodeLength 被写回站点配置：%d", cfg.CodeLength)
			}
			if cfg.Aliyun.Domestic.Endpoint != "" || cfg.Aliyun.International.Endpoint != "" {
				t.Errorf("Endpoint 默认值被写回站点配置：%q / %q", cfg.Aliyun.Domestic.Endpoint, cfg.Aliyun.International.Endpoint)
			}
		})
	}
}

// TestApplyDefaults 钉住全局约束的 8 个默认值与 CodeLength 双向钳制，
// 以及 endpoint／ParamName／腾讯云 Region 的区域默认值。
func TestApplyDefaults(t *testing.T) {
	t.Run("零值配置填八个默认值", func(t *testing.T) {
		cfg := &config.PluginSmsConfig{}
		ApplyDefaults(cfg)
		assertNumericDefaults(t, cfg)
	})

	t.Run("CodeLength双向钳制", func(t *testing.T) {
		// 正数越界钳到边界；非正数与其余 7 个参数同口径，按「未配置」回默认 6
		// （钳到 4 会凭空把验证码强度降到 4 位，比回默认更危险）。
		cases := []struct{ in, want int }{
			{0, 6}, {3, 4}, {1, 4}, {-2, 6}, {4, 4}, {5, 5}, {6, 6}, {7, 7}, {8, 8}, {9, 8}, {100, 8},
		}
		for _, c := range cases {
			cfg := &config.PluginSmsConfig{CodeLength: c.in}
			ApplyDefaults(cfg)
			if cfg.CodeLength != c.want {
				t.Errorf("CodeLength %d → %d, want %d", c.in, cfg.CodeLength, c.want)
			}
		}
	})

	t.Run("已显式设置的合法值不被改写", func(t *testing.T) {
		cfg := &config.PluginSmsConfig{
			CodeLength:         8,
			CodeTTL:            600,
			ResendInterval:     120,
			MaxVerifyAttempts:  3,
			DailyLimitPerPhone: 20,
			HourlyLimitPerIp:   5,
			DailyLimitPerIp:    50,
			LogRetentionDays:   30,
		}
		before := *cfg
		ApplyDefaults(cfg)
		// 只比 8 个数值字段：endpoint／ParamName／Region 本来就该被填默认值，
		// 拿整个结构体比会把「填了别的默认值」误判成「改写了合法值」。
		got := config.PluginSmsConfig{
			CodeLength: cfg.CodeLength, CodeTTL: cfg.CodeTTL, ResendInterval: cfg.ResendInterval,
			MaxVerifyAttempts: cfg.MaxVerifyAttempts, DailyLimitPerPhone: cfg.DailyLimitPerPhone,
			HourlyLimitPerIp: cfg.HourlyLimitPerIp, DailyLimitPerIp: cfg.DailyLimitPerIp,
			LogRetentionDays: cfg.LogRetentionDays,
		}
		if !reflect.DeepEqual(before, got) {
			t.Errorf("合法值被改写了\nbefore: %+v\nafter:  %+v", before, got)
		}
	})

	t.Run("非正数按未配置处理", func(t *testing.T) {
		cfg := &config.PluginSmsConfig{
			CodeTTL:            -1,
			ResendInterval:     -60,
			MaxVerifyAttempts:  -5,
			DailyLimitPerPhone: -10,
			HourlyLimitPerIp:   -10,
			DailyLimitPerIp:    -30,
			LogRetentionDays:   -90,
		}
		ApplyDefaults(cfg)
		assertNumericDefaults(t, cfg)
	})

	t.Run("区域默认endpoint与ParamName与腾讯云Region", func(t *testing.T) {
		cfg := &config.PluginSmsConfig{}
		ApplyDefaults(cfg)
		if cfg.Aliyun.Domestic.Endpoint != "dysmsapi.aliyuncs.com" {
			t.Errorf("阿里云国内 endpoint = %q", cfg.Aliyun.Domestic.Endpoint)
		}
		if cfg.Aliyun.International.Endpoint != "dysmsapi.ap-southeast-1.aliyuncs.com" {
			t.Errorf("阿里云国际 endpoint = %q", cfg.Aliyun.International.Endpoint)
		}
		if cfg.Aliyun.Domestic.ParamName != "code" || cfg.Aliyun.International.ParamName != "code" {
			t.Errorf("阿里云 ParamName 默认应为 code：%q / %q", cfg.Aliyun.Domestic.ParamName, cfg.Aliyun.International.ParamName)
		}
		if cfg.Tencent.Domestic.Region != "ap-guangzhou" || cfg.Tencent.International.Region != "ap-guangzhou" {
			t.Errorf("腾讯云 Region 默认应为 ap-guangzhou：%q / %q", cfg.Tencent.Domestic.Region, cfg.Tencent.International.Region)
		}
	})

	t.Run("已配置的区域值不被默认值覆盖", func(t *testing.T) {
		cfg := &config.PluginSmsConfig{
			Aliyun: config.SmsAliyunConfig{
				Domestic:      config.SmsAliyunRegion{Endpoint: "dysmsapi.aliyuncs.com", ParamName: "verify_code"},
				International: config.SmsAliyunRegion{Endpoint: "dysmsapi.ap-southeast-5.aliyuncs.com", ParamName: "pin"},
			},
			Tencent: config.SmsTencentConfig{
				Domestic:      config.SmsTencentRegion{Region: "ap-beijing"},
				International: config.SmsTencentRegion{Region: "ap-singapore"},
			},
		}
		ApplyDefaults(cfg)
		if cfg.Aliyun.Domestic.ParamName != "verify_code" || cfg.Aliyun.International.ParamName != "pin" {
			t.Errorf("ParamName 被覆盖：%q / %q", cfg.Aliyun.Domestic.ParamName, cfg.Aliyun.International.ParamName)
		}
		if cfg.Aliyun.International.Endpoint != "dysmsapi.ap-southeast-5.aliyuncs.com" {
			t.Errorf("国际 endpoint 被覆盖：%q", cfg.Aliyun.International.Endpoint)
		}
		if cfg.Tencent.Domestic.Region != "ap-beijing" || cfg.Tencent.International.Region != "ap-singapore" {
			t.Errorf("腾讯云 Region 被覆盖：%q / %q", cfg.Tencent.Domestic.Region, cfg.Tencent.International.Region)
		}
	})

	t.Run("幂等且不碰模板map", func(t *testing.T) {
		cfg := validConfig(config.SmsProviderAliyun)
		ApplyDefaults(cfg)
		once := cloneConfig(t, cfg)
		ApplyDefaults(cfg)
		if !reflect.DeepEqual(once, cfg) {
			t.Errorf("ApplyDefaults 不幂等\nonce: %+v\ntwice: %+v", once, cfg)
		}
		if len(cfg.Aliyun.Domestic.Templates) != 2 || cfg.Aliyun.Domestic.Templates[SceneLogin] != "SMS_DOM_LOGIN" {
			t.Errorf("ApplyDefaults 不该动模板 map：%v", cfg.Aliyun.Domestic.Templates)
		}
	})

	t.Run("nil配置不panic", func(t *testing.T) {
		ApplyDefaults(nil)
	})
}

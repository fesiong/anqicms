package sms

import (
	"net/url"
	"reflect"
	"testing"

	"kandaoni.com/anqicms/config"
)

// 本文件是裁决 R40／R41 的 Resolve 层护栏，与适配器层的优先级用例互补不互替：
// 适配器测试自己构造 Profile，根本不经过 resolve.go 的分支，抓不住那里的变异。
//
// R40：resolveTwilio 有两条用户明确批准的模式——「只填 ApiKeySid+ApiKeySecret、
// AuthToken 空」与「只填 MessagingServiceSid、From 空」。任务 2 的 23 条用例
// 只有负向对照（把 !hasAuthToken && !hasApiKeyPair 改成 !hasAuthToken、
// 或把 slot.From == "" && slot.MessagingServiceSid == "" 改成 slot.From == ""，
// 一条都不会红）。这里给这两种配置各补一次正向 Resolve。
//
// R41①：Endpoint 里的 AccountSid 必须 url.PathEscape，堵住路径注入。
// 全部用假凭据（AC_fake_*／SK_fake_*／MG_fake_*）。

// TestResolveTwilioOptionalCredentialPairs 两条批准模式各调一次 Resolve，
// 断言返回合法 Profile 且凭据／发件方字段就位（整份 DeepEqual，漏填任何字段都红）。
func TestResolveTwilioOptionalCredentialPairs(t *testing.T) {
	t.Run("只填ApiKey对_AuthToken空", func(t *testing.T) {
		cfg := validConfig(config.SmsProviderTwilio)
		cfg.Twilio.International.AuthToken = ""
		cfg.Twilio.International.ApiKeySid = "SK_fake_sid"
		cfg.Twilio.International.ApiKeySecret = "fake_api_secret"
		// 用例前提自检：AuthToken 真的空了，否则这条正向对照退化成「两对都填」。
		if cfg.Twilio.International.AuthToken != "" {
			t.Fatal("用例前提被破坏：AuthToken 应为空")
		}

		p, err := Resolve(cfg, config.SmsRegionInternational, SceneLogin)
		if err != nil {
			t.Fatalf("只填 ApiKey 对是已批准的配置，必须解析成功: %v", err)
		}
		want := &Profile{
			Provider:     config.SmsProviderTwilio,
			Region:       config.SmsRegionInternational,
			Endpoint:     "https://api.twilio.com/2010-04-01/Accounts/AC_fake_sid/Messages.json",
			AccountSid:   "AC_fake_sid",
			ApiKeySid:    "SK_fake_sid",
			ApiKeySecret: "fake_api_secret",
			From:         "+15005550006",
			TemplateId:   "Your code is {{code}}",
		}
		if !reflect.DeepEqual(p, want) {
			t.Errorf("Profile 落位不符\n got: %+v\nwant: %+v", p, want)
		}
	})

	t.Run("只填MessagingServiceSid_From空", func(t *testing.T) {
		cfg := validConfig(config.SmsProviderTwilio)
		cfg.Twilio.International.From = ""
		cfg.Twilio.International.MessagingServiceSid = "MG_fake_sid"
		// 用例前提自检：From 真的空了，否则这条正向对照退化成「两个都填」。
		if cfg.Twilio.International.From != "" {
			t.Fatal("用例前提被破坏：From 应为空")
		}

		p, err := Resolve(cfg, config.SmsRegionInternational, SceneLogin)
		if err != nil {
			t.Fatalf("只填 MessagingServiceSid 是已批准的配置，必须解析成功: %v", err)
		}
		want := &Profile{
			Provider:            config.SmsProviderTwilio,
			Region:              config.SmsRegionInternational,
			Endpoint:            "https://api.twilio.com/2010-04-01/Accounts/AC_fake_sid/Messages.json",
			AccountSid:          "AC_fake_sid",
			AuthToken:           "fake_auth_token",
			MessagingServiceSid: "MG_fake_sid",
			TemplateId:          "Your code is {{code}}",
		}
		if !reflect.DeepEqual(p, want) {
			t.Errorf("Profile 落位不符\n got: %+v\nwant: %+v", p, want)
		}
	})
}

// TestResolveTwilioEndpointEscapesAccountSid 裁决 R41①：AccountSid 填进
// twilioEndpointFormat 前必须 url.PathEscape。`/`、`?`、`#`、空格不转义会分别
// 改写路径层级、劫持 query、劫持 fragment，把请求打到别的资源上。
// 同时钉住两件事：① unreserved 字符（下划线）是恒等变换，既有护栏不该被打破；
// ② Profile.AccountSid 仍是**原值**——它是 Basic 鉴权的用户名侧，转义了就鉴权失败。
func TestResolveTwilioEndpointEscapesAccountSid(t *testing.T) {
	cases := []struct {
		sid     string
		escaped string
	}{
		{"AC_fake_sid", "AC_fake_sid"},
		{"AC_fake/sid", "AC_fake%2Fsid"},
		{"AC_fake?sid", "AC_fake%3Fsid"},
		{"AC_fake#sid", "AC_fake%23sid"},
		{"AC fake sid", "AC%20fake%20sid"},
	}
	for _, c := range cases {
		t.Run(c.sid, func(t *testing.T) {
			cfg := validConfig(config.SmsProviderTwilio)
			cfg.Twilio.International.AccountSid = c.sid
			p, err := Resolve(cfg, config.SmsRegionInternational, SceneLogin)
			if err != nil {
				t.Fatalf("Resolve 失败: %v", err)
			}
			wantEndpoint := "https://api.twilio.com/2010-04-01/Accounts/" + c.escaped + "/Messages.json"
			if p.Endpoint != wantEndpoint {
				t.Fatalf("Endpoint 未对 AccountSid 做路径转义\n got: %s\nwant: %s", p.Endpoint, wantEndpoint)
			}
			u, err := url.Parse(p.Endpoint)
			if err != nil {
				t.Fatalf("Endpoint 不是可解析的 URL: %v", err)
			}
			if u.Scheme != "https" || u.Host != "api.twilio.com" {
				t.Fatalf("scheme/host 被 AccountSid 改写了: %q %q", u.Scheme, u.Host)
			}
			if u.Path != "/2010-04-01/Accounts/"+c.sid+"/Messages.json" {
				t.Fatalf("解码后的路径不符: %q", u.Path)
			}
			if u.RawQuery != "" || u.Fragment != "" {
				t.Fatalf("AccountSid 里的 ?/# 未被转义，劫持了 query/fragment: query=%q fragment=%q", u.RawQuery, u.Fragment)
			}
			if p.AccountSid != c.sid {
				t.Fatalf("Profile.AccountSid 必须保留原值（Basic 用户名侧）: got %q, want %q", p.AccountSid, c.sid)
			}
		})
	}
}

package sms

import (
	"context"
	"errors"
	"net/http"
	"net/http/httptest"
	"net/url"
	"strings"
	"testing"

	"kandaoni.com/anqicms/config"
)

// 本文件锁死 2026-11-15 整分支审查（审查者 R1）发现的三类真实缺陷：
//
//	① 关键：网络错误的文本里带整条 URL → 验证码随 err_msg 落库；
//	② 重要：阿里云 endpoint 不归一化 → 站长带 scheme 填值就拼出畸形 URL；
//	③ 重要：Twilio 文案漏写 {{code}} → 发出不含验证码的短信却记成功。
//
// 每条都有「反向对照」：不是靠实现自身算期望，而是把**真实会发生的输入**
// 喂进去，看断言是否真的拦得住。

// ---------------------------------------------------------------------------
// ① 网络错误不得携带 URL
// ---------------------------------------------------------------------------

// TestNetworkErrorMessageStripsURL 直接锁 helper 的语义：
// *url.Error 的 Error() 含整条 URL，必须只留内层原因。
func TestNetworkErrorMessageStripsURL(t *testing.T) {
	inner := errors.New("dial tcp: lookup dysmsapi.aliyuncs.com: no such host")
	ue := &url.Error{
		Op:  "Post",
		URL: `https://dysmsapi.aliyuncs.com/?PhoneNumbers=13900001111&TemplateParam={"code":"123456"}`,
		Err: inner,
	}
	got := networkErrorMessage(ue)
	if strings.Contains(got, "123456") {
		t.Fatalf("剥 URL 后仍含验证码: %q", got)
	}
	if strings.Contains(got, "http") {
		t.Fatalf("剥 URL 后仍含 URL: %q", got)
	}
	if !strings.Contains(got, inner.Error()) {
		t.Fatalf("应保留内层原因 %q，实际 %q", inner.Error(), got)
	}
	if !strings.Contains(got, "Post") {
		t.Fatalf("应保留外层 Op（Post），实际 %q", got)
	}

	// 重定向时内层仍是 *url.Error：必须剥到底。
	nested := &url.Error{Op: "Post", URL: "https://a.example/x", Err: ue}
	got = networkErrorMessage(nested)
	if strings.Contains(got, "123456") || strings.Contains(got, "http") {
		t.Fatalf("嵌套 url.Error 未剥干净: %q", got)
	}

	// 非 url.Error（context deadline 等）：原样保留，不许变空。
	if got = networkErrorMessage(context.DeadlineExceeded); got != context.DeadlineExceeded.Error() {
		t.Fatalf("非 url.Error 应原样保留，实际 %q", got)
	}
	if got = networkErrorMessage(nil); got != "" {
		t.Fatalf("nil 应返回空串，实际 %q", got)
	}
}

// TestAliyunSenderNetworkErrorDoesNotLeakCode 端到端：真 sender + 端口已关闭的
// 服务器 → *SendError.Message 里既不能有验证码，也不能有 TemplateParam / URL。
//
// 这条是 R1 上报的关键缺陷的**行为级**防线：修复前 Message 是
// `Post "https://…&TemplateParam={"code":"123456"}": dial tcp …`（179 字节，
// 验证码在偏移 153），而 smsSend.go 会把它写进 sms_logs.err_msg。
func TestAliyunSenderNetworkErrorDoesNotLeakCode(t *testing.T) {
	srv := httptest.NewTLSServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {}))
	endpoint := aliyunTestEndpoint(srv)
	client := srv.Client()
	srv.Close() // 端口不再监听 → 连接必然失败

	sender := &aliyunSender{client: client}
	req := aliyunTestSendRequest()
	_, err := sender.Send(context.Background(), aliyunTestProfile(endpoint), req)
	if err == nil {
		t.Fatal("连接失败必须返回错误")
	}
	var se *SendError
	if !errors.As(err, &se) {
		t.Fatalf("错误不是 *SendError: %v", err)
	}
	if strings.Contains(se.Message, req.Code) {
		t.Fatalf("错误文本泄露验证码 %q: %q", req.Code, se.Message)
	}
	if strings.Contains(se.Message, "TemplateParam") {
		t.Fatalf("错误文本泄露请求参数: %q", se.Message)
	}
	if strings.Contains(se.Message, "http") {
		t.Fatalf("错误文本泄露 URL: %q", se.Message)
	}
}

// TestTwilioSenderNetworkErrorDoesNotLeakAccountSid 同上，但盯 Twilio：
// 它的 URL 路径里带 AccountSid，剥掉 URL 才能不把账号标识写进 err_msg。
func TestTwilioSenderNetworkErrorDoesNotLeakAccountSid(t *testing.T) {
	srv := httptest.NewTLSServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {}))
	// Endpoint 必须是真的 https 绝对 URL（适配器会先校验），路径里带 AccountSid。
	endpoint := srv.URL + "/2010-04-01/Accounts/" + twilioTestAccountSid + "/Messages.json"
	client := srv.Client()
	srv.Close()

	sender := &twilioSender{client: client}
	_, err := sender.Send(context.Background(), twilioTestProfile(endpoint), twilioTestSendRequest())
	if err == nil {
		t.Fatal("连接失败必须返回错误")
	}
	var se *SendError
	if !errors.As(err, &se) {
		t.Fatalf("错误不是 *SendError: %v", err)
	}
	if strings.Contains(se.Message, twilioTestAccountSid) {
		t.Fatalf("错误文本泄露 AccountSid: %q", se.Message)
	}
	if strings.Contains(se.Message, "http") {
		t.Fatalf("错误文本泄露 URL: %q", se.Message)
	}
}

// ---------------------------------------------------------------------------
// ② endpoint 归一化（裁决 R36）
// ---------------------------------------------------------------------------

func TestNormalizeEndpointHost(t *testing.T) {
	cases := []struct {
		name, in, want string
	}{
		{"带 scheme", "https://dysmsapi.aliyuncs.com", "dysmsapi.aliyuncs.com"},
		{"带 http scheme", "http://dysmsapi.aliyuncs.com", "dysmsapi.aliyuncs.com"},
		{"scheme 大写", "HTTPS://dysmsapi.aliyuncs.com", "dysmsapi.aliyuncs.com"},
		{"尾斜杠", "dysmsapi.aliyuncs.com/", "dysmsapi.aliyuncs.com"},
		{"scheme + 尾斜杠", "https://dysmsapi.aliyuncs.com/", "dysmsapi.aliyuncs.com"},
		{"首尾空白", "  dysmsapi.aliyuncs.com  ", "dysmsapi.aliyuncs.com"},
		{"空串", "", ""},
		{"保留路径（R36：只剥 scheme 与尾斜杠）", "dysmsapi.aliyuncs.com/v1", "dysmsapi.aliyuncs.com/v1"},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			if got := NormalizeEndpointHost(c.in); got != c.want {
				t.Fatalf("NormalizeEndpointHost(%q) = %q，期望 %q", c.in, got, c.want)
			}
		})
	}
}

// TestApplyDefaultsNormalizesAliyunEndpoint 库里既有值（站长带 scheme 填的）
// 必须被防御性归一化，否则适配器会拼出 https://https//… 的畸形 URL。
func TestApplyDefaultsNormalizesAliyunEndpoint(t *testing.T) {
	cfg := &config.PluginSmsConfig{}
	cfg.Aliyun.Domestic.Endpoint = "https://dysmsapi.aliyuncs.com/"
	cfg.Aliyun.International.Endpoint = "http://dysmsapi.ap-southeast-1.aliyuncs.com"
	ApplyDefaults(cfg)
	if cfg.Aliyun.Domestic.Endpoint != "dysmsapi.aliyuncs.com" {
		t.Fatalf("国内 endpoint 未归一化: %q", cfg.Aliyun.Domestic.Endpoint)
	}
	if cfg.Aliyun.International.Endpoint != "dysmsapi.ap-southeast-1.aliyuncs.com" {
		t.Fatalf("国际 endpoint 未归一化: %q", cfg.Aliyun.International.Endpoint)
	}
}

// TestResolveAliyunNormalizesEndpoint 走完整 Resolve：带 scheme 的 endpoint
// 解析出来的 Profile.Endpoint 必须是纯 host。
func TestResolveAliyunNormalizesEndpoint(t *testing.T) {
	cfg := &config.PluginSmsConfig{Provider: config.SmsProviderAliyun}
	cfg.Aliyun.Domestic.AccessKeyId = "ak"
	cfg.Aliyun.Domestic.AccessKeySecret = "sk"
	cfg.Aliyun.Domestic.SignName = "签名"
	cfg.Aliyun.Domestic.Templates = map[string]string{SceneLogin: "SMS_1"}
	cfg.Aliyun.Domestic.Endpoint = "https://dysmsapi.aliyuncs.com/"

	p, err := Resolve(cfg, config.SmsRegionDomestic, SceneLogin)
	if err != nil {
		t.Fatalf("Resolve 失败: %v", err)
	}
	if p.Endpoint != "dysmsapi.aliyuncs.com" {
		t.Fatalf("Profile.Endpoint 未归一化: %q", p.Endpoint)
	}
	if strings.Contains(p.Endpoint, "://") {
		t.Fatalf("Profile.Endpoint 仍含 scheme: %q", p.Endpoint)
	}
}

// ---------------------------------------------------------------------------
// ③ Twilio 文案必须含 {{code}}
// ---------------------------------------------------------------------------

// TestResolveTwilioRejectsTemplateWithoutCodePlaceholder 文案漏写占位符时必须
// 在**发送前**拒掉：否则用户收到一条不含验证码的短信，而系统记「发送成功」。
func TestResolveTwilioRejectsTemplateWithoutCodePlaceholder(t *testing.T) {
	newCfg := func(text string) *config.PluginSmsConfig {
		cfg := &config.PluginSmsConfig{Provider: config.SmsProviderTwilio}
		cfg.Twilio.International.AccountSid = "AC_x"
		cfg.Twilio.International.AuthToken = "tok"
		cfg.Twilio.International.From = "+15005550006"
		cfg.Twilio.International.Texts = map[string]string{SceneLogin: text}
		return cfg
	}

	t.Run("含占位符_放行", func(t *testing.T) {
		p, err := Resolve(newCfg("Your code is {{code}}"), config.SmsRegionInternational, SceneLogin)
		if err != nil {
			t.Fatalf("含占位符的文案应放行: %v", err)
		}
		if p.TemplateId != "Your code is {{code}}" {
			t.Fatalf("TemplateId 槽应承载未渲染原文，实际 %q", p.TemplateId)
		}
	})

	t.Run("漏占位符_拒发", func(t *testing.T) {
		_, err := Resolve(newCfg("Your verification code is 123456"), config.SmsRegionInternational, SceneLogin)
		if err == nil {
			t.Fatal("漏写 {{code}} 的文案必须被拒")
		}
		if !errors.Is(err, ErrSmsTemplateInvalid) {
			t.Fatalf("应报 ErrSmsTemplateInvalid，实际 %v", err)
		}
	})

	t.Run("空文案_仍报模板缺失", func(t *testing.T) {
		_, err := Resolve(newCfg(""), config.SmsRegionInternational, SceneLogin)
		if !errors.Is(err, ErrSmsTemplateMissing) {
			t.Fatalf("空文案应报 ErrSmsTemplateMissing，实际 %v", err)
		}
	})
}

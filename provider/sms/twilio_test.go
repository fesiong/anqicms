package sms

import (
	"bytes"
	"context"
	"encoding/base64"
	"errors"
	"fmt"
	"go/ast"
	"go/parser"
	"go/token"
	"io"
	"net/http"
	"net/http/httptest"
	"net/url"
	"runtime"
	"strconv"
	"strings"
	"sync/atomic"
	"testing"
	"time"
	"unicode/utf8"

	"kandaoni.com/anqicms/config"
)

// 本文件锁死 Twilio 适配器的行为契约（规格 §5.4／§5.6／§10.1／§3.2）：
// POST 完整 URL（裁决 R41：Profile.Endpoint 已是完整 URL，不再拼 https://）、
// HTTP Basic 且 API Key 优先、路径里的 AccountSid 恒为 AccountSid、
// 表单编码且 MessagingServiceSid 优先于 From、Body 取已渲染的 req.Text、
// 平台受理（HTTP 200／201，Twilio 的 POST 创建返回 201）且 status 不为
// failed/undelivered、error_code 为空才算成功、
// HTTP 200 非 JSON 绝不伪装成功、permanent 错误码表逐条覆盖、
// 10s 超时不重试、响应体读取有上限、本期不带 ContentSid／StatusCallback。
// Twilio 无签名算法，不需要 SDK 对照向量；全部用假凭据 + 本地 httptest，永不访问外网。

const twilioOKBody = `{"sid":"SM1","status":"queued","error_code":null,"error_message":null}`

// twilioTestAccountSid 与任务 2 的 resolve_test.go 取值一致（下划线是 unreserved
// 字符，url.PathEscape 对它是恒等变换）。全部是假凭据。
const twilioTestAccountSid = "AC_fake_sid"

type twilioCapturedRequest struct {
	method     string
	host       string
	path       string
	requestURI string
	header     http.Header
	body       string
	form       url.Values
	hits       atomic.Int32
}

func newTwilioFakeServer(t *testing.T, status int, respBody string) (*httptest.Server, *twilioCapturedRequest) {
	t.Helper()
	got := &twilioCapturedRequest{}
	srv := httptest.NewTLSServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		got.hits.Add(1)
		b, _ := io.ReadAll(r.Body)
		got.method = r.Method
		got.host = r.Host
		got.path = r.URL.Path
		got.requestURI = r.RequestURI
		got.header = r.Header.Clone()
		got.body = string(b)
		// 直接解析 body 而不是 r.PostForm：断言的是线上真实字节，
		// 且不与 query 参数混在一起。
		got.form, _ = url.ParseQuery(string(b))
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(status)
		fmt.Fprint(w, respBody)
	}))
	t.Cleanup(srv.Close)
	return srv, got
}

// twilioTestEndpoint 造完整 URL（不是 host）——Twilio 是三家里唯一的例外，
// Resolve 已把 AccountSid 填进 twilioEndpointFormat。
func twilioTestEndpoint(srv *httptest.Server) string {
	return srv.URL + "/2010-04-01/Accounts/" + twilioTestAccountSid + "/Messages.json"
}

func twilioTestProfile(endpoint string) Profile {
	return Profile{
		Provider:   config.SmsProviderTwilio,
		Region:     config.SmsRegionInternational,
		Endpoint:   endpoint,
		AccountSid: twilioTestAccountSid,
		AuthToken:  "fake_auth_token",
		From:       "+15005550006",
		// TemplateId 槽承载未渲染的本地文案原文（规格 §3.3）。
		// 与 req.Text 故意不同：适配器取错槽会立刻被 Body 断言抓住。
		TemplateId: "Your code is {{code}}",
	}
}

func twilioTestSendRequest() SendRequest {
	return SendRequest{
		To:    "+15551234567",
		Scene: SceneLogin,
		Code:  "123456",
		Text:  "Your code is 123456",
		OutId: "9001",
	}
}

// twilioStatusBody 2xx 的 Message 资源形状（error_code 为 null）。
func twilioStatusBody(status string) string {
	return fmt.Sprintf(`{"sid":"SM1","status":%q,"error_code":null,"error_message":null}`, status)
}

// twilioErrorCodeBody 2xx 但带 error_code。error_code 是 integer|null
// （核验自官方 Message 资源文档，2026-11-14）。
func twilioErrorCodeBody(status string, code int64, message string) string {
	return fmt.Sprintf(`{"sid":"SM1","status":%q,"error_code":%d,"error_message":%q}`, status, code, message)
}

// twilioExceptionBody 非 2xx 的 REST 异常体：code/message/more_info/status，
// 其中 status 是**整数** HTTP 码（核验自 twilio.com/docs/usage/twilios-response
// 与 Role Assignment API reference 的 JSON 示例）。它与 Message 资源的
// status（字符串枚举）同名不同型，用一个结构体同时解两种形状会整体解析失败。
func twilioExceptionBody(code int64, message string, httpStatus int) string {
	return fmt.Sprintf(`{"code":%d,"message":%q,"more_info":"https://www.twilio.com/docs/errors/%d","status":%d}`,
		code, message, code, httpStatus)
}

// twilioNoDialTransport 记录并拒绝任何出网尝试。用于「提前拒」用例：
// 畸形 endpoint 若被放行，去掉校验后会真的去 dial api.twilio.com。
type twilioNoDialTransport struct{ calls atomic.Int32 }

func (tr *twilioNoDialTransport) RoundTrip(*http.Request) (*http.Response, error) {
	tr.calls.Add(1)
	return nil, errors.New("不应发起网络请求")
}

// twilioBasicHeader 期望的 Authorization 字面值。
func twilioBasicHeader(user, pass string) string {
	return "Basic " + base64.StdEncoding.EncodeToString([]byte(user+":"+pass))
}

// TestTwilioSenderAuth 钉住 HTTP Basic 的凭据对选择（规格 §5.4）：
// 只填 AuthToken → base64(AccountSid:AuthToken)；ApiKey 对齐全 → 用 API Key
// （优先于 AuthToken）；半对不算配齐，回落 AuthToken。
// 四条都断言 URL 路径里的 AccountSid 仍是 AccountSid——它恒必填，不随凭据对切换。
// 抓它的破坏：把用户名写成 ApiKeySid 时无条件切换（|| 而不是 &&）、
// 路径里改用 ApiKeySid、Authorization 用 Bearer、漏 Basic 前缀。
func TestTwilioSenderAuth(t *testing.T) {
	cases := []struct {
		name     string
		mutate   func(*Profile)
		wantUser string
		wantPass string
	}{
		{"只填AuthToken", func(*Profile) {}, twilioTestAccountSid, "fake_auth_token"},
		{"只填ApiKey对", func(p *Profile) {
			p.AuthToken = ""
			p.ApiKeySid = "SK_fake_sid"
			p.ApiKeySecret = "fake_api_secret"
		}, "SK_fake_sid", "fake_api_secret"},
		{"两者都填_ApiKey优先", func(p *Profile) {
			p.ApiKeySid = "SK_fake_sid"
			p.ApiKeySecret = "fake_api_secret"
		}, "SK_fake_sid", "fake_api_secret"},
		{"ApiKey只填一半_回落AuthToken", func(p *Profile) {
			p.ApiKeySid = "SK_fake_sid"
		}, twilioTestAccountSid, "fake_auth_token"},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			srv, got := newTwilioFakeServer(t, http.StatusOK, twilioOKBody)
			p := twilioTestProfile(twilioTestEndpoint(srv))
			c.mutate(&p)
			sender := &twilioSender{client: srv.Client()}
			if _, err := sender.Send(context.Background(), p, twilioTestSendRequest()); err != nil {
				t.Fatalf("Send 返回错误: %v", err)
			}
			want := twilioBasicHeader(c.wantUser, c.wantPass)
			if auth := got.header.Get("Authorization"); auth != want {
				t.Fatalf("Authorization = %q, want %q", auth, want)
			}
			if got.method != http.MethodPost {
				t.Fatalf("方法 = %s, want POST", got.method)
			}
			wantPath := "/2010-04-01/Accounts/" + twilioTestAccountSid + "/Messages.json"
			if got.path != wantPath {
				t.Fatalf("路径 = %q, want %q（AccountSid 不随凭据对切换）", got.path, wantPath)
			}
			if got.host != strings.TrimPrefix(srv.URL, "https://") {
				t.Fatalf("请求未打到 Endpoint 指定的 host: got %q, want %q", got.host, strings.TrimPrefix(srv.URL, "https://"))
			}
			if got.requestURI != wantPath {
				t.Fatalf("请求行 = %q, want %q（Endpoint 不得被再拼一层前缀或 query）", got.requestURI, wantPath)
			}
		})
	}
}

// TestTwilioSenderFrom 钉住发件方二选一（规格 §5.4）：只填 From → 带 From；
// 两者都填 → **只带 MessagingServiceSid**；两者都空 → 提前拒（ErrSmsCredentialMissing）
// 且一个字节都不出网。同时钉住表单编码、To 带 +、Body 取已渲染的 req.Text。
func TestTwilioSenderFrom(t *testing.T) {
	cases := []struct {
		name    string
		mutate  func(*Profile)
		wantKey string
		wantVal string
		wantErr bool
	}{
		{"只填From", func(*Profile) {}, "From", "+15005550006", false},
		{"只填MessagingServiceSid", func(p *Profile) {
			p.From = ""
			p.MessagingServiceSid = "MG_fake_sid"
		}, "MessagingServiceSid", "MG_fake_sid", false},
		{"两者都填_只带MessagingServiceSid", func(p *Profile) {
			p.MessagingServiceSid = "MG_fake_sid"
		}, "MessagingServiceSid", "MG_fake_sid", false},
		{"两者都空_提前拒", func(p *Profile) { p.From = "" }, "", "", true},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			srv, got := newTwilioFakeServer(t, http.StatusOK, twilioOKBody)
			p := twilioTestProfile(twilioTestEndpoint(srv))
			c.mutate(&p)
			sender := &twilioSender{client: srv.Client()}
			req := twilioTestSendRequest()
			res, err := sender.Send(context.Background(), p, req)

			if c.wantErr {
				if err == nil {
					t.Fatal("From 与 MessagingServiceSid 都空必须拒绝发送")
				}
				if !errors.Is(err, ErrSmsCredentialMissing) {
					t.Fatalf("应能 errors.Is 到 ErrSmsCredentialMissing: %v", err)
				}
				var se *SendError
				if !errors.As(err, &se) {
					t.Fatalf("错误不是 *SendError: %v", err)
				}
				if se.Class != ClassPermanent {
					t.Fatalf("配置缺失应为 permanent, got %s", se.Class)
				}
				if n := got.hits.Load(); n != 0 {
					t.Fatalf("提前拒不得出网: 服务端收到 %d 次请求", n)
				}
				if res.BizId != "" {
					t.Fatalf("失败时不应有回执: %+v", res)
				}
				return
			}

			if err != nil {
				t.Fatalf("Send 返回错误: %v", err)
			}
			if ct := got.header.Get("Content-Type"); ct != "application/x-www-form-urlencoded" {
				t.Fatalf("Content-Type = %q, want application/x-www-form-urlencoded（不得带 charset）", ct)
			}
			if v := got.form.Get("To"); v != "+15551234567" {
				t.Fatalf("To = %q, want 带 + 的 E.164 +15551234567", v)
			}
			if v := got.form.Get("Body"); v != req.Text {
				t.Fatalf("Body = %q, want 已渲染文案 %q（不是 TemplateId 槽的 {{code}} 原文）", v, req.Text)
			}
			if v := got.form.Get(c.wantKey); v != c.wantVal {
				t.Fatalf("%s = %q, want %q", c.wantKey, v, c.wantVal)
			}
			// 二选一：另一个键整个不出现（不是出现但为空）。
			other := "From"
			if c.wantKey == "From" {
				other = "MessagingServiceSid"
			}
			if _, present := got.form[other]; present {
				t.Fatalf("二选一时不得出现 %s 键: %s", other, got.body)
			}
			// 规格 §3.2：本期不做 Content SID 映射，也不用 StatusCallback。
			// 表单键集合穷举，多发任何参数都会红。
			wantKeys := map[string]bool{"To": true, "Body": true, c.wantKey: true}
			if len(got.form) != len(wantKeys) {
				t.Fatalf("表单键数量 = %d, want %d: %s", len(got.form), len(wantKeys), got.body)
			}
			for k := range got.form {
				if !wantKeys[k] {
					t.Fatalf("表单出现本期不该有的键 %q（ContentSid／StatusCallback 是 YAGNI）: %s", k, got.body)
				}
			}
		})
	}
}

// TestTwilioSenderSuccess 成功判据：平台受理（HTTP 200／201）、status 不为
// failed/undelivered 且 error_code 为空。BizId 取 sid，RawCode 取 status 原值。
//
// ⚠️ Twilio 的 POST 创建返回 **201 Created**，不是 200（官方「Twilio API responses」
// 状态码表：「201 Created (For POST requests only) Twilio successfully created a
// new resource」；Postman  collections 文档的发短信一节同样写 201）。
// 照抄阿里云／腾讯云的「只认 200」会把每一次真实成功发送判成 HTTP_201 失败。
// 204 之类没有 Message 资源体的 2xx 不算受理，必须落失败。
func TestTwilioSenderSuccess(t *testing.T) {
	for _, status := range []int{http.StatusOK, http.StatusCreated} {
		t.Run(strconv.Itoa(status), func(t *testing.T) {
			srv, got := newTwilioFakeServer(t, status, twilioOKBody)
			sender := &twilioSender{client: srv.Client()}
			res, err := sender.Send(context.Background(), twilioTestProfile(twilioTestEndpoint(srv)), twilioTestSendRequest())
			if err != nil {
				t.Fatalf("HTTP %d 是平台受理，Send 不应返回错误: %v", status, err)
			}
			if res.BizId != "SM1" {
				t.Fatalf("BizId = %q, want SM1（取 sid）", res.BizId)
			}
			if res.RawCode != "queued" {
				t.Fatalf("RawCode = %q, want queued（status 原值）", res.RawCode)
			}
			if res.RawMsg != "" {
				t.Fatalf("error_message 为 null 时 RawMsg 应为空: %q", res.RawMsg)
			}
			if got.form.Get("Body") != "Your code is 123456" {
				t.Fatalf("Body = %q, want 已渲染文案", got.form.Get("Body"))
			}
		})
	}

	t.Run("204无资源体不算成功", func(t *testing.T) {
		srv, _ := newTwilioFakeServer(t, http.StatusNoContent, "")
		sender := &twilioSender{client: srv.Client()}
		res, err := sender.Send(context.Background(), twilioTestProfile(twilioTestEndpoint(srv)), twilioTestSendRequest())
		if err == nil {
			t.Fatal("204 没有 Message 资源体，不得伪装成功")
		}
		var se *SendError
		if !errors.As(err, &se) {
			t.Fatalf("错误不是 *SendError: %v", err)
		}
		if se.Class != ClassTransient {
			t.Fatalf("应为 transient, got %s", se.Class)
		}
		// 204 走的是「平台未受理」的异常路径，不是「受理了但 body 解不开」：
		// 把判据放宽成「所有 2xx 都算受理」会让这里变成 NonJSONResponse。
		if se.Code != "HTTP_204" {
			t.Fatalf("204 应按未受理回落 HTTP_204, got %q", se.Code)
		}
		if res.BizId != "" {
			t.Fatalf("失败时不应有回执: %+v", res)
		}
	})
}

// TestTwilioSenderFailureStatus HTTP 200 不等于成功：status 为 failed/undelivered
// 必须判失败；error_code 非空时即使 status == "queued"／"sent" 也算失败。
// 没有 error_code 的 failed/undelivered 无码可查 → transient，Code 保留 status 原值。
func TestTwilioSenderFailureStatus(t *testing.T) {
	cases := []struct {
		name     string
		body     string
		wantCode string
		want     ErrorClass
	}{
		{"failed", twilioStatusBody("failed"), "failed", ClassTransient},
		{"undelivered", twilioStatusBody("undelivered"), "undelivered", ClassTransient},
		{"queued但带error_code", twilioErrorCodeBody("queued", 21211, "Invalid 'To' Phone Number"), "21211", ClassPermanent},
		{"sent但带error_code", twilioErrorCodeBody("sent", 30002, "Account suspended"), "30002", ClassPermanent},
		{"delivered但带error_code", twilioErrorCodeBody("delivered", 21610, "unsubscribed"), "21610", ClassPermanent},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			srv, _ := newTwilioFakeServer(t, http.StatusOK, c.body)
			sender := &twilioSender{client: srv.Client()}
			res, err := sender.Send(context.Background(), twilioTestProfile(twilioTestEndpoint(srv)), twilioTestSendRequest())
			if err == nil {
				t.Fatal("失败状态／非空 error_code 绝不能伪装成功")
			}
			var se *SendError
			if !errors.As(err, &se) {
				t.Fatalf("错误不是 *SendError: %v", err)
			}
			if se.Code != c.wantCode {
				t.Fatalf("Code = %q, want %q（平台原值不得被吞）", se.Code, c.wantCode)
			}
			if se.Class != c.want {
				t.Fatalf("Class = %s, want %s", se.Class, c.want)
			}
			if !errors.Is(err, ErrSmsSendFailed) {
				t.Fatal("errors.Is(err, ErrSmsSendFailed) 应为 true")
			}
			if res.BizId != "" {
				t.Fatalf("失败时不应有回执: %+v", res)
			}
		})
	}
}

// TestTwilioSenderNonJSONBody（审查重点 5）：网关插 HTML 错误页／CDN 拦截时
// HTTP 200 但 body 不是 JSON → 必须判失败，原始 body 片段（≤200 字节、合法 UTF-8）
// 留进 Message，分类 transient。Message 的唯一目的地是 sms_logs.err_msg
// （规格 §10.4），按字节硬切会留下半个汉字，utf8mb4 列拒收时整条流水写入失败。
func TestTwilioSenderNonJSONBody(t *testing.T) {
	cases := []struct {
		name string
		body string
	}{
		{"HTML错误页", "<html>502 Bad Gateway</html>"},
		{"超长body截断200字节", strings.Repeat("x", 500)},
		{"多字节中文拦截页", strings.Repeat("网关拦截", 100)},
		{"JSON但是空对象", "{}"},
		{"JSON但缺status", `{"sid":"SM1"}`},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			srv, _ := newTwilioFakeServer(t, http.StatusOK, c.body)
			sender := &twilioSender{client: srv.Client()}
			res, err := sender.Send(context.Background(), twilioTestProfile(twilioTestEndpoint(srv)), twilioTestSendRequest())
			if err == nil {
				t.Fatal("非 JSON／缺 status 的 200 响应绝不能伪装成功")
			}
			var se *SendError
			if !errors.As(err, &se) {
				t.Fatalf("错误不是 *SendError: %v", err)
			}
			if se.Class != ClassTransient {
				t.Fatalf("应为 transient, got %s", se.Class)
			}
			if !utf8.ValidString(se.Message) || !utf8.ValidString(se.Error()) {
				t.Fatalf("Message/Error() 含非法 UTF-8 字节: %q", se.Message)
			}
			if res.BizId != "" {
				t.Fatalf("失败时不应有回执: %+v", res)
			}
			switch c.name {
			case "HTML错误页":
				if !strings.Contains(se.Message, "<html>502 Bad Gateway</html>") {
					t.Fatalf("Message 未保留原始 body 片段: %q", se.Message)
				}
			case "超长body截断200字节":
				if !strings.Contains(se.Message, strings.Repeat("x", 200)) {
					t.Fatalf("Message 未保留前 200 字节: %q", se.Message)
				}
				if strings.Contains(se.Message, strings.Repeat("x", 201)) {
					t.Fatalf("Message 未截断到 200 字节: %q", se.Message)
				}
			case "多字节中文拦截页":
				// 1200 字节切到 200 → 落在第 67 个汉字中间，只丢半个 rune，
				// 应是 16 组「网关拦截」+「网关」（198 字节）。
				if !strings.HasSuffix(se.Message, "网关") {
					t.Fatalf("Message 未回退到完整 rune 边界（应以「网关」结尾）: %q", se.Message)
				}
				if strings.Contains(se.Message, strings.Repeat("网关拦截", 17)) {
					t.Fatalf("Message 未截断到 200 字节: %q", se.Message)
				}
			}
		})
	}
}

// twilioPermanentCases 是规格 §5.6 Twilio 行每一类 permanent 错误的逐条用例。
// 码值核验自官方 Error and Warning Dictionary（2026-11-14）：
//   - 20003 Permission Denied（鉴权失败）https://www.twilio.com/docs/api/errors/20003
//   - 21211 Invalid 'To' Phone Number        https://www.twilio.com/docs/api/errors/21211
//   - 21606 'From' number is not a valid message-capable Twilio number
//     https://www.twilio.com/docs/api/errors/21606
//   - 21610 Attempt to send to unsubscribed recipient
//     https://www.twilio.com/docs/api/errors/21610
//   - 20005 Account not active：「project was suspended after its balance fell to
//     $0 or below」https://www.twilio.com/docs/api/errors/20005
//   - 30002 Account suspended：「account balance became negative, you had overdue
//     payments」https://www.twilio.com/docs/api/errors/30002
//
// 规格只写了「余额不足」没给码；官方字典里没有任何一条标题叫 insufficient funds，
// 余额耗尽被停机的两条官方码就是 20005（请求即拒）与 30002（已入队、投递前被停）。
// 与生产表 twilioPermanentCodes 的双向穷举由
// TestTwilioPermanentCodeTableExhaustive 用 go/ast 锁定，不写死总数。
var twilioPermanentCases = []struct {
	code     string
	category string
}{
	{"20003", "鉴权失败"},
	{"21211", "To非法"},
	{"21606", "From非法"},
	{"21610", "收件人已退订"},
	{"20005", "余额不足"},
	{"30002", "余额不足"},
}

// TestTwilioSenderErrorCodeClassification 表驱动覆盖每一类 permanent 错误。
// 只走 HTTP 200 + Message 资源 error_code 这一种形状：非 2xx 的 permanent
// 由 ClassifyHTTPStatus 保证，把异常体形状也塞进来会变成等价变异
// （删掉表里的码照样 permanent，对照空转）。
// status 取 failed 与 queued 两种：分类必须与 status 无关，
// 「只在 status==failed 时才查表」的实现会被 queued 那组抓住。
func TestTwilioSenderErrorCodeClassification(t *testing.T) {
	for _, c := range twilioPermanentCases {
		for _, status := range []string{"failed", "queued"} {
			t.Run(c.code+"/"+status, func(t *testing.T) {
				code, err := strconv.ParseInt(c.code, 10, 64)
				if err != nil {
					t.Fatalf("用例码不是整数: %q", c.code)
				}
				srv, _ := newTwilioFakeServer(t, http.StatusOK, twilioErrorCodeBody(status, code, "raw message"))
				sender := &twilioSender{client: srv.Client()}
				_, err = sender.Send(context.Background(), twilioTestProfile(twilioTestEndpoint(srv)), twilioTestSendRequest())
				var se *SendError
				if !errors.As(err, &se) {
					t.Fatalf("错误不是 *SendError: %v", err)
				}
				if se.Class != ClassPermanent {
					t.Fatalf("%s（%s）应为 permanent, got %s", c.code, c.category, se.Class)
				}
				if se.Code != c.code {
					t.Fatalf("原始码被改写: %q", se.Code)
				}
				if se.Message != "raw message" {
					t.Fatalf("平台原始消息被改写: %q", se.Message)
				}
			})
		}
	}

	t.Run("未列入表的码原样保留且transient", func(t *testing.T) {
		srv, _ := newTwilioFakeServer(t, http.StatusOK, twilioErrorCodeBody("queued", 99999, "some future code"))
		sender := &twilioSender{client: srv.Client()}
		_, err := sender.Send(context.Background(), twilioTestProfile(twilioTestEndpoint(srv)), twilioTestSendRequest())
		var se *SendError
		if !errors.As(err, &se) {
			t.Fatalf("错误不是 *SendError: %v", err)
		}
		if se.Class != ClassTransient {
			t.Fatalf("未列入表的码应为 transient, got %s", se.Class)
		}
		if se.Code != "99999" {
			t.Fatalf("未识别的码必须原样保留, got %q", se.Code)
		}
		if se.Message != "some future code" {
			t.Fatalf("平台原始消息被改写: %q", se.Message)
		}
	})

	t.Run("平台消息为空时回落中文提示", func(t *testing.T) {
		srv, _ := newTwilioFakeServer(t, http.StatusOK, twilioErrorCodeBody("failed", 20005, ""))
		sender := &twilioSender{client: srv.Client()}
		_, err := sender.Send(context.Background(), twilioTestProfile(twilioTestEndpoint(srv)), twilioTestSendRequest())
		var se *SendError
		if !errors.As(err, &se) {
			t.Fatalf("错误不是 *SendError: %v", err)
		}
		if se.Message == "" {
			t.Fatal("平台消息为空时应回落已知错误码的中文提示")
		}
	})

	// 真实的创建响应是 201，不是 200：错误码表在 201 上也必须生效。
	t.Run("201创建响应带error_code也查表", func(t *testing.T) {
		srv, _ := newTwilioFakeServer(t, http.StatusCreated, twilioErrorCodeBody("undelivered", 30002, "Account suspended"))
		sender := &twilioSender{client: srv.Client()}
		_, err := sender.Send(context.Background(), twilioTestProfile(twilioTestEndpoint(srv)), twilioTestSendRequest())
		var se *SendError
		if !errors.As(err, &se) {
			t.Fatalf("错误不是 *SendError: %v", err)
		}
		if se.Class != ClassPermanent || se.Code != "30002" {
			t.Fatalf("201 上的 error_code 分类失效: class=%s code=%q", se.Class, se.Code)
		}
	})
}

// TestTwilioPermanentCodeTableExhaustive 用 go/ast 穷举生产表 twilioPermanentCodes
// 的键，与测试用例表双向比对：新增错误码漏写测试会红，测试写了表里没有的码也会红。
// 不写死错误码总数（AGENTS.md §6）。
func TestTwilioPermanentCodeTableExhaustive(t *testing.T) {
	fset := token.NewFileSet()
	file, err := parser.ParseFile(fset, "twilio.go", nil, 0)
	if err != nil {
		t.Fatalf("解析 twilio.go 失败: %v", err)
	}
	prodCodes := make(map[string]bool)
	ast.Inspect(file, func(n ast.Node) bool {
		spec, ok := n.(*ast.ValueSpec)
		if !ok {
			return true
		}
		for i, name := range spec.Names {
			if name.Name != "twilioPermanentCodes" || i >= len(spec.Values) {
				continue
			}
			lit, ok := spec.Values[i].(*ast.CompositeLit)
			if !ok {
				continue
			}
			for _, elt := range lit.Elts {
				kv, ok := elt.(*ast.KeyValueExpr)
				if !ok {
					continue
				}
				key, ok := kv.Key.(*ast.BasicLit)
				if !ok || key.Kind != token.STRING {
					continue
				}
				code, err := strconv.Unquote(key.Value)
				if err == nil {
					prodCodes[code] = true
				}
			}
		}
		return true
	})
	// 反空转：解析不到任何键时双向差集皆空会假绿。
	if len(prodCodes) == 0 {
		t.Fatal("未从 twilio.go 解析到 twilioPermanentCodes 的任何键：护栏会空转")
	}
	testCodes := make(map[string]bool)
	for _, c := range twilioPermanentCases {
		testCodes[c.code] = true
	}
	for code := range prodCodes {
		if !testCodes[code] {
			t.Errorf("生产表错误码 %s 没有对应的分类测试用例", code)
		}
	}
	for code := range testCodes {
		if !prodCodes[code] {
			t.Errorf("测试用例 %s 不在生产表 twilioPermanentCodes 中", code)
		}
	}
}

// TestTwilioSenderEndpointShape 裁决 R41：Profile.Endpoint 是完整 URL，
// 发请求前必须校验它是 scheme==https、host 非空的绝对 URL，不合法则 permanent
// 提前拒（规格 §5.4「提前拒能给出可读原因」），不让畸形 URL 走到网络层才以
// 401/404 暴露。正锚先跑：合法 https URL 必须真的发出去并成功。
func TestTwilioSenderEndpointShape(t *testing.T) {
	t.Run("正锚_合法https绝对URL", func(t *testing.T) {
		srv, got := newTwilioFakeServer(t, http.StatusOK, twilioOKBody)
		sender := &twilioSender{client: srv.Client()}
		res, err := sender.Send(context.Background(), twilioTestProfile(twilioTestEndpoint(srv)), twilioTestSendRequest())
		if err != nil {
			t.Fatalf("合法 endpoint 必须发送成功: %v", err)
		}
		if res.BizId != "SM1" || got.hits.Load() != 1 {
			t.Fatalf("正锚未真的出网: res=%+v hits=%d", res, got.hits.Load())
		}
	})

	cases := []struct {
		name     string
		endpoint string
	}{
		{"空", ""},
		{"纯host不是URL", "api.twilio.com"},
		{"无scheme", "api.twilio.com/2010-04-01/Accounts/AC_fake_sid/Messages.json"},
		{"http不是https", "http://api.twilio.com/2010-04-01/Accounts/AC_fake_sid/Messages.json"},
		{"https但host为空", "https:///2010-04-01/Accounts/AC_fake_sid/Messages.json"},
		{"相对路径", "/2010-04-01/Accounts/AC_fake_sid/Messages.json"},
		{"畸形URL", "https://[bad"},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			tr := &twilioNoDialTransport{}
			sender := &twilioSender{client: &http.Client{Transport: tr}}
			p := twilioTestProfile(c.endpoint)
			_, err := sender.Send(context.Background(), p, twilioTestSendRequest())
			if err == nil {
				t.Fatalf("畸形 endpoint %q 必须提前拒", c.endpoint)
			}
			var se *SendError
			if !errors.As(err, &se) {
				t.Fatalf("错误不是 *SendError: %v", err)
			}
			if se.Class != ClassPermanent {
				t.Fatalf("畸形 endpoint 应为 permanent（不是网络层 transient）, got %s", se.Class)
			}
			if n := tr.calls.Load(); n != 0 {
				t.Fatalf("提前拒不得出网: RoundTrip 被调用 %d 次", n)
			}
		})
	}
}

// TestTwilioSenderPreservesEscapedAccountSid 裁决 R41：Resolve 对 AccountSid 做
// url.PathEscape 后填进 Endpoint，适配器必须原样把转义序列送上请求行——
// 任何一次「解码后再拼」都会把 %2F 变回路径分隔符，路径注入防御失效。
func TestTwilioSenderPreservesEscapedAccountSid(t *testing.T) {
	srv, got := newTwilioFakeServer(t, http.StatusOK, twilioOKBody)
	p := twilioTestProfile(srv.URL + "/2010-04-01/Accounts/AC_fake%2Fsid/Messages.json")
	p.AccountSid = "AC_fake/sid"
	sender := &twilioSender{client: srv.Client()}
	if _, err := sender.Send(context.Background(), p, twilioTestSendRequest()); err != nil {
		t.Fatalf("Send 返回错误: %v", err)
	}
	if !strings.Contains(got.requestURI, "AC_fake%2Fsid") {
		t.Fatalf("请求行未保留 %%2F 转义: %q", got.requestURI)
	}
	if strings.Contains(got.requestURI, "AC_fake/sid") {
		t.Fatalf("请求行把 %%2F 解码成了路径分隔符: %q", got.requestURI)
	}
	if got.path != "/2010-04-01/Accounts/AC_fake/sid/Messages.json" {
		t.Fatalf("服务端解码后的路径不符: %q", got.path)
	}
}

// TestTwilioSenderFactoryTimeout 声明式锁死出厂超时＝规格 §10.4 强制的 10s。
// 不必真等 10s 才能证明出厂值。抓它的破坏：把 twilioSendTimeout 改小。
func TestTwilioSenderFactoryTimeout(t *testing.T) {
	sender, ok := NewTwilioSender().(*twilioSender)
	if !ok {
		t.Fatalf("NewTwilioSender 未返回 *twilioSender: %T", NewTwilioSender())
	}
	if sender.client == nil {
		t.Fatal("出厂 client 为 nil")
	}
	if sender.client.Timeout != 10*time.Second {
		t.Fatalf("出厂发送超时必须是 10s（规格 §10.4）, got %v", sender.client.Timeout)
	}
}

// TestTwilioSenderTimeoutNoRetry 超时判 transient，且服务端只收到一次请求
// （短信按条计费，自动重试＝重复扣费）。用 200ms 超时的 client 跑阻塞 handler，
// 成本 ~0.2s 而不是 10s。
func TestTwilioSenderTimeoutNoRetry(t *testing.T) {
	if testing.Short() {
		t.Skip("-short：跳过阻塞 handler 用例（出厂超时值由 TestTwilioSenderFactoryTimeout 锁）")
	}
	var hits atomic.Int32
	srv := httptest.NewTLSServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		hits.Add(1)
		// 必须先读完请求体再阻塞：net/http 只在「请求体已读完」时才启动
		// background read，而正是它负责发现客户端断开并 cancel r.Context()。
		// Twilio 是 POST 带表单 body，不读 body 就永远收不到 Done，
		// handler 卡死 → srv.Close 的 WaitGroup 永久等待（任务 4 踩过的坑）。
		_, _ = io.Copy(io.Discard, r.Body)
		<-r.Context().Done()
	}))
	t.Cleanup(srv.Close)

	client := srv.Client()
	client.Timeout = 200 * time.Millisecond
	sender := &twilioSender{client: client}

	start := time.Now()
	_, err := sender.Send(context.Background(), twilioTestProfile(twilioTestEndpoint(srv)), twilioTestSendRequest())
	elapsed := time.Since(start)
	if err == nil {
		t.Fatal("超时必须返回错误")
	}
	var se *SendError
	if !errors.As(err, &se) {
		t.Fatalf("错误不是 *SendError: %v", err)
	}
	if se.Class != ClassTransient {
		t.Fatalf("超时应为 transient, got %s", se.Class)
	}
	if elapsed > 5*time.Second {
		t.Fatalf("200ms 超时不该拖到 %v：疑似挂死或重试", elapsed)
	}
	time.Sleep(200 * time.Millisecond)
	if n := hits.Load(); n != 1 {
		t.Fatalf("不得自动重试（按条计费）: 服务端收到 %d 次请求", n)
	}
}

// TestTwilioSenderLimitsResponseBody body 读取必须有上限（走 R38 共享 helper）。
// 用 8 MiB 验证：① 仍正确判失败并留下 ≤200 字节的合法 UTF-8 片段；
// ② 进程累计分配增量远小于 body 本身（证明只读了上限那一段）。
func TestTwilioSenderLimitsResponseBody(t *testing.T) {
	const bodySize = 8 << 20 // 8 MiB
	chunk := bytes.Repeat([]byte("x"), 64<<10)
	srv := httptest.NewTLSServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "text/html")
		w.WriteHeader(http.StatusOK)
		for written := 0; written < bodySize; written += len(chunk) {
			if _, err := w.Write(chunk); err != nil {
				return // 客户端读到上限就关连接，写失败是预期行为
			}
		}
	}))
	t.Cleanup(srv.Close)

	var before, after runtime.MemStats
	runtime.GC()
	runtime.ReadMemStats(&before)
	sender := &twilioSender{client: srv.Client()}
	_, err := sender.Send(context.Background(), twilioTestProfile(twilioTestEndpoint(srv)), twilioTestSendRequest())
	runtime.ReadMemStats(&after)

	if err == nil {
		t.Fatal("超大非 JSON body 必须判失败")
	}
	var se *SendError
	if !errors.As(err, &se) {
		t.Fatalf("错误不是 *SendError: %v", err)
	}
	if se.Class != ClassTransient {
		t.Fatalf("应为 transient, got %s", se.Class)
	}
	if !utf8.ValidString(se.Message) || !strings.Contains(se.Message, strings.Repeat("x", 200)) {
		t.Fatalf("超大 body 仍要留下前 200 字节片段: %q", se.Message)
	}
	if strings.Contains(se.Message, strings.Repeat("x", 201)) {
		t.Fatalf("片段未截断到 200 字节: %q", se.Message)
	}
	const allocBudget = 4 << 20
	delta := after.TotalAlloc - before.TotalAlloc
	t.Logf("读 %d 字节响应体累计分配 %d 字节（预算 %d）", bodySize, delta, allocBudget)
	if delta > allocBudget {
		t.Fatalf("读响应体分配了 %d 字节（> %d）：body 读取没设上限", delta, allocBudget)
	}
}

// TestTwilioSenderNetworkError 连接失败 → transient 的 *SendError（规格 §10.1）。
func TestTwilioSenderNetworkError(t *testing.T) {
	srv := httptest.NewTLSServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {}))
	endpoint := twilioTestEndpoint(srv)
	client := srv.Client()
	srv.Close() // 端口不再监听

	sender := &twilioSender{client: client}
	_, err := sender.Send(context.Background(), twilioTestProfile(endpoint), twilioTestSendRequest())
	if err == nil {
		t.Fatal("连接失败必须返回错误")
	}
	var se *SendError
	if !errors.As(err, &se) {
		t.Fatalf("错误不是 *SendError: %v", err)
	}
	if se.Class != ClassTransient {
		t.Fatalf("网络错误应为 transient, got %s", se.Class)
	}
	if !errors.Is(err, ErrSmsSendFailed) {
		t.Fatal("errors.Is(err, ErrSmsSendFailed) 应为 true")
	}
}

// TestTwilioSenderHTTPStatusClassification 非 2xx 按 ClassifyHTTPStatus 分类，
// REST 异常体的 code 原样保留（规格 §10.1）。异常体的 status 是整数，
// 与 Message 资源的字符串 status 同名不同型——用同一个结构体解会整体失败，
// 那时 code 会被吞成 HTTP_4xx，本用例的 Code 断言会红。
func TestTwilioSenderHTTPStatusClassification(t *testing.T) {
	cases := []struct {
		status int
		want   ErrorClass
	}{
		{http.StatusBadRequest, ClassPermanent},
		{http.StatusUnauthorized, ClassPermanent},
		{http.StatusForbidden, ClassPermanent},
		{http.StatusTooManyRequests, ClassTransient},
		{http.StatusInternalServerError, ClassTransient},
		{http.StatusBadGateway, ClassTransient},
	}
	for _, c := range cases {
		t.Run(strconv.Itoa(c.status), func(t *testing.T) {
			srv, _ := newTwilioFakeServer(t, c.status, twilioExceptionBody(21211, "Invalid 'To' Phone Number", c.status))
			sender := &twilioSender{client: srv.Client()}
			_, err := sender.Send(context.Background(), twilioTestProfile(twilioTestEndpoint(srv)), twilioTestSendRequest())
			var se *SendError
			if !errors.As(err, &se) {
				t.Fatalf("错误不是 *SendError: %v", err)
			}
			if se.Class != c.want {
				t.Fatalf("HTTP %d 应为 %s, got %s", c.status, c.want, se.Class)
			}
			if se.Code != "21211" {
				t.Fatalf("非 2xx 时平台 code 也应原样保留, got %q", se.Code)
			}
			if se.Message != "Invalid 'To' Phone Number" {
				t.Fatalf("平台原始消息被改写: %q", se.Message)
			}
		})
	}

	t.Run("非JSON异常体回落HTTP码并留片段", func(t *testing.T) {
		srv, _ := newTwilioFakeServer(t, http.StatusBadGateway, "<html>502 Bad Gateway</html>")
		sender := &twilioSender{client: srv.Client()}
		_, err := sender.Send(context.Background(), twilioTestProfile(twilioTestEndpoint(srv)), twilioTestSendRequest())
		var se *SendError
		if !errors.As(err, &se) {
			t.Fatalf("错误不是 *SendError: %v", err)
		}
		if se.Code != "HTTP_502" {
			t.Fatalf("无平台码时应回落 HTTP_502, got %q", se.Code)
		}
		if !strings.Contains(se.Message, "<html>502 Bad Gateway</html>") {
			t.Fatalf("Message 未保留 body 片段: %q", se.Message)
		}
	})
}

package sms

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"go/ast"
	"go/parser"
	"go/token"
	"io"
	"net"
	"net/http"
	"net/http/httptest"
	"runtime"
	"strconv"
	"strings"
	"sync/atomic"
	"testing"
	"time"
	"unicode/utf8"
)

// 本文件锁死腾讯云适配器的行为契约（规格 §5.3／§5.6／§10.1）：
// 请求形态与官方 SDK 一致（POST JSON body、PhoneNumberSet 带 +、
// TemplateParamSet 是位置数组、国际 SignName 键整个不出现）、
// SendStatusSet[0].Code == "Ok" 才算成功（大小写与阿里云的 "OK" 不同）、
// HTTP 200 非 JSON 绝不伪装成功、permanent 错误码表逐条覆盖、
// 10s 超时不重试、响应体读取有上限。
// 全部用假凭据 + 本地 httptest，永不访问外网。

const tencentOKBody = `{"Response":{"SendStatusSet":[{"SerialNo":"S-1","PhoneNumber":"+8613900001111","IsoCode":"CN","Code":"Ok","Message":"send success","Fee":1,"SessionContext":"42"}],"RequestId":"R-1"}}`

type tencentCapturedRequest struct {
	method   string
	host     string
	path     string
	rawQuery string
	header   http.Header
	body     string
}

func newTencentFakeServer(t *testing.T, status int, respBody string) (*httptest.Server, *tencentCapturedRequest) {
	t.Helper()
	got := &tencentCapturedRequest{}
	srv := httptest.NewTLSServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		b, _ := io.ReadAll(r.Body)
		got.method = r.Method
		got.host = r.Host
		got.path = r.URL.Path
		got.rawQuery = r.URL.RawQuery
		got.header = r.Header.Clone()
		got.body = string(b)
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(status)
		fmt.Fprint(w, respBody)
	}))
	t.Cleanup(srv.Close)
	return srv, got
}

func tencentTestEndpoint(srv *httptest.Server) string {
	return strings.TrimPrefix(srv.URL, "https://")
}

func tencentTestProfile(endpoint string) Profile {
	return Profile{
		Provider:    "tencent",
		Region:      "domestic",
		Endpoint:    endpoint,
		SecretId:    "AKIDFakeAdapterSecretId00000000000",
		SecretKey:   "FakeAdapterSecretKey000000000000",
		SmsSdkAppId: "1400006666",
		CloudRegion: "ap-guangzhou",
		SignName:    "安企测试",
		TemplateId:  "1234567",
		// ParamName 对腾讯云留空：TemplateParamSet 是位置数组（规格 §3.6）。
	}
}

func tencentTestSendRequest() SendRequest {
	return SendRequest{To: "+8613900001111", Scene: SceneLogin, Code: "123456", OutId: "9001"}
}

// tencentRedirectClient 把发往真实 endpoint 域名的连接重定向到本地假服务器，
// 保留 Host 头——用于「请求确实打到对应 host」与向量同形验证。
// 测试证书只对 127.0.0.1/example.com 有效，这里只验签名与路由，不验 TLS。
func tencentRedirectClient(srv *httptest.Server) *http.Client {
	tr := srv.Client().Transport.(*http.Transport).Clone()
	tr.TLSClientConfig.InsecureSkipVerify = true
	addr := srv.Listener.Addr().String()
	tr.DialContext = func(ctx context.Context, network, _ string) (net.Conn, error) {
		return (&net.Dialer{}).DialContext(ctx, network, addr)
	}
	return &http.Client{Transport: tr}
}

// tencentStatusBody 业务级失败：Code 在 SendStatusSet[0]。
func tencentStatusBody(code, message string) string {
	return fmt.Sprintf(`{"Response":{"SendStatusSet":[{"SerialNo":"S-1","PhoneNumber":"+8613900001111","IsoCode":"CN","Code":%q,"Message":%q,"Fee":1,"SessionContext":"9001"}],"RequestId":"R-1"}}`, code, message)
}

// tencentErrorBody 接口/网关级失败：Code 在 Response.Error（HTTP 仍可能是 200）。
// 形状核验自官方 SDK common@v1.3.172/http/response.go:53-61 的 ErrorResponse。
func tencentErrorBody(code, message string) string {
	return fmt.Sprintf(`{"Response":{"Error":{"Code":%q,"Message":%q},"RequestId":"R-1"}}`, code, message)
}

// tencentBodyFields 把请求体解析成通用 map，便于断言「键是否出现」。
func tencentBodyFields(t *testing.T, body string) map[string]interface{} {
	t.Helper()
	var m map[string]interface{}
	if err := json.Unmarshal([]byte(body), &m); err != nil {
		t.Fatalf("请求体不是 JSON 对象: %v\nbody: %s", err, body)
	}
	return m
}

// TestTencentSenderSuccess 断言成功判据（SendStatusSet[0].Code == "Ok"）、回执
// 字段，以及与官方 SDK 一致的请求形态：POST /、JSON body、PhoneNumberSet 带 +、
// TemplateParamSet 是位置数组、SessionContext 透传 OutId、四个 X-TC-* 头齐全。
// 抓它的破坏：把成功判据写成 HTTP 200、号码去 + 发出、TemplateParamSet 写成
// 对象、漏 SessionContext、漏签名头、Content-Type 带 charset。
func TestTencentSenderSuccess(t *testing.T) {
	srv, got := newTencentFakeServer(t, http.StatusOK, tencentOKBody)
	p := tencentTestProfile(tencentTestEndpoint(srv))
	sender := &tencentSender{client: srv.Client()}
	res, err := sender.Send(context.Background(), p, tencentTestSendRequest())
	if err != nil {
		t.Fatalf("Send 返回错误: %v", err)
	}
	if res.BizId != "S-1" || res.RawCode != "Ok" || res.RawMsg != "send success" {
		t.Fatalf("回执字段不符: %+v", res)
	}
	if got.method != http.MethodPost || got.path != "/" || got.rawQuery != "" {
		t.Fatalf("请求形态不符: %s %s?%s", got.method, got.path, got.rawQuery)
	}
	if ct := got.header.Get("Content-Type"); ct != "application/json" {
		t.Fatalf("Content-Type = %q, want application/json（TC3 签的就是这个字面值）", ct)
	}

	fields := tencentBodyFields(t, got.body)
	var phones []string
	if err := json.Unmarshal([]byte(got.body), &struct {
		PhoneNumberSet *[]string `json:"PhoneNumberSet"`
	}{PhoneNumberSet: &phones}); err != nil {
		t.Fatalf("PhoneNumberSet 不是字符串数组: %v", err)
	}
	if len(phones) != 1 || phones[0] != "+8613900001111" {
		t.Fatalf("PhoneNumberSet = %v, want 带 + 的 E.164 单元素数组 [+8613900001111]", phones)
	}
	var params []string
	if err := json.Unmarshal([]byte(got.body), &struct {
		TemplateParamSet *[]string `json:"TemplateParamSet"`
	}{TemplateParamSet: &params}); err != nil {
		t.Fatalf("TemplateParamSet 不是位置数组（写成对象了？）: %v", err)
	}
	if len(params) != 1 || params[0] != "123456" {
		t.Fatalf("TemplateParamSet = %v, want [123456]", params)
	}
	for _, f := range []struct {
		key  string
		want string
	}{
		{"SmsSdkAppId", "1400006666"},
		{"TemplateId", "1234567"},
		{"SignName", "安企测试"},
		{"SessionContext", "9001"},
	} {
		if v, _ := fields[f.key].(string); v != f.want {
			t.Fatalf("body[%s] = %q, want %q", f.key, v, f.want)
		}
	}

	for _, h := range []struct {
		name string
		want string
	}{
		{"X-TC-Action", "SendSms"},
		{"X-TC-Version", "2021-01-11"},
		{"X-TC-Region", "ap-guangzhou"},
	} {
		if v := got.header.Get(h.name); v != h.want {
			t.Fatalf("%s = %q, want %q", h.name, v, h.want)
		}
	}
	ts, err := strconv.ParseInt(got.header.Get("X-TC-Timestamp"), 10, 64)
	if err != nil {
		t.Fatalf("X-TC-Timestamp 不是 Unix 秒: %q", got.header.Get("X-TC-Timestamp"))
	}
	if d := time.Since(time.Unix(ts, 0)); d < -time.Minute || d > time.Minute {
		t.Fatalf("X-TC-Timestamp 偏离当前时间 %v", d)
	}
	auth := got.header.Get("Authorization")
	if !strings.HasPrefix(auth, "TC3-HMAC-SHA256 Credential="+p.SecretId+"/") {
		t.Fatalf("Authorization 形态不符: %q", auth)
	}
	if !strings.Contains(auth, "/sms/tc3_request, SignedHeaders=content-type;host, Signature=") {
		t.Fatalf("Authorization 缺少 scope/SignedHeaders 段: %q", auth)
	}
}

// TestTencentSenderBusinessFailure：HTTP 200 不等于成功。SendStatusSet[0].Code
// 非 "Ok" 必须返回 *SendError，原始码原样保留，且能 errors.Is 到通用哨兵。
// 单独钉住大小写：腾讯云成功码是 "Ok"，阿里云才是 "OK"——把 "OK" 当成功会
// 让所有腾讯云失败静默变成「发送成功」。
func TestTencentSenderBusinessFailure(t *testing.T) {
	t.Run("日限流码原样保留", func(t *testing.T) {
		srv, _ := newTencentFakeServer(t, http.StatusOK,
			tencentStatusBody("LimitExceeded.PhoneNumberDailyLimit", "单个手机号日下发短信条数超过设定的上限"))
		sender := &tencentSender{client: srv.Client()}
		res, err := sender.Send(context.Background(), tencentTestProfile(tencentTestEndpoint(srv)), tencentTestSendRequest())
		if err == nil {
			t.Fatal("HTTP 200 但业务码非 Ok 必须返回错误")
		}
		var se *SendError
		if !errors.As(err, &se) {
			t.Fatalf("错误不是 *SendError: %v", err)
		}
		if se.Code != "LimitExceeded.PhoneNumberDailyLimit" {
			t.Fatalf("原始码被吞: %q", se.Code)
		}
		if !strings.Contains(se.Message, "单个手机号日下发短信条数超过设定的上限") {
			t.Fatalf("平台原始消息丢失: %q", se.Message)
		}
		if !errors.Is(err, ErrSmsSendFailed) {
			t.Fatal("errors.Is(err, ErrSmsSendFailed) 应为 true")
		}
		if res.BizId != "" {
			t.Fatalf("失败时不应有回执: %+v", res)
		}
	})

	t.Run("大写OK不是腾讯云的成功码", func(t *testing.T) {
		srv, _ := newTencentFakeServer(t, http.StatusOK, tencentStatusBody("OK", "send success"))
		sender := &tencentSender{client: srv.Client()}
		res, err := sender.Send(context.Background(), tencentTestProfile(tencentTestEndpoint(srv)), tencentTestSendRequest())
		if err == nil {
			t.Fatal(`Code == "OK"（大写 K）必须判失败：腾讯云成功码是 "Ok"`)
		}
		var se *SendError
		if !errors.As(err, &se) {
			t.Fatalf("错误不是 *SendError: %v", err)
		}
		if se.Code != "OK" {
			t.Fatalf("原始码被改写: %q", se.Code)
		}
		if res.BizId != "" {
			t.Fatalf("失败时不应有回执: %+v", res)
		}
	})

	t.Run("网关级ResponseError也是失败", func(t *testing.T) {
		srv, _ := newTencentFakeServer(t, http.StatusOK,
			tencentErrorBody("AuthFailure.SignatureFailure", "签名错误"))
		sender := &tencentSender{client: srv.Client()}
		_, err := sender.Send(context.Background(), tencentTestProfile(tencentTestEndpoint(srv)), tencentTestSendRequest())
		var se *SendError
		if !errors.As(err, &se) {
			t.Fatalf("错误不是 *SendError: %v", err)
		}
		if se.Code != "AuthFailure.SignatureFailure" {
			t.Fatalf("Response.Error.Code 被吞: %q", se.Code)
		}
		if se.Class != ClassPermanent {
			t.Fatalf("AuthFailure.SignatureFailure 应为 permanent, got %s", se.Class)
		}
	})
}

// TestTencentSenderNonJSONBody（审查重点 5）：网关插 HTML 错误页 / CDN 拦截时
// HTTP 200 但 body 不是 JSON → 必须判失败，原始 body 片段（截断 200 字节）
// 留进 Message，分类 transient，绝不返回 nil error。
// Message 的唯一目的地是 sms_logs.err_msg（规格 §10.4），所以它必须是合法
// UTF-8：按字节硬切 200 会切在汉字中间，utf8mb4 列拒收非法字节序列时整条发送
// 流水写入失败，最需要留痕的场景反而什么线索都没留下。
func TestTencentSenderNonJSONBody(t *testing.T) {
	cases := []struct {
		name string
		body string
	}{
		{"HTML错误页", "<html>502 Bad Gateway</html>"},
		{"超长body截断200字节", strings.Repeat("x", 500)},
		{"多字节中文拦截页", strings.Repeat("网关拦截", 100)},
		{"JSON但缺Response", "{}"},
		{"JSON但状态码为空", `{"Response":{"SendStatusSet":[{}],"RequestId":"R"}}`},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			srv, _ := newTencentFakeServer(t, http.StatusOK, c.body)
			sender := &tencentSender{client: srv.Client()}
			_, err := sender.Send(context.Background(), tencentTestProfile(tencentTestEndpoint(srv)), tencentTestSendRequest())
			if err == nil {
				t.Fatal("非 JSON / 无状态码的 200 响应绝不能伪装成功")
			}
			var se *SendError
			if !errors.As(err, &se) {
				t.Fatalf("错误不是 *SendError: %v", err)
			}
			if se.Class != ClassTransient {
				t.Fatalf("应为 transient, got %s", se.Class)
			}
			if !utf8.ValidString(se.Message) {
				t.Fatalf("Message 含非法 UTF-8 字节，写 sms_logs.err_msg 会整条失败: %q", se.Message)
			}
			if !utf8.ValidString(se.Error()) {
				t.Fatalf("Error() 含非法 UTF-8 字节: %q", se.Error())
			}
			if c.name == "HTML错误页" && !strings.Contains(se.Message, "<html>502 Bad Gateway</html>") {
				t.Fatalf("Message 未保留原始 body 片段: %q", se.Message)
			}
			if c.name == "超长body截断200字节" {
				if !strings.Contains(se.Message, strings.Repeat("x", 200)) {
					t.Fatalf("Message 未保留前 200 字节: %q", se.Message)
				}
				if strings.Contains(se.Message, strings.Repeat("x", 201)) {
					t.Fatalf("Message 未截断到 200 字节: %q", se.Message)
				}
			}
			if c.name == "多字节中文拦截页" {
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

// TestTencentSenderEmptyStatusSet：空数组不能当成功（平台没给出任何下发状态，
// 无从判断是否真的发出去了，必须落失败让站长可查）。
func TestTencentSenderEmptyStatusSet(t *testing.T) {
	srv, _ := newTencentFakeServer(t, http.StatusOK, `{"Response":{"SendStatusSet":[],"RequestId":"R"}}`)
	sender := &tencentSender{client: srv.Client()}
	res, err := sender.Send(context.Background(), tencentTestProfile(tencentTestEndpoint(srv)), tencentTestSendRequest())
	if err == nil {
		t.Fatal("SendStatusSet 为空数组必须判失败")
	}
	var se *SendError
	if !errors.As(err, &se) {
		t.Fatalf("错误不是 *SendError: %v", err)
	}
	if se.Class != ClassTransient {
		t.Fatalf("应为 transient, got %s", se.Class)
	}
	if res.BizId != "" {
		t.Fatalf("失败时不应有回执: %+v", res)
	}
}

// tencentPermanentCases 是规格 §5.6 腾讯云行每一类 permanent 错误的逐条用例。
// 码值核验自官方 SDK sms@v1.3.172/v20210111/errors.go（产品特有码）与
// 腾讯云公共错误码文档（AuthFailure.* 网关级码，2026-11-14 核验）。
// 与生产表 tencentPermanentCodes 的双向穷举由
// TestTencentPermanentCodeTableExhaustive 用 go/ast 锁定，不写死总数。
var tencentPermanentCases = []struct {
	code     string
	category string
}{
	{"FailedOperation.SignatureIncorrectOrUnapproved", "签名未审核"},
	{"FailedOperation.MissingSignatureList", "签名未审核"},
	{"FailedOperation.TemplateIncorrectOrUnapproved", "模板未审核"},
	{"FailedOperation.MissingTemplateList", "模板未审核"},
	{"FailedOperation.TemplateUnapprovedOrNotExist", "模板未审核"},
	{"AuthFailure.SecretIdNotFound", "SecretId无效"},
	{"AuthFailure.InvalidSecretId", "SecretId无效"},
	{"AuthFailure.SignatureFailure", "SecretId无效"},
	{"AuthFailure.UnauthorizedOperation", "SecretId无效"},
	{"InvalidParameterValue.SdkAppIdNotExist", "SmsSdkAppId不存在"},
	{"UnauthorizedOperation.SmsSdkAppIdVerifyFail", "SmsSdkAppId不存在"},
	{"FailedOperation.InsufficientBalanceInSmsPackage", "余额不足"},
	{"UnauthorizedOperation.ServiceSuspendDueToArrears", "余额不足"},
	{"InvalidParameterValue.IncorrectPhoneNumber", "号码格式非法"},
	{"FailedOperation.PhoneNumberParseFail", "号码格式非法"},
}

// TestTencentSenderErrorCodeClassification 表驱动覆盖每一类 permanent 错误。
// 每个码都走两种响应形状（SendStatusSet[0].Code 与 Response.Error.Code）：
// 平台把哪个码放哪一层无法离线核实，分类必须与形状无关。
// 未列入表的码 → transient 且 Code 原样保留（排查唯一线索）。
func TestTencentSenderErrorCodeClassification(t *testing.T) {
	shapes := []struct {
		name string
		body func(code, message string) string
	}{
		{"SendStatusSet", tencentStatusBody},
		{"ResponseError", tencentErrorBody},
	}
	for _, c := range tencentPermanentCases {
		for _, shape := range shapes {
			t.Run(c.code+"/"+shape.name, func(t *testing.T) {
				srv, _ := newTencentFakeServer(t, http.StatusOK, shape.body(c.code, "raw message"))
				sender := &tencentSender{client: srv.Client()}
				_, err := sender.Send(context.Background(), tencentTestProfile(tencentTestEndpoint(srv)), tencentTestSendRequest())
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

	for _, shape := range shapes {
		t.Run("未列入表的码原样保留且transient/"+shape.name, func(t *testing.T) {
			srv, _ := newTencentFakeServer(t, http.StatusOK, shape.body("FailedOperation.SomeFutureUnknownCode", "unknown"))
			sender := &tencentSender{client: srv.Client()}
			_, err := sender.Send(context.Background(), tencentTestProfile(tencentTestEndpoint(srv)), tencentTestSendRequest())
			var se *SendError
			if !errors.As(err, &se) {
				t.Fatalf("错误不是 *SendError: %v", err)
			}
			if se.Class != ClassTransient {
				t.Fatalf("未列入表的码应为 transient, got %s", se.Class)
			}
			if se.Code != "FailedOperation.SomeFutureUnknownCode" {
				t.Fatalf("未识别的码必须原样保留, got %q", se.Code)
			}
		})

		t.Run("平台消息为空时回落中文提示/"+shape.name, func(t *testing.T) {
			srv, _ := newTencentFakeServer(t, http.StatusOK, shape.body("FailedOperation.InsufficientBalanceInSmsPackage", ""))
			sender := &tencentSender{client: srv.Client()}
			_, err := sender.Send(context.Background(), tencentTestProfile(tencentTestEndpoint(srv)), tencentTestSendRequest())
			var se *SendError
			if !errors.As(err, &se) {
				t.Fatalf("错误不是 *SendError: %v", err)
			}
			if se.Message == "" {
				t.Fatal("平台消息为空时应回落已知错误码的中文提示")
			}
		})
	}
}

// TestTencentPermanentCodeTableExhaustive 用 go/ast 穷举生产表
// tencentPermanentCodes 的键，与测试用例表双向比对：新增错误码漏写测试会红，
// 测试写了表里没有的码也会红。不写死错误码总数（AGENTS.md §6）。
func TestTencentPermanentCodeTableExhaustive(t *testing.T) {
	fset := token.NewFileSet()
	file, err := parser.ParseFile(fset, "tencent.go", nil, 0)
	if err != nil {
		t.Fatalf("解析 tencent.go 失败: %v", err)
	}
	prodCodes := make(map[string]bool)
	ast.Inspect(file, func(n ast.Node) bool {
		spec, ok := n.(*ast.ValueSpec)
		if !ok {
			return true
		}
		for i, name := range spec.Names {
			if name.Name != "tencentPermanentCodes" || i >= len(spec.Values) {
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
		t.Fatal("未从 tencent.go 解析到 tencentPermanentCodes 的任何键：护栏会空转")
	}
	testCodes := make(map[string]bool)
	for _, c := range tencentPermanentCases {
		testCodes[c.code] = true
	}
	for code := range prodCodes {
		if !testCodes[code] {
			t.Errorf("生产表错误码 %s 没有对应的分类测试用例", code)
		}
	}
	for code := range testCodes {
		if !prodCodes[code] {
			t.Errorf("测试用例 %s 不在生产表 tencentPermanentCodes 中", code)
		}
	}
}

// TestTencentSenderSignNameVariants 国内 SignName 非空 → 请求体带该值；
// 国际 SignName 为空 → 请求体**不带** SignName 键（腾讯云国际选填，
// 传空串会被平台当签名内容校验）。两组输入各冻在一组向量里。
func TestTencentSenderSignNameVariants(t *testing.T) {
	vectors := loadTencentVectors(t)
	var sawNonEmpty, sawEmpty bool
	for _, v := range vectors {
		t.Run(v.Name, func(t *testing.T) {
			srv, got := newTencentFakeServer(t, http.StatusOK, tencentOKBody)
			sender := &tencentSender{client: srv.Client()}
			p := tencentVectorProfile(v)
			p.Endpoint = tencentTestEndpoint(srv)
			req := SendRequest{To: v.Input.To, Scene: SceneLogin, Code: v.Input.Code, OutId: v.Input.OutId}
			if _, err := sender.Send(context.Background(), p, req); err != nil {
				t.Fatalf("Send 失败: %v", err)
			}
			fields := tencentBodyFields(t, got.body)
			raw, present := fields["SignName"]
			if v.Input.SignName != "" {
				sawNonEmpty = true
				if !present {
					t.Fatalf("国内 SignName 非空时请求体必须带该键: %s", got.body)
				}
				if raw != v.Input.SignName {
					t.Fatalf("SignName = %q, want %q", raw, v.Input.SignName)
				}
				if !strings.Contains(got.body, `"SignName":"`+v.Input.SignName+`"`) {
					t.Fatalf("SignName 未按 SDK 的字面形态出现: %s", got.body)
				}
			} else {
				sawEmpty = true
				if present {
					t.Fatalf("国际 SignName 为空时请求体不得出现 SignName 键: %s", got.body)
				}
				if strings.Contains(got.body, "SignName") {
					t.Fatalf("请求体仍含 SignName 字样: %s", got.body)
				}
			}
		})
	}
	// 反空转：两种变体都必须真的跑到，否则本护栏从未执行。
	if !sawNonEmpty || !sawEmpty {
		t.Fatalf("向量必须同时覆盖 SignName 非空与为空两种变体: nonEmpty=%v empty=%v", sawNonEmpty, sawEmpty)
	}
}

// TestTencentSenderMatchesOfficialVector wire 请求与 SDK 向量逐字节同形，
// 且把适配器实际发出的请求用向量的时间戳重签后，Authorization 与 SDK 向量
// 逐字节一致。适配器多发/漏发任何参与签名的头、body 字段顺序或编码不同都会红。
func TestTencentSenderMatchesOfficialVector(t *testing.T) {
	for _, v := range loadTencentVectors(t) {
		t.Run(v.Name, func(t *testing.T) {
			srv, got := newTencentFakeServer(t, http.StatusOK, tencentOKBody)
			sender := &tencentSender{client: tencentRedirectClient(srv)}
			p := tencentVectorProfile(v)
			req := SendRequest{To: v.Input.To, Scene: SceneLogin, Code: v.Input.Code, OutId: v.Input.OutId}
			if _, err := sender.Send(context.Background(), p, req); err != nil {
				t.Fatalf("Send 失败: %v", err)
			}
			if got.host != v.Endpoint {
				t.Fatalf("请求未打到对应 host: got %q, want %q", got.host, v.Endpoint)
			}
			if got.method != v.Method || got.path != v.Path || got.rawQuery != v.RawQuery {
				t.Fatalf("请求形态不符: %s %s?%s", got.method, got.path, got.rawQuery)
			}
			if got.body != v.Body {
				t.Fatalf("body 与 SDK 向量不是逐字节一致\n got: %s\nwant: %s", got.body, v.Body)
			}
			if ct := got.header.Get("Content-Type"); ct != v.Headers["content-type"] {
				t.Fatalf("Content-Type = %q, want %q", ct, v.Headers["content-type"])
			}

			// 用适配器实际发出的请求（header/body/host）+ 向量的时间戳重签，
			// 必须得到 SDK 的 Authorization。
			rebuilt, err := http.NewRequest(got.method, "https://"+got.host+got.path, strings.NewReader(got.body))
			if err != nil {
				t.Fatalf("重建请求失败: %v", err)
			}
			rebuilt.Host = got.host
			for k, vals := range got.header {
				for _, val := range vals {
					rebuilt.Header.Add(k, val)
				}
			}
			if err := SignTencent(rebuilt, p, []byte(got.body), tencentVectorTime(t, v)); err != nil {
				t.Fatalf("SignTencent 失败: %v", err)
			}
			if auth := rebuilt.Header.Get("Authorization"); auth != v.Authorization {
				t.Fatalf("适配器请求重签后与 SDK 向量不一致\n got: %s\nwant: %s", auth, v.Authorization)
			}
		})
	}
}

// TestTencentSenderFactoryTimeout 声明式锁死出厂超时＝规格 §10.4 强制的 10s。
// 不必真等 10s 才能证明出厂值。抓它的破坏：把 tencentSendTimeout 改小。
func TestTencentSenderFactoryTimeout(t *testing.T) {
	sender, ok := NewTencentSender().(*tencentSender)
	if !ok {
		t.Fatalf("NewTencentSender 未返回 *tencentSender: %T", NewTencentSender())
	}
	if sender.client == nil {
		t.Fatal("出厂 client 为 nil")
	}
	if sender.client.Timeout != 10*time.Second {
		t.Fatalf("出厂发送超时必须是 10s（规格 §10.4）, got %v", sender.client.Timeout)
	}
}

// TestTencentSenderTimeoutNoRetry 超时判 transient，且服务端只收到一次请求
// （短信按条计费，自动重试＝重复扣费）。用 200ms 超时的 client 跑同一个阻塞
// handler，成本 ~0.2s 而不是 10s；不断言 9–12s 墙钟窗口（负载机上会假红），
// 只留一个宽松的上界当挂死护栏。
func TestTencentSenderTimeoutNoRetry(t *testing.T) {
	if testing.Short() {
		t.Skip("-short：跳过阻塞 handler 用例（出厂超时值由 TestTencentSenderFactoryTimeout 锁）")
	}
	var hits atomic.Int32
	srv := httptest.NewTLSServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		hits.Add(1)
		// 必须先读完请求体再阻塞：net/http 只在「请求体已读完」时才启动
		// background read（server.go:2143-2147 的 bodyRemains 分支），而
		// 正是它负责发现客户端断开并 cancel r.Context()。腾讯云是 POST 带
		// JSON body（阿里云是 nil body，所以那边不需要这一步），不读 body
		// 就永远收不到 Done，handler 卡死 → srv.Close 的 WaitGroup 永久等待。
		_, _ = io.Copy(io.Discard, r.Body)
		<-r.Context().Done()
	}))
	t.Cleanup(srv.Close)

	client := srv.Client()
	client.Timeout = 200 * time.Millisecond
	sender := &tencentSender{client: client}

	start := time.Now()
	_, err := sender.Send(context.Background(), tencentTestProfile(tencentTestEndpoint(srv)), tencentTestSendRequest())
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

// TestTencentSenderLimitsResponseBody：body 读取必须有上限（走 R38 共享 helper）。
// 用 8 MiB（有限但远超 200 字节）验证：① 仍正确判失败并留下 ≤200 字节的合法
// UTF-8 片段；② 进程累计分配增量远小于 body 本身（证明只读了上限那一段）。
// 抓它的破坏：去掉 io.LimitReader / 绕过 readSMSResponseBody。
func TestTencentSenderLimitsResponseBody(t *testing.T) {
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
	sender := &tencentSender{client: srv.Client()}
	_, err := sender.Send(context.Background(), tencentTestProfile(tencentTestEndpoint(srv)), tencentTestSendRequest())
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
	// 上限 64 KiB：ReadAll 的倍增缓冲 + TLS 握手开销远小于 4 MiB；
	// 不设上限时 8 MiB body 会让累计分配涨到 ~16 MiB 以上。
	const allocBudget = 4 << 20
	delta := after.TotalAlloc - before.TotalAlloc
	t.Logf("读 %d 字节响应体累计分配 %d 字节（预算 %d）", bodySize, delta, allocBudget)
	if delta > allocBudget {
		t.Fatalf("读响应体分配了 %d 字节（> %d）：body 读取没设上限", delta, allocBudget)
	}
}

// TestTencentSenderNetworkError 连接失败 → transient 的 *SendError（规格 §10.1）。
func TestTencentSenderNetworkError(t *testing.T) {
	srv := httptest.NewTLSServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {}))
	endpoint := tencentTestEndpoint(srv)
	client := srv.Client()
	srv.Close() // 端口不再监听

	sender := &tencentSender{client: client}
	_, err := sender.Send(context.Background(), tencentTestProfile(endpoint), tencentTestSendRequest())
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

// TestTencentSenderHTTPStatusClassification 非 2xx 按 ClassifyHTTPStatus 分类，
// 平台返回的 Code 原样保留（规格 §10.1）。
func TestTencentSenderHTTPStatusClassification(t *testing.T) {
	cases := []struct {
		status int
		want   ErrorClass
	}{
		{http.StatusBadRequest, ClassPermanent},
		{http.StatusForbidden, ClassPermanent},
		{http.StatusTooManyRequests, ClassTransient},
		{http.StatusInternalServerError, ClassTransient},
		{http.StatusBadGateway, ClassTransient},
	}
	for _, c := range cases {
		t.Run(strconv.Itoa(c.status), func(t *testing.T) {
			srv, _ := newTencentFakeServer(t, c.status, tencentErrorBody("RequestLimitExceeded", "raw"))
			sender := &tencentSender{client: srv.Client()}
			_, err := sender.Send(context.Background(), tencentTestProfile(tencentTestEndpoint(srv)), tencentTestSendRequest())
			var se *SendError
			if !errors.As(err, &se) {
				t.Fatalf("错误不是 *SendError: %v", err)
			}
			if se.Class != c.want {
				t.Fatalf("HTTP %d 应为 %s, got %s", c.status, c.want, se.Class)
			}
			if se.Code != "RequestLimitExceeded" {
				t.Fatalf("非 2xx 时平台 Code 也应原样保留, got %q", se.Code)
			}
		})
	}
}

// TestTencentSenderOutIdOmittedWhenEmpty OutId 为空时不写 SessionContext 键
// （与阿里云 OutId 的处理一致：空值不是有效上下文，平台侧 <512 字节校验）。
func TestTencentSenderOutIdOmittedWhenEmpty(t *testing.T) {
	srv, got := newTencentFakeServer(t, http.StatusOK, tencentOKBody)
	sender := &tencentSender{client: srv.Client()}
	req := tencentTestSendRequest()
	req.OutId = ""
	if _, err := sender.Send(context.Background(), tencentTestProfile(tencentTestEndpoint(srv)), req); err != nil {
		t.Fatalf("Send 失败: %v", err)
	}
	if _, present := tencentBodyFields(t, got.body)["SessionContext"]; present {
		t.Fatalf("OutId 为空时不应出现 SessionContext 键: %s", got.body)
	}
}

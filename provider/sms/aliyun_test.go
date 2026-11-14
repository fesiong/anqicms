package sms

import (
	"context"
	"errors"
	"fmt"
	"go/ast"
	"go/parser"
	"go/token"
	"io"
	"net"
	"net/http"
	"net/http/httptest"
	"net/url"
	"strconv"
	"strings"
	"sync/atomic"
	"testing"
	"time"
)

// 本文件锁死阿里云适配器的行为契约（规格 §5.2／§5.6／§10.1）：
// 请求形态与官方 SDK 一致（参数在 query、空 body）、Code=="OK" 才算成功、
// HTTP 200 非 JSON 绝不伪装成功、permanent 错误码表逐条覆盖、
// 国内/国际两个 endpoint 都打到对应 host 且签名与向量一致、10s 超时不重试。
// 全部用假凭据 + 本地 httptest，永不访问外网。

// aliyunEmptyBodySHA256 是空 body 的 SHA256（手工核对的已知常量，与向量一致）。
const aliyunEmptyBodySHA256 = "e3b0c44298fc1c149afbf4c8996fb92427ae41e4649b934ca495991b7852b855"

const aliyunOKBody = `{"Code":"OK","Message":"OK","BizId":"1234567890","RequestId":"R-1"}`

type aliyunCapturedRequest struct {
	method   string
	host     string
	path     string
	rawQuery string
	query    url.Values
	header   http.Header
	body     string
}

func newAliyunFakeServer(t *testing.T, status int, respBody string) (*httptest.Server, *aliyunCapturedRequest) {
	t.Helper()
	got := &aliyunCapturedRequest{}
	srv := httptest.NewTLSServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		b, _ := io.ReadAll(r.Body)
		got.method = r.Method
		got.host = r.Host
		got.path = r.URL.Path
		got.rawQuery = r.URL.RawQuery
		got.query = r.URL.Query()
		got.header = r.Header.Clone()
		got.body = string(b)
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(status)
		fmt.Fprint(w, respBody)
	}))
	t.Cleanup(srv.Close)
	return srv, got
}

func aliyunTestEndpoint(srv *httptest.Server) string {
	return strings.TrimPrefix(srv.URL, "https://")
}

func aliyunTestProfile(endpoint string) Profile {
	return Profile{
		Provider:        "aliyun",
		Region:          "domestic",
		Endpoint:        endpoint,
		AccessKeyId:     "LTAI5tFakeAdapterAK000",
		AccessKeySecret: "FakeAdapterSecret00000000000000",
		SignName:        "安企测试",
		TemplateId:      "SMS_2026111400001",
		ParamName:       "code",
	}
}

func aliyunTestSendRequest() SendRequest {
	return SendRequest{To: "+8613900001111", Scene: SceneLogin, Code: "123456", OutId: "9001"}
}

// aliyunRedirectClient 把发往真实 endpoint 域名的连接重定向到本地假服务器，
// 保留 Host 头——用于「请求确实打到对应 host」与生产超时行为测试。
// 测试证书只对 127.0.0.1/example.com 有效，这里只验签名与路由，不验 TLS。
func aliyunRedirectClient(srv *httptest.Server) *http.Client {
	tr := srv.Client().Transport.(*http.Transport).Clone()
	tr.TLSClientConfig.InsecureSkipVerify = true
	addr := srv.Listener.Addr().String()
	tr.DialContext = func(ctx context.Context, network, _ string) (net.Conn, error) {
		return (&net.Dialer{}).DialContext(ctx, network, addr)
	}
	return &http.Client{Transport: tr}
}

// TestAliyunSenderSuccess 断言成功判据（Code=="OK"）、回执字段，以及与官方 SDK
// 一致的请求形态：POST /、参数在 query、空 body、PhoneNumbers 不带 +（裁决 R29）、
// TemplateParam 键名取 Profile.ParamName、OutId 透传、五个签名头齐全。
// 抓它的破坏：把业务成功判据写成 HTTP 200、号码带去 + 发出、模板变量名写死 code、
// 漏 OutId、漏签名头。
func TestAliyunSenderSuccess(t *testing.T) {
	cases := []struct {
		name      string
		paramName string
		wantParam string
	}{
		{"默认变量名code", "code", `{"code":"123456"}`},
		{"自定义变量名", "verifyCode", `{"verifyCode":"123456"}`},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			srv, got := newAliyunFakeServer(t, http.StatusOK, aliyunOKBody)
			p := aliyunTestProfile(aliyunTestEndpoint(srv))
			p.ParamName = c.paramName
			sender := &aliyunSender{client: srv.Client()}
			res, err := sender.Send(context.Background(), p, aliyunTestSendRequest())
			if err != nil {
				t.Fatalf("Send 返回错误: %v", err)
			}
			if res.BizId != "1234567890" || res.RawCode != "OK" || res.RawMsg != "OK" {
				t.Fatalf("回执字段不符: %+v", res)
			}
			if got.method != http.MethodPost || got.path != "/" {
				t.Fatalf("请求形态不符: %s %s", got.method, got.path)
			}
			if got.body != "" {
				t.Fatalf("官方 SDK 的 SendSms 参数在 query、body 为空，实际 body: %q", got.body)
			}
			if v := got.query.Get("TemplateParam"); v != c.wantParam {
				t.Fatalf("TemplateParam = %q, want %q", v, c.wantParam)
			}
			if v := got.query.Get("OutId"); v != "9001" {
				t.Fatalf("OutId = %q, want 9001", v)
			}
			if v := got.query.Get("PhoneNumbers"); v != "8613900001111" {
				t.Fatalf("PhoneNumbers = %q, want 不带 + 的 8613900001111", v)
			}
			if v := got.query.Get("SignName"); v != "安企测试" {
				t.Fatalf("SignName = %q", v)
			}
			if v := got.query.Get("TemplateCode"); v != "SMS_2026111400001" {
				t.Fatalf("TemplateCode = %q", v)
			}
			if v := got.header.Get("x-acs-action"); v != "SendSms" {
				t.Fatalf("x-acs-action = %q", v)
			}
			if v := got.header.Get("x-acs-version"); v != "2017-05-25" {
				t.Fatalf("x-acs-version = %q", v)
			}
			if v := got.header.Get("x-acs-content-sha256"); v != aliyunEmptyBodySHA256 {
				t.Fatalf("x-acs-content-sha256 = %q, want 空 body 哈希", v)
			}
			if _, err := time.Parse("2006-01-02T15:04:05Z", got.header.Get("x-acs-date")); err != nil {
				t.Fatalf("x-acs-date 不是 UTC ISO8601: %q", got.header.Get("x-acs-date"))
			}
			if got.header.Get("x-acs-signature-nonce") == "" {
				t.Fatal("缺少 x-acs-signature-nonce")
			}
			auth := got.header.Get("Authorization")
			if !strings.HasPrefix(auth, "ACS3-HMAC-SHA256 Credential=LTAI5tFakeAdapterAK000,SignedHeaders=") {
				t.Fatalf("Authorization 形态不符: %q", auth)
			}
		})
	}
}

// TestAliyunSenderNonceUnique 连发两次，nonce 必须不同（防重放）。
// 抓它的破坏：把 nonce 写死成常量。
func TestAliyunSenderNonceUnique(t *testing.T) {
	var nonces []string
	srv := httptest.NewTLSServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		nonces = append(nonces, r.Header.Get("x-acs-signature-nonce"))
		w.Header().Set("Content-Type", "application/json")
		fmt.Fprint(w, aliyunOKBody)
	}))
	t.Cleanup(srv.Close)
	sender := &aliyunSender{client: srv.Client()}
	p := aliyunTestProfile(aliyunTestEndpoint(srv))
	for i := 0; i < 2; i++ {
		if _, err := sender.Send(context.Background(), p, aliyunTestSendRequest()); err != nil {
			t.Fatalf("第 %d 次 Send 失败: %v", i+1, err)
		}
	}
	if len(nonces) != 2 || nonces[0] == "" || nonces[0] == nonces[1] {
		t.Fatalf("两次发送的 nonce 必须非空且不同: %v", nonces)
	}
}

// TestAliyunSenderBusinessFailure：HTTP 200 不等于成功。业务码非 OK 必须返回
// *SendError，原始码原样保留（errors.As 落日志），且能 errors.Is 到通用哨兵。
func TestAliyunSenderBusinessFailure(t *testing.T) {
	srv, _ := newAliyunFakeServer(t, http.StatusOK,
		`{"Code":"isv.BUSINESS_LIMIT_CONTROL","Message":"触发业务流控","RequestId":"R-2"}`)
	sender := &aliyunSender{client: srv.Client()}
	res, err := sender.Send(context.Background(), aliyunTestProfile(aliyunTestEndpoint(srv)), aliyunTestSendRequest())
	if err == nil {
		t.Fatal("HTTP 200 但业务码非 OK 必须返回错误")
	}
	var se *SendError
	if !errors.As(err, &se) {
		t.Fatalf("错误不是 *SendError: %v", err)
	}
	if se.Code != "isv.BUSINESS_LIMIT_CONTROL" {
		t.Fatalf("原始码被吞: %q", se.Code)
	}
	if se.Class != ClassPermanent {
		t.Fatalf("isv.BUSINESS_LIMIT_CONTROL 应为 permanent, got %s", se.Class)
	}
	if !strings.Contains(se.Message, "触发业务流控") {
		t.Fatalf("平台原始消息丢失: %q", se.Message)
	}
	if !errors.Is(err, ErrSmsSendFailed) {
		t.Fatal("errors.Is(err, ErrSmsSendFailed) 应为 true")
	}
	if res.BizId != "" {
		t.Fatalf("失败时不应有回执: %+v", res)
	}
}

// TestAliyunSenderNonJSONBody（审查重点 5）：网关插 HTML 错误页 / CDN 拦截时
// HTTP 200 但 body 不是 JSON → 必须判失败，原始 body 片段（截断 200 字节）
// 留进 Message，分类 transient，绝不返回 nil error。
func TestAliyunSenderNonJSONBody(t *testing.T) {
	cases := []struct {
		name string
		body string
	}{
		{"HTML错误页", "<html>502 Bad Gateway</html>"},
		{"超长body截断200字节", strings.Repeat("x", 500)},
		{"JSON但缺Code", "{}"},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			srv, _ := newAliyunFakeServer(t, http.StatusOK, c.body)
			sender := &aliyunSender{client: srv.Client()}
			_, err := sender.Send(context.Background(), aliyunTestProfile(aliyunTestEndpoint(srv)), aliyunTestSendRequest())
			if err == nil {
				t.Fatal("非 JSON / 无 Code 的 200 响应绝不能伪装成功")
			}
			var se *SendError
			if !errors.As(err, &se) {
				t.Fatalf("错误不是 *SendError: %v", err)
			}
			if se.Class != ClassTransient {
				t.Fatalf("应为 transient, got %s", se.Class)
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
		})
	}
}

// aliyunPermanentCases 是规格 §5.6 阿里云行每一类 permanent 错误的逐条用例
// （错误码取自官方 API 错误码文档，2026-11-14 核验）。
// 与生产表 aliyunPermanentCodes 的双向穷举由 TestAliyunPermanentCodeTableExhaustive
// 用 go/ast 锁定，不写死总数。
var aliyunPermanentCases = []struct {
	code     string
	category string
}{
	{"isv.SMS_SIGNATURE_ILLEGAL", "签名不存在/未审核"},
	{"isv.SIGN_STATE_ILLEGAL", "签名状态不可用"},
	{"isv.SMS_SIGN_ILLEGAL", "签名禁止使用"},
	{"isv.SMS_TEMPLATE_ILLEGAL", "模板不存在/未审核"},
	{"InvalidAccessKeyId.NotFound", "AK不存在"},
	{"Forbidden.AccessKeyDisabled", "AK被禁用"},
	{"SignatureDoesNotMatch", "AK Secret错误"},
	{"isp.RAM_PERMISSION_DENY", "RAM无权限"},
	{"isv.ACCOUNT_NOT_EXISTS", "账户不存在"},
	{"isv.PRODUCT_UN_SUBSCRIPT", "未开通短信服务"},
	{"isv.AMOUNT_NOT_ENOUGH", "账户余额不足"},
	{"isv.OUT_OF_SERVICE", "欠费停机"},
	{"isv.MOBILE_NUMBER_ILLEGAL", "号码格式非法"},
	{"isv.BUSINESS_LIMIT_CONTROL", "平台级流控"},
	{"isv.DAY_LIMIT_CONTROL", "日发送限额"},
	{"isv.MONTH_LIMIT_CONTROL", "月发送限额"},
	{"MOBILE_SEND_LIMIT", "单号码发送上限"},
}

// TestAliyunSenderErrorCodeClassification 表驱动覆盖每一类 permanent 错误；
// 未列入表的码 → transient 且 Code 原样保留（排查唯一线索，不得吞成「发送失败」）。
func TestAliyunSenderErrorCodeClassification(t *testing.T) {
	for _, c := range aliyunPermanentCases {
		t.Run(c.code, func(t *testing.T) {
			srv, _ := newAliyunFakeServer(t, http.StatusOK,
				fmt.Sprintf(`{"Code":%q,"Message":"raw message"}`, c.code))
			sender := &aliyunSender{client: srv.Client()}
			_, err := sender.Send(context.Background(), aliyunTestProfile(aliyunTestEndpoint(srv)), aliyunTestSendRequest())
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

	t.Run("未列入表的码原样保留且transient", func(t *testing.T) {
		srv, _ := newAliyunFakeServer(t, http.StatusOK,
			`{"Code":"isv.SOME_FUTURE_UNKNOWN_CODE","Message":"unknown"}`)
		sender := &aliyunSender{client: srv.Client()}
		_, err := sender.Send(context.Background(), aliyunTestProfile(aliyunTestEndpoint(srv)), aliyunTestSendRequest())
		var se *SendError
		if !errors.As(err, &se) {
			t.Fatalf("错误不是 *SendError: %v", err)
		}
		if se.Class != ClassTransient {
			t.Fatalf("未列入表的码应为 transient, got %s", se.Class)
		}
		if se.Code != "isv.SOME_FUTURE_UNKNOWN_CODE" {
			t.Fatalf("未识别的码必须原样保留, got %q", se.Code)
		}
	})

	t.Run("平台消息为空时回落中文提示", func(t *testing.T) {
		srv, _ := newAliyunFakeServer(t, http.StatusOK,
			`{"Code":"isv.AMOUNT_NOT_ENOUGH","Message":""}`)
		sender := &aliyunSender{client: srv.Client()}
		_, err := sender.Send(context.Background(), aliyunTestProfile(aliyunTestEndpoint(srv)), aliyunTestSendRequest())
		var se *SendError
		if !errors.As(err, &se) {
			t.Fatalf("错误不是 *SendError: %v", err)
		}
		if se.Message == "" {
			t.Fatal("平台消息为空时应回落已知错误码的中文提示")
		}
	})
}

// TestAliyunPermanentCodeTableExhaustive 用 go/ast 穷举生产表 aliyunPermanentCodes
// 的键，与测试用例表双向比对：新增错误码漏写测试会红，测试写了表里没有的码也会红。
// 不写死错误码总数（AGENTS.md §6）。
func TestAliyunPermanentCodeTableExhaustive(t *testing.T) {
	fset := token.NewFileSet()
	file, err := parser.ParseFile(fset, "aliyun.go", nil, 0)
	if err != nil {
		t.Fatalf("解析 aliyun.go 失败: %v", err)
	}
	prodCodes := make(map[string]bool)
	ast.Inspect(file, func(n ast.Node) bool {
		spec, ok := n.(*ast.ValueSpec)
		if !ok {
			return true
		}
		for i, name := range spec.Names {
			if name.Name != "aliyunPermanentCodes" || i >= len(spec.Values) {
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
		t.Fatal("未从 aliyun.go 解析到 aliyunPermanentCodes 的任何键：护栏会空转")
	}
	testCodes := make(map[string]bool)
	for _, c := range aliyunPermanentCases {
		testCodes[c.code] = true
	}
	for code := range prodCodes {
		if !testCodes[code] {
			t.Errorf("生产表错误码 %s 没有对应的分类测试用例", code)
		}
	}
	for code := range testCodes {
		if !prodCodes[code] {
			t.Errorf("测试用例 %s 不在生产表 aliyunPermanentCodes 中", code)
		}
	}
}

// TestAliyunSenderBothEndpoints 国内/国际两个 endpoint：请求确实打到对应 host，
// wire 请求与 SDK 向量逐字节同形，且把适配器实际发出的请求用向量的时间/nonce
// 重签后，Authorization 与 SDK 向量逐字节一致（裁决 R29：两个 endpoint 的
// PhoneNumbers 都断言去 + 结果）。
func TestAliyunSenderBothEndpoints(t *testing.T) {
	for _, v := range loadAliyunVectors(t) {
		t.Run(v.Name, func(t *testing.T) {
			srv, got := newAliyunFakeServer(t, http.StatusOK, aliyunOKBody)
			sender := &aliyunSender{client: aliyunRedirectClient(srv)}
			p := Profile{
				Provider:        "aliyun",
				Endpoint:        v.Endpoint,
				AccessKeyId:     v.AccessKeyId,
				AccessKeySecret: v.AccessKeySecret,
				SignName:        v.Input.SignName,
				TemplateId:      v.Input.TemplateId,
				ParamName:       v.Input.ParamName,
			}
			req := SendRequest{To: v.Input.To, Scene: SceneLogin, Code: v.Input.Code, OutId: v.Input.OutId}
			if _, err := sender.Send(context.Background(), p, req); err != nil {
				t.Fatalf("Send 失败: %v", err)
			}
			if got.host != v.Endpoint {
				t.Fatalf("请求未打到对应 host: got %q, want %q", got.host, v.Endpoint)
			}
			if got.method != v.Method || got.path != v.Path {
				t.Fatalf("请求形态不符: %s %s", got.method, got.path)
			}
			if got.rawQuery != v.RawQuery {
				t.Fatalf("wire query 与 SDK 向量不一致\n got: %s\nwant: %s", got.rawQuery, v.RawQuery)
			}
			if want := strings.TrimPrefix(v.Input.To, "+"); got.query.Get("PhoneNumbers") != want {
				t.Fatalf("PhoneNumbers = %q, want 去 + 的 %q", got.query.Get("PhoneNumbers"), want)
			}
			if got.body != v.Body {
				t.Fatalf("body 与向量不一致: %q", got.body)
			}

			// 用适配器实际发出的请求（header/query/body/host）+ 向量的时间/nonce 重签，
			// 必须得到 SDK 的 Authorization。适配器多发/漏发任何参与签名的头都会红。
			u := "https://" + got.host + got.path
			if got.rawQuery != "" {
				u += "?" + got.rawQuery
			}
			rebuilt, err := http.NewRequest(got.method, u, strings.NewReader(got.body))
			if err != nil {
				t.Fatalf("重建请求失败: %v", err)
			}
			rebuilt.Host = got.host
			for k, vals := range got.header {
				for _, val := range vals {
					rebuilt.Header.Add(k, val)
				}
			}
			date, err := time.Parse("2006-01-02T15:04:05Z", v.Headers["x-acs-date"])
			if err != nil {
				t.Fatalf("解析向量时间失败: %v", err)
			}
			if err := SignAliyun(rebuilt, p, date, v.Headers["x-acs-signature-nonce"]); err != nil {
				t.Fatalf("SignAliyun 失败: %v", err)
			}
			if auth := rebuilt.Header.Get("Authorization"); auth != v.Authorization {
				t.Fatalf("适配器请求重签后与 SDK 向量不一致\n got: %s\nwant: %s", auth, v.Authorization)
			}
		})
	}
}

// TestAliyunSenderTimeoutNoRetry 用生产默认 client 的超时行为验证：
// 挂起请求 ~10s 后返回 transient 错误，且服务端只收到一次请求（按条计费，不自动重试）。
func TestAliyunSenderTimeoutNoRetry(t *testing.T) {
	var hits atomic.Int32
	srv := httptest.NewTLSServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		hits.Add(1)
		<-r.Context().Done()
	}))
	t.Cleanup(srv.Close)

	prod := NewAliyunSender().(*aliyunSender)
	client := aliyunRedirectClient(srv)
	client.Timeout = prod.client.Timeout // 保留生产超时，只改路由
	sender := &aliyunSender{client: client}

	start := time.Now()
	_, err := sender.Send(context.Background(), aliyunTestProfile("dysmsapi.aliyuncs.com"), aliyunTestSendRequest())
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
	if elapsed < 9*time.Second || elapsed > 12*time.Second {
		t.Fatalf("发送超时应在 10s 左右, got %v", elapsed)
	}
	time.Sleep(200 * time.Millisecond)
	if n := hits.Load(); n != 1 {
		t.Fatalf("不得自动重试（按条计费）: 服务端收到 %d 次请求", n)
	}
}

// TestAliyunSenderNetworkError 连接失败 → transient 的 *SendError（规格 §10.1）。
func TestAliyunSenderNetworkError(t *testing.T) {
	srv := httptest.NewTLSServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {}))
	endpoint := aliyunTestEndpoint(srv)
	client := srv.Client()
	srv.Close() // 端口不再监听

	sender := &aliyunSender{client: client}
	_, err := sender.Send(context.Background(), aliyunTestProfile(endpoint), aliyunTestSendRequest())
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

// TestAliyunSenderHTTPStatusClassification 非 2xx 按 ClassifyHTTPStatus 分类，
// 平台返回的 Code 原样保留（规格 §10.1）。
func TestAliyunSenderHTTPStatusClassification(t *testing.T) {
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
			srv, _ := newAliyunFakeServer(t, c.status, `{"Code":"Throttling","Message":"raw"}`)
			sender := &aliyunSender{client: srv.Client()}
			_, err := sender.Send(context.Background(), aliyunTestProfile(aliyunTestEndpoint(srv)), aliyunTestSendRequest())
			var se *SendError
			if !errors.As(err, &se) {
				t.Fatalf("错误不是 *SendError: %v", err)
			}
			if se.Class != c.want {
				t.Fatalf("HTTP %d 应为 %s, got %s", c.status, c.want, se.Class)
			}
			if se.Code != "Throttling" {
				t.Fatalf("非 2xx 时平台 Code 也应原样保留, got %q", se.Code)
			}
		})
	}
}

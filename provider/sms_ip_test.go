package provider

import (
	"net/http"
	"net/http/httptest"
	"net/netip"
	"testing"

	"github.com/kataras/iris/v12"
	"kandaoni.com/anqicms/config"
)

func smsTestPrefixes(t *testing.T, items ...string) []netip.Prefix {
	t.Helper()
	return smsParseTrustedProxies(items)
}

// TestSmsResolveClientIP 覆盖 IP 限额取值的三类部署形态。
//
// ①「直连公网」—— 对端公网且不在 trusted：必须忽略一切转发头（否则换个头就能无限发付费短信）。
// ②「本机/内网反代」—— 对端私有或环回：采信转发头，且从 XFF 最右往左剥离。
// ③「CDN」—— 对端是 CDN 回源节点，但必须先在 trusted_proxies 里登记才会被采信；
// 不登记就只能按回源节点算（suspicious 标记用于提示站长）。
//
// 反向对照（人工）：把 smsResolveClientIP 里 `if !peerTrusted { return peer, ... }`
// 这一支改成「无条件取 XFF」，用例① 必红（会返回 1.2.3.4）。
func TestSmsResolveClientIP(t *testing.T) {
	cases := []struct {
		name       string
		peer       string
		xff        string
		xRealIP    string
		cfIP       string
		trusted    []string
		want       string
		suspicious bool
	}{
		{
			// ① 直连：客户端自己塞 X-Real-IP，不能被采信。
			name: "① 公网对端+伪造X-Real-IP取对端", peer: "203.0.113.9:12345",
			xRealIP: "1.2.3.4", want: "203.0.113.9", suspicious: true,
		},
		{
			name: "① 公网对端+伪造XFF取对端", peer: "203.0.113.9:12345",
			xff: "1.2.3.4, 5.6.7.8", want: "203.0.113.9", suspicious: true,
		},
		{
			name: "① 公网对端无头取对端", peer: "203.0.113.9:12345", want: "203.0.113.9",
		},
		{
			// ② 本机反代：nginx 覆盖 X-Real-IP。
			name: "② 私有对端+X-Real-IP取头", peer: "10.0.0.5:12345",
			xRealIP: "1.2.3.4", want: "1.2.3.4",
		},
		{
			name: "② 私有对端+XFF取最右公网", peer: "10.0.0.5:12345",
			xff: "1.2.3.4, 198.51.100.7", want: "198.51.100.7",
		},
		{
			name: "② 环回对端无头取对端", peer: "127.0.0.1:12345", want: "127.0.0.1",
		},
		{
			// ③ CDN 已登记：客户端在左边追加伪造值，最右才是 CDN 追加的真实地址。
			name: "③ CDN已配置+左侧伪造值不影响结果", peer: "203.0.113.9:12345",
			xff: "1.1.1.1, 2.2.2.2, 198.51.100.7", trusted: []string{"203.0.113.0/24"},
			want: "198.51.100.7",
		},
		{
			name: "③ CDN已配置+单跳XFF", peer: "203.0.113.9:12345",
			xff: "198.51.100.7", trusted: []string{"203.0.113.0/24"}, want: "198.51.100.7",
		},
		{
			// CDN 自己也会出现在 XFF 右侧（多级 CDN），要一并跳过。
			name: "③ CDN已配置+XFF含另一CDN节点", peer: "203.0.113.9:12345",
			xff: "198.51.100.7, 203.0.113.20", trusted: []string{"203.0.113.0/24"},
			want: "198.51.100.7",
		},
		{
			name: "③ CDN已配置+XFF空回落X-Real-IP", peer: "203.0.113.9:12345",
			xRealIP: "198.51.100.7", trusted: []string{"203.0.113.0/24"}, want: "198.51.100.7",
		},
		{
			name: "③ CDN已配置+XFF全私有回落CF头", peer: "203.0.113.9:12345",
			xff: "10.0.0.9, 127.0.0.1", cfIP: "198.51.100.7",
			trusted: []string{"203.0.113.0/24"}, want: "198.51.100.7",
		},
		{
			// 精确单 IP 也要能当 trusted（站长只知道一个回源出口时最常用）。
			name: "③ trusted用单IP精确匹配", peer: "203.0.113.9:12345",
			xff: "198.51.100.7", trusted: []string{"203.0.113.9"}, want: "198.51.100.7",
		},
		{
			name: "③ IPv6 CDN+CIDR", peer: "[2001:db8::9]:443",
			xff: "2001:4860:4860::8888", trusted: []string{"2001:db8::/32"},
			want: "2001:4860:4860::8888",
		},
		{
			// 关键回归：对端是公网但不在 trusted（= 站长忘了登记 CDN 网段）→
			// 仍然取对端，并置 suspicious 让上层告警。绝不能因为「像 CDN」就采信头。
			name: "③ CDN未登记仍取对端并告警", peer: "203.0.113.9:12345",
			xff: "198.51.100.7", trusted: []string{"192.0.2.0/24"},
			want: "203.0.113.9", suspicious: true,
		},
		{
			name: "④ XFF含非法串不panic", peer: "10.0.0.5:12345",
			xff: "unknown, , 198.51.100.7", want: "198.51.100.7",
		},
		{
			name: "④ 公网IPv6无端口不panic", peer: "2001:4860:4860::8888", want: "2001:4860:4860::8888",
		},
		{
			name: "④ 空RemoteAddr不panic", peer: "", want: "",
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			got, suspicious := smsResolveClientIP(
				tc.peer, tc.xff, tc.xRealIP, tc.cfIP, smsTestPrefixes(t, tc.trusted...))
			if got != tc.want {
				t.Fatalf("clientIP = %q，期望 %q", got, tc.want)
			}
			if suspicious != tc.suspicious {
				t.Fatalf("suspicious = %v，期望 %v", suspicious, tc.suspicious)
			}
		})
	}
}

// TestSmsParseTrustedProxies 保证脏配置不会让发码打挂：非法项跳过、合法项照常生效。
func TestSmsParseTrustedProxies(t *testing.T) {
	got := smsParseTrustedProxies([]string{"  203.0.113.0/24 ", "", "not-an-ip", "203.0.113.9", "2001:db8::/32"})
	if len(got) != 3 {
		t.Fatalf("解析出 %d 条，期望 3 条（跳过空串与非法项）：%v", len(got), got)
	}
	if !smsInPrefixes(netip.MustParseAddr("203.0.113.77"), got) {
		t.Fatalf("203.0.113.77 应命中 203.0.113.0/24")
	}
	if !smsInPrefixes(netip.MustParseAddr("203.0.113.9"), got) {
		t.Fatalf("203.0.113.9 应命中单 IP 形式")
	}
	if smsInPrefixes(netip.MustParseAddr("198.51.100.7"), got) {
		t.Fatalf("198.51.100.7 不应命中任何网段")
	}
}

// TestSmsClientIPEndToEnd 走真实 iris 上下文，验证：
//   - 站点未配置 trusted_proxies 时，公网对端的伪造头一律不采信；
//   - 站点配置了 trusted_proxies 时，才按 XFF 最右取值；
//   - 并确认反向对照的前提成立：iris 的 ctx.RemoteAddr() 确实会采信该头。
//
// 反向对照（人工）：删掉 smsResolveClientIP 的 `!peerTrusted` 分支后，
// 第一条断言会拿到 1.2.3.4，本用例必红。
func TestSmsClientIPEndToEnd(t *testing.T) {
	newApp := func() *iris.Application {
		app := iris.New().Configure(iris.WithRemoteAddrHeader("X-Real-IP"))
		return app
	}

	t.Run("未配置 trusted_proxies 时取对端", func(t *testing.T) {
		var got, proxyOnly string
		app := newApp()
		app.Get("/x", func(ctx iris.Context) {
			got = SmsClientIP(ctx, &Website{})
			proxyOnly = ctx.RemoteAddr()
		})
		if err := app.Build(); err != nil {
			t.Fatalf("构建测试应用失败：%v", err)
		}
		req := httptest.NewRequest(http.MethodGet, "/x", nil)
		req.RemoteAddr = "203.0.113.9:12345"
		req.Header.Set("X-Real-IP", "1.2.3.4")
		app.ServeHTTP(httptest.NewRecorder(), req)

		if got != "203.0.113.9" {
			t.Fatalf("SmsClientIP = %q，期望对端 203.0.113.9（未登记代理时不得采信 X-Real-IP）", got)
		}
		if proxyOnly != "1.2.3.4" {
			t.Fatalf("反向对照失效：ctx.RemoteAddr() = %q，期望伪造头 1.2.3.4（否则本用例无法区分两种实现）", proxyOnly)
		}
	})

	t.Run("配置了 CDN 网段时取 XFF 最右", func(t *testing.T) {
		var got string
		app := newApp()
		app.Get("/x", func(ctx iris.Context) {
			got = SmsClientIP(ctx, &Website{PluginSms: &config.PluginSmsConfig{
				TrustedProxies: []string{"203.0.113.0/24"},
			}})
		})
		if err := app.Build(); err != nil {
			t.Fatalf("构建测试应用失败：%v", err)
		}
		req := httptest.NewRequest(http.MethodGet, "/x", nil)
		req.RemoteAddr = "203.0.113.9:12345"
		// 左侧是客户端伪造值，最右是 CDN 追加的真实地址。
		req.Header.Set("X-Forwarded-For", "6.6.6.6, 198.51.100.7")
		req.Header.Set("X-Real-IP", "7.7.7.7")
		app.ServeHTTP(httptest.NewRecorder(), req)

		if got != "198.51.100.7" {
			t.Fatalf("SmsClientIP = %q，期望 198.51.100.7（XFF 最右的真实客户端）", got)
		}
	})
}

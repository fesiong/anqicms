package provider

import (
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/kataras/iris/v12"
)

// TestSmsIpClientIP 表驱动覆盖 SmsClientIP 的四种关键形态：
//
//	① 公网对端 + 伪造 X-Real-IP → 取对端（直连公网，头一定是客户端自己写的）；
//	② 私有对端 + X-Real-IP     → 取头（前面有本机/内网反代）；
//	③ 环回对端 + 无头           → 取对端；
//	④ RemoteAddr 不带端口 / IPv6 形式不 panic。
//
// 反向对照（人工）：删掉 SmsClientIP 里「公网对端直接返回 peer」的分支后，
// ① 会退化成 ctx.RemoteAddr()（即伪造头），本用例必红。
func TestSmsIpClientIP(t *testing.T) {
	cases := []struct {
		name       string
		remoteAddr string
		xRealIP    string
		want       string
	}{
		{
			name:       "① 公网对端+伪造X-Real-IP取对端",
			remoteAddr: "203.0.113.9:12345",
			xRealIP:    "1.2.3.4",
			want:       "203.0.113.9",
		},
		{
			name:       "② 私有对端+X-Real-IP取头",
			remoteAddr: "10.0.0.5:12345",
			xRealIP:    "1.2.3.4",
			want:       "1.2.3.4",
		},
		{
			name:       "③ 环回对端无头取对端",
			remoteAddr: "127.0.0.1:12345",
			want:       "127.0.0.1",
		},
		{
			name:       "④ 公网IPv6无端口不panic",
			remoteAddr: "2001:4860:4860::8888",
			want:       "2001:4860:4860::8888",
		},
		{
			name:       "④ 公网IPv6带端口去端口",
			remoteAddr: "[2001:4860:4860::8888]:443",
			want:       "2001:4860:4860::8888",
		},
		{
			name:       "④ 环回无端口不panic",
			remoteAddr: "127.0.0.1",
			want:       "127.0.0.1",
		},
		{
			name:       "④ 空RemoteAddr不panic",
			remoteAddr: "",
			want:       "",
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			var got, proxyOnly string
			app := iris.New().Configure(iris.WithRemoteAddrHeader("X-Real-IP"))
			app.Get("/x", func(ctx iris.Context) {
				got = SmsClientIP(ctx)
				// 模拟「删掉公网对端分支」后的实现：无条件采信 iris 的代理头解析。
				proxyOnly = ctx.RemoteAddr()
			})
			if err := app.Build(); err != nil {
				t.Fatalf("构建测试应用失败：%v", err)
			}
			req := httptest.NewRequest(http.MethodGet, "/x", nil)
			req.RemoteAddr = tc.remoteAddr
			if tc.xRealIP != "" {
				req.Header.Set("X-Real-IP", tc.xRealIP)
			}
			app.ServeHTTP(httptest.NewRecorder(), req)

			if got != tc.want {
				t.Fatalf("SmsClientIP = %q，期望 %q", got, tc.want)
			}
			// 反向对照：用例① 必须能区分「忽略头」与「无条件信头」两种实现。
			if tc.name == "① 公网对端+伪造X-Real-IP取对端" && proxyOnly != tc.xRealIP {
				t.Fatalf("反向对照失效：ctx.RemoteAddr() = %q，期望伪造头 %q（否则本用例无法区分两种实现）",
					proxyOnly, tc.xRealIP)
			}
		})
	}
}

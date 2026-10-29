package middleware

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/kataras/iris/v12"
	"kandaoni.com/anqicms/config"
	"kandaoni.com/anqicms/library"
	"kandaoni.com/anqicms/provider"
)

// 中间件取 IP 用的是 ctx.RemoteAddr()。启动时 bootstrap 配了
// iris.WithRemoteAddrHeader(X-Real-IP, X-Forwarded-For, CF-Connecting-IP)，
// 所以反代后拿到的是真实客户端 IP，不是 127.0.0.1——否则限额会变成全站共享，
// 一个攻击者就能把正常用户全挤出去。这条断言锁的就是这个假设。
func TestAccountProbeGuardLimitsPerClientIP(t *testing.T) {
	restoreLimiter := swapAccountProbeLimiter(library.NewRateLimiter(2, time.Minute))
	defer restoreLimiter()

	app := newProbeTestApp(t)

	do := func(ip string) (int, map[string]interface{}) {
		req := httptest.NewRequest(http.MethodGet, "/probe", nil)
		req.Header.Set("X-Forwarded-For", ip)
		rec := httptest.NewRecorder()
		app.ServeHTTP(rec, req)

		var body map[string]interface{}
		_ = json.Unmarshal(rec.Body.Bytes(), &body)
		return rec.Code, body
	}

	// 同一 IP 前两次放行，第三次拒绝。
	for i := 1; i <= 2; i++ {
		if code, _ := do("203.0.113.10"); code != http.StatusOK {
			t.Fatalf("第 %d 次请求应放行，实际 HTTP %d", i, code)
		}
	}
	code, body := do("203.0.113.10")
	if code != http.StatusTooManyRequests {
		t.Fatalf("超限后应返回 429，实际 %d", code)
	}
	if got, _ := body["code"].(float64); got == 0 {
		t.Fatalf("超限响应必须是非 0 业务码（前端按 code 判断成败），实际 %v", body)
	}
	// 用户看到的是这句话：必须是翻译后的文案，不能把语言键原样吐出去
	// （TplTr 找不到键时会退回 fmt.Sprintf(str)，即返回 "TooManyRequests"）。
	if msg, _ := body["msg"].(string); msg == "" || msg == "TooManyRequests" {
		t.Fatalf("超限文案未翻译（msg=%q），说明 locales 里缺 TooManyRequests 键", msg)
	}

	// 另一个 IP 有独立额度。若这里被拒，说明 IP 没取到（退化成了固定 key），
	// 线上就是「一个人把全站额度耗尽」。
	if code, _ := do("198.51.100.20"); code != http.StatusOK {
		t.Fatalf("不同 IP 应有独立额度，实际 HTTP %d（IP 提取可能失效）", code)
	}
}

func swapAccountProbeLimiter(l *library.RateLimiter) func() {
	old := accountProbeLimiter
	accountProbeLimiter = l
	return func() { accountProbeLimiter = old }
}

func newProbeTestApp(t *testing.T) *iris.Application {
	t.Helper()

	app := iris.New()
	// 与 bootstrap.go 一致：先加载 locales，再注入 provider，否则
	// CurrentSite(ctx) 里的 ctx.GetLocale() 会直接空指针 panic。
	if err := app.I18n.Load("../locales/*/*.yml", config.LoadLocales()...); err != nil {
		t.Fatalf("加载 locales 失败：%v", err)
	}
	app.I18n.SetDefault("zh-CN")
	provider.SetI18n(app.I18n)
	t.Cleanup(func() { provider.SetI18n(nil) })

	app.Configure(iris.WithRemoteAddrHeader("X-Real-IP", "X-Forwarded-For", "CF-Connecting-IP"))
	app.Get("/probe", AccountProbeGuard, func(ctx iris.Context) {
		ctx.JSON(iris.Map{"code": 0})
	})
	if err := app.Build(); err != nil {
		t.Fatalf("构建测试应用失败：%v", err)
	}
	return app
}

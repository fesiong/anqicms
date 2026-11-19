package middleware

// 启动闸门的护栏：初始化没完成时一律 503，且 /__startup 必须能报进度。
//
// 事故背景见 provider/startup.go。这里守的是一条容易被"顺手优化掉"的边界：
// 未就绪时不能让任何业务 handler 跑起来（那时站点注册表是空的或只注册了一部分）。

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/kataras/iris/v12"
	"kandaoni.com/anqicms/provider"
)

func newStartupGateApp() *iris.Application {
	app := iris.New()
	// 生产里闸门是第一个中间件（bootstrap.loadGlobalMiddleware）。
	app.Use(StartupGate)
	app.Get("/hello", func(ctx iris.Context) {
		_, _ = ctx.WriteString("ok")
	})
	// 真实路由由 route.Register 注册；闸门会先把它接管掉，这条只是兜底。
	app.Get(provider.StartupStatusPath, StartupStatus)
	if err := app.Build(); err != nil {
		panic(err)
	}
	return app
}

func doRequest(app *iris.Application, path string, accept string) *httptest.ResponseRecorder {
	req := httptest.NewRequest(http.MethodGet, path, nil)
	if accept != "" {
		req.Header.Set("Accept", accept)
	}
	rec := httptest.NewRecorder()
	app.ServeHTTP(rec, req)
	return rec
}

func TestStartupGateBlocksBusinessUntilReady(t *testing.T) {
	app := newStartupGateApp()

	// 收尾：把阶段落定成 ready，避免影响同包其它用例（它们都走闸门）。
	defer provider.FinishStartup(false)

	provider.BeginStartup()

	// 1) 业务请求：503 + Retry-After，且不能带出 "ok"
	rec := doRequest(app, "/hello", "")
	if rec.Code != http.StatusServiceUnavailable {
		t.Fatalf("未就绪时业务请求应 503，实际 %d（body=%s）", rec.Code, rec.Body.String())
	}
	if rec.Header().Get("Retry-After") == "" {
		t.Fatal("503 必须带 Retry-After，否则浏览器/负载均衡会当成硬故障")
	}
	if rec.Body.String() == "ok" {
		t.Fatal("未就绪时 handler 不能执行：那一刻站点注册表是空的")
	}

	// 2) 状态端点：503（就绪探针语义）+ 可读的进度 JSON
	rec = doRequest(app, provider.StartupStatusPath, "")
	if rec.Code != http.StatusServiceUnavailable {
		t.Fatalf("未就绪时 /__startup 应 503（当就绪探针用），实际 %d", rec.Code)
	}
	var snap provider.StartupSnapshot
	if err := json.Unmarshal(rec.Body.Bytes(), &snap); err != nil {
		t.Fatalf("/__startup 不是合法 JSON：%v（body=%s）", err, rec.Body.String())
	}
	if snap.Phase != "running" || snap.Ready {
		t.Fatalf("状态端点应报 running/未就绪：%+v", snap)
	}

	// 3) 前端 XHR/Accept: application/json 也要拿到 JSON 而不是 HTML 页
	rec = doRequest(app, "/hello", "application/json")
	if rec.Code != http.StatusServiceUnavailable {
		t.Fatalf("JSON 请求应 503，实际 %d", rec.Code)
	}
	if err := json.Unmarshal(rec.Body.Bytes(), &snap); err != nil {
		t.Fatalf("Accept: application/json 应返回 JSON：%v（body=%s）", err, rec.Body.String())
	}

	// 4) 就绪后：业务请求放行，状态端点 200
	provider.FinishStartup(false)
	rec = doRequest(app, "/hello", "")
	if rec.Code != http.StatusOK || rec.Body.String() != "ok" {
		t.Fatalf("就绪后应放行，实际 %d body=%q", rec.Code, rec.Body.String())
	}
	rec = doRequest(app, provider.StartupStatusPath, "")
	if rec.Code != http.StatusOK {
		t.Fatalf("就绪后 /__startup 应 200，实际 %d", rec.Code)
	}
}

func TestStartupGateAllowsFreshInstall(t *testing.T) {
	// 没有默认库（全新安装）时阶段停在 idle，闸门必须放行 /install，
	// 否则第一次装站的人只会看到 503。
	app := newStartupGateApp()
	rec := doRequest(app, "/hello", "")
	if rec.Code != http.StatusOK {
		t.Fatalf("idle 阶段（全新安装）必须放行，实际 %d", rec.Code)
	}
}

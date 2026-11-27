package provider

// 静态缓存构建拿不到分页数的回归护栏。
//
// 2026-11-19 现场：后台触发「生成栏目页」后，缓存目录里每页只有 index.html，
// 第二页往后一个都没有，而且没有任何日志。原因是后台 handler 用
// `go w2.BuildCategoryCache(ctx)` 把请求的 iris ctx 带进了 goroutine，
// handler 一返回请求 context 就被 cancel；view.DjangoEngine.ExecuteWriter
// 第一步检查 ctx.Request().Context().Err()，直接返回 context.Canceled，
// 模板一个字都没渲染 —— 而分页数正是「渲染一次模板」的副作用（webInfo.TotalPages），
// 于是恒为 0，分页循环整段跳过。
//
// 反向对照：同一个已取消的父 context，直接 ctx.Clone() 必须仍然是 canceled，
// 否则说明本用例没能复现取消状态，护栏就是空转。

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/kataras/iris/v12"
)

func TestNewCacheRenderContextDetachesRequestContext(t *testing.T) {
	app := iris.New()

	var inheritedErr, detachedErr error
	app.Get("/probe", func(ctx iris.Context) {
		parent, cancel := context.WithCancel(ctx.Request().Context())
		ctx.ResetRequest(ctx.Request().WithContext(parent))
		// 模拟 handler 已经返回：请求 context 被取消。
		cancel()

		inheritedErr = ctx.Clone().Request().Context().Err()
		detachedErr = newCacheRenderContext(ctx).Request().Context().Err()

		_, _ = ctx.WriteString("ok")
	})

	if err := app.Build(); err != nil {
		t.Fatalf("构建测试应用失败: %v", err)
	}
	srv := httptest.NewServer(app)
	defer srv.Close()

	resp, err := http.Get(srv.URL + "/probe")
	if err != nil {
		t.Fatalf("请求探针路由失败: %v", err)
	}
	defer resp.Body.Close()

	// 反向对照：不带脱钩的 Clone 必须还能看到 cancel，证明取消状态真实存在。
	if inheritedErr == nil {
		t.Fatal("反向对照失败：ctx.Clone() 未继承 canceled，本用例没有复现问题现场")
	}
	if detachedErr != nil {
		t.Fatalf("newCacheRenderContext 未脱钩请求 context，仍为 %v", detachedErr)
	}
}

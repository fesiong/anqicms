package controller

import (
	"go/ast"
	"go/parser"
	"go/token"
	"io"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/kataras/iris/v12"
)

// TestFrontHTMLResponseHasContentLength 锁定前台 HTML 页面必须带 Content-Length。
//
// 背景（2026-11-19）：`/manual-other/3669.html` 只回 Transfer-Encoding: chunked。
// 原因是两条叠加：
//  1. 页面为了写 HTML 缓存先 `ctx.Recorder()`，而 iris v12.2.11 的
//     ResponseRecorder.FlushResponse 是「先 WriteHeader 再 Write(body)」，
//     头一发出 net/http 再也补不了长度；
//  2. body 普遍 10k~25k，超过 net/http 的 2048 字节缓冲阈值，就算不录制也会 chunked。
//
// 所以 renderView / writeCachedHTML 必须在头发出前显式回填长度。
// 本用例用 httptest.NewServer（真起服务）而不是进程内 ServeHTTP，
// 因为 chunked 是 net/http 真实写出响应时的行为，httptest.ResponseRecorder 看不到。
func TestFrontHTMLResponseHasContentLength(t *testing.T) {
	body := "<!DOCTYPE html><html><head><title>t</title></head><body>" +
		strings.Repeat("<p>content-length probe 内容长度探针</p>", 120) +
		"</body></html>"
	// 必须超过 net/http 的 bufferBeforeChunkingSize(2048)，否则旧写法也可能自动带上长度，
	// 反向对照就失去意义（断言会假绿）。
	if len(body) <= 2048 {
		t.Fatalf("探针 body 太短(%d)，无法触发 chunked 分支", len(body))
	}

	viewDir := t.TempDir()
	if err := os.WriteFile(filepath.Join(viewDir, "probe.html"), []byte(body), 0o644); err != nil {
		t.Fatalf("写探针模板失败: %v", err)
	}

	app := iris.New()
	app.RegisterView(iris.HTML(viewDir, ".html"))
	// 修复后的两个出口。
	app.HandleMany("GET HEAD", "/render", func(ctx iris.Context) {
		if err := renderView(ctx, "probe.html"); err != nil {
			t.Errorf("renderView 失败: %v", err)
		}
	})
	app.Get("/cached", func(ctx iris.Context) {
		writeCachedHTML(ctx, []byte(body))
	})
	// 反向对照：旧写法（取 recorder 后直接 ctx.View），必须仍然是 chunked。
	// 若哪天 iris 改了 FlushResponse 顺序导致旧写法也带长度，这条会红，
	// 提醒我们重新评估这个 helper 是否还必要。
	app.Get("/legacy", func(ctx iris.Context) {
		ctx.Recorder()
		_ = ctx.View("probe.html")
	})

	// iris 的 router 必须先 Build 才能被 http.Handler 使用（否则 ServeHTTP 里 nil 解引用）。
	if err := app.Build(); err != nil {
		t.Fatalf("Build 失败: %v", err)
	}

	server := httptest.NewServer(app)
	defer server.Close()

	fetch := func(method, path string) (*http.Response, string) {
		t.Helper()
		req, err := http.NewRequest(method, server.URL+path, nil)
		if err != nil {
			t.Fatalf("构造请求失败: %v", err)
		}
		resp, err := http.DefaultClient.Do(req)
		if err != nil {
			t.Fatalf("%s %s 请求失败: %v", method, path, err)
		}
		defer resp.Body.Close()
		data, _ := io.ReadAll(resp.Body)
		return resp, string(data)
	}

	t.Run("renderView", func(t *testing.T) {
		resp, got := fetch(http.MethodGet, "/render")
		if resp.StatusCode != http.StatusOK {
			t.Fatalf("状态码 %d", resp.StatusCode)
		}
		if got != body {
			t.Fatalf("body 不一致: got %d bytes, want %d", len(got), len(body))
		}
		if resp.ContentLength != int64(len(body)) {
			t.Errorf("Content-Length=%d, 期望 %d（header=%q）", resp.ContentLength, len(body), resp.Header.Get("Content-Length"))
		}
		if len(resp.TransferEncoding) != 0 {
			t.Errorf("不应出现 Transfer-Encoding: %v", resp.TransferEncoding)
		}
	})

	t.Run("writeCachedHTML", func(t *testing.T) {
		resp, got := fetch(http.MethodGet, "/cached")
		if got != body {
			t.Fatalf("body 不一致: got %d bytes, want %d", len(got), len(body))
		}
		if resp.ContentLength != int64(len(body)) {
			t.Errorf("Content-Length=%d, 期望 %d", resp.ContentLength, len(body))
		}
		if len(resp.TransferEncoding) != 0 {
			t.Errorf("不应出现 Transfer-Encoding: %v", resp.TransferEncoding)
		}
	})

	// HEAD 必须同样给出长度（探活 / CDN 回源只看头）。
	t.Run("renderViewHEAD", func(t *testing.T) {
		resp, got := fetch(http.MethodHead, "/render")
		if resp.StatusCode != http.StatusOK {
			t.Fatalf("HEAD 状态码 %d", resp.StatusCode)
		}
		if got != "" {
			t.Errorf("HEAD 不应有 body，实际 %d bytes", len(got))
		}
		if resp.ContentLength != int64(len(body)) {
			t.Errorf("HEAD Content-Length=%d, 期望 %d", resp.ContentLength, len(body))
		}
	})

	// 反向对照：证明这条断言抓得住「没回填长度」的写法。
	t.Run("legacy写法必须是chunked", func(t *testing.T) {
		resp, _ := fetch(http.MethodGet, "/legacy")
		if resp.ContentLength != -1 {
			t.Fatalf("旧写法竟然带了 Content-Length=%d，说明对照失效（iris 行为已变），请重新评估 helper", resp.ContentLength)
		}
		if len(resp.TransferEncoding) == 0 || resp.TransferEncoding[0] != "chunked" {
			t.Fatalf("旧写法应为 chunked，实际 %v", resp.TransferEncoding)
		}
	})
}

// bareCtxViewCalls 用 go/ast 找出源码里直接调用 `ctx.View(...)` 的行号。
//
// 用 AST 而不是字符串匹配：注释里提到 ctx.View、或者 `ctx.Application().View(...)`
// 都不应被误判（后者是缓存生成侧的合法入口）。
func bareCtxViewCalls(filename string, src []byte) ([]int, error) {
	fset := token.NewFileSet()
	file, err := parser.ParseFile(fset, filename, src, 0)
	if err != nil {
		return nil, err
	}
	var lines []int
	ast.Inspect(file, func(n ast.Node) bool {
		call, ok := n.(*ast.CallExpr)
		if !ok {
			return true
		}
		sel, ok := call.Fun.(*ast.SelectorExpr)
		if !ok {
			return true
		}
		recv, ok := sel.X.(*ast.Ident)
		if !ok || recv.Name != "ctx" || sel.Sel == nil || sel.Sel.Name != "View" {
			return true
		}
		lines = append(lines, fset.Position(call.Pos()).Line)
		return true
	})
	return lines, nil
}

// TestNoBareCtxViewInFrontControllers 锁定前台 handler 不得绕过 renderView 直接渲染。
//
// 绕过 renderView 同样能出页面，但拿不到 body 长度 → 又退回 chunked，
// 而且不会有任何功能测试变红（页面照样能打开）。所以只能用源码穷举兜住。
// htmlResponse.go 是 helper 自身，唯一允许出现 ctx.View 的地方。
func TestNoBareCtxViewInFrontControllers(t *testing.T) {
	files, err := filepath.Glob("*.go")
	if err != nil {
		t.Fatalf("列举源码失败: %v", err)
	}
	if len(files) == 0 {
		t.Fatal("没有扫到任何源码文件，断言失效")
	}

	scanned := 0
	for _, name := range files {
		if strings.HasSuffix(name, "_test.go") || name == "htmlResponse.go" {
			continue
		}
		src, err := os.ReadFile(name)
		if err != nil {
			t.Fatalf("读取 %s 失败: %v", name, err)
		}
		lines, err := bareCtxViewCalls(name, src)
		if err != nil {
			t.Fatalf("解析 %s 失败: %v", name, err)
		}
		scanned++
		for _, line := range lines {
			t.Errorf("%s:%d 直接调用了 ctx.View，前台页面必须走 renderView（否则没有 Content-Length）", name, line)
		}
	}
	if scanned == 0 {
		t.Fatal("一个文件都没扫，断言失效")
	}

	// 反向对照：检查器必须抓得住违规写法，同时不误伤合法写法。
	violating := []byte("package x\n\nfunc f(ctx iris.Context) { ctx.View(\"a.html\") }\n")
	lines, err := bareCtxViewCalls("synthetic.go", violating)
	if err != nil {
		t.Fatalf("解析合成源码失败: %v", err)
	}
	if len(lines) != 1 {
		t.Fatalf("检查器抓不住裸 ctx.View（找到 %d 处），上面的断言等于空转: %v", len(lines), lines)
	}
	legal := []byte("package x\n\nfunc f(ctx iris.Context) { _ = renderView(ctx, \"a.html\"); _ = ctx.Application().View(ctx, \"a.html\", \"\", nil) }\n")
	if lines, err = bareCtxViewCalls("synthetic.go", legal); err != nil || len(lines) != 0 {
		t.Fatalf("检查器误伤合法写法（err=%v lines=%v）", err, lines)
	}
}

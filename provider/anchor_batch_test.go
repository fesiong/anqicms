package provider

import (
	"go/ast"
	"go/parser"
	"go/token"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/kataras/iris/v12"
	"kandaoni.com/anqicms/config"
	"kandaoni.com/anqicms/model"
)

// TestAnchorQuotaNotEatenByExistingLinks 锁定「文内原有的链接不能占用锚文本配额」。
//
// 背景：replaceContentTextWithHits 用 existsKeywords 判「是否达到密度上限」，
// 而 existsKeywords 还预填了正文里原有的 <a> 文本与 markdown 链接文本。
// 结果是：一篇文章自带的目录、外链把配额先吃满 → 第一次替换前
// len(existsKeywords) >= maxAnchorNum 就成立 → 整篇一个锚文本都加不上。
// 表现出来就是「点了批量更新，链接多的那些文章完全没变化」。
func TestAnchorQuotaNotEatenByExistingLinks(t *testing.T) {
	w := testAnchorSite(t)
	// 密度 500：约 1000 字的正文只允许 2 个锚文本，方便构造「配额很小」的场景。
	w.PluginAnchor.AnchorDensity = 500
	// 不去重加粗，避免第 3 次命中被加粗干扰计数
	w.PluginAnchor.NoStrongTag = 1

	anchors := []*model.Anchor{
		{Id: 301, Title: "锚文本甲", Link: "/a"},
		{Id: 302, Title: "锚文本乙", Link: "/b"},
	}

	// 开头一行放两个关键词，后面放 3 个「本来就有」的 markdown 链接。
	content := "开头段落里提到了锚文本甲，之后又说到了锚文本乙。\n\n" +
		"[既有链接一](/x1) 和 [既有链接二](/x2) 以及 [既有链接三](/x3)\n\n" +
		strings.Repeat("这里是一段很长的普通说明文字，用来抬高正文总字数。", 40)

	_, hits, modified := w.replaceContentTextWithHits(anchors, content, "/doc")
	if !modified {
		t.Fatal("文内有 3 个既有链接、配额只有 2，导致一个锚文本都没加：配额被文内原有链接吃光了")
	}
	if len(hits) != 2 {
		t.Fatalf("期望加 2 个锚文本（密度上限），实际 %d 个：%v", len(hits), hits)
	}
}

// TestAnchorDensityDefaultFollowsConfig 锁定密度默认值口径一致：
// config.PluginAnchorConfig 注释写「小于10时默认为100」，后台设置页也是 <10→100，
// 替换逻辑这里曾经写成 <20→200 —— 同样的文章只能拿到一半的锚文本。
func TestAnchorDensityDefaultFollowsConfig(t *testing.T) {
	w := testAnchorSite(t)
	w.PluginAnchor.AnchorDensity = 0 // 未设置
	w.PluginAnchor.NoStrongTag = 1

	anchors := []*model.Anchor{
		{Id: 401, Title: "锚文本丙", Link: "/c"},
		{Id: 402, Title: "锚文本丁", Link: "/d"},
	}
	// 约 200 字：density=100 → 上限 2；density=200（旧默认）→ 上限 1
	content := "这里提到了锚文本丙，然后又提到了锚文本丁。" +
		strings.Repeat("用来凑字数的普通句子，不含任何关键词。", 10)

	_, hits, _ := w.replaceContentTextWithHits(anchors, content, "/doc")
	if len(hits) != 2 {
		t.Fatalf("默认密度应等价于 100（此正文可放 2 个锚文本），实际命中 %d 个", len(hits))
	}
	// 不能把「默认值修正」写回站点配置
	if w.PluginAnchor.AnchorDensity != 0 {
		t.Fatalf("替换逻辑不应就地改写站点配置，AnchorDensity 被改成了 %d", w.PluginAnchor.AnchorDensity)
	}
}

// TestReplaceAnchorRunsDetachedFromRequestContext 复现并锁死本次的核心 bug：
// 后台 handler 用 `go currentSite.ReplaceAnchor(...)` 起异步任务，currentSite 是
// 挂了**请求 ctx** 的副本；handler 一返回 ctx 立刻 canceled，之后所有
// `w.DB.WithContext(w.Ctx())`（GetArchiveDataById 就是）全部失败，
// 任务静默跑一半 —— 用户看到的就是「批量更新只生效了一小部分」。
//
// 这里用真实 HTTP 服务器 + 真实请求复现 ctx 生命周期，再验证 Async() 能救回来。
func TestReplaceAnchorRunsDetachedFromRequestContext(t *testing.T) {
	if _, err := GetDBWebsiteInfo(1); err != nil {
		t.Skipf("跳过：无法读取站点 1（%v）", err)
	}

	var attachedCtxErr error // 请求返回后，挂着 ctx 的副本查库会怎样
	var detachedCtxErr error // 摘掉 ctx 之后查库会怎样
	var wg sync.WaitGroup
	proceed := make(chan struct{})

	app := iris.New()
	if err := app.I18n.Load(config.ExecPath+"locales/*/*.yml", config.LoadLocales()...); err != nil {
		t.Logf("i18n 加载失败（不影响断言）：%v", err)
	}
	app.Get("/probe", func(ctx iris.Context) {
		site := CurrentSite(ctx)
		wg.Add(1)
		go func() {
			defer wg.Done()
			// 等 handler 返回、请求 ctx 被 cancel 之后再查库
			<-proceed
			var one struct{ Id int64 }
			attachedCtxErr = site.DB.WithContext(site.Ctx()).Raw("select id from archives limit 1").Scan(&one).Error
			asyncSite := site.Async()
			detachedCtxErr = asyncSite.DB.WithContext(asyncSite.Ctx()).Raw("select id from archives limit 1").Scan(&one).Error
		}()
		ctx.WriteString("ok")
	})
	if err := app.Build(); err != nil {
		t.Fatalf("Build 失败：%v", err)
	}
	ts := httptest.NewServer(app)
	defer ts.Close()

	resp, err := http.Get(ts.URL + "/probe")
	if err != nil {
		t.Fatalf("请求失败：%v", err)
	}
	resp.Body.Close()
	// 请求已返回，等待 ctx 被 cancel
	time.Sleep(200 * time.Millisecond)
	close(proceed)
	wg.Wait()

	if attachedCtxErr == nil {
		t.Fatalf("预期挂着请求 ctx 的异步任务查库失败（context canceled），实际成功 —— 前置事实变了，请重新评估 Async 的必要性")
	}
	if !strings.Contains(attachedCtxErr.Error(), "context canceled") {
		t.Logf("注意：失败原因不是 context canceled，而是 %v", attachedCtxErr)
	}
	if detachedCtxErr != nil {
		t.Fatalf("Async() 之后查库仍失败：%v", detachedCtxErr)
	}
}

// TestAnchorHandlersUseAsyncForBackgroundJobs 护栏：后台锚文本相关的异步任务
// 必须走 Async()，否则请求返回后任务静默失效（见上面的复现测试）。
//
// 断言的是语法事实（GoStmt 的调用链），而不是日志或行为——
// 这类「静默失效」只有 AST 检查能在改动时立刻报警。
func TestAnchorHandlersUseAsyncForBackgroundJobs(t *testing.T) {
	src := "../controller/manageController/pluginAnchor.go"
	fset := token.NewFileSet()
	file, err := parser.ParseFile(fset, src, nil, 0)
	if err != nil {
		t.Fatalf("解析 %s 失败：%v", src, err)
	}

	var goroutines []string
	ast.Inspect(file, func(n ast.Node) bool {
		stmt, ok := n.(*ast.GoStmt)
		if !ok {
			return true
		}
		call, ok := stmt.Call.Fun.(*ast.SelectorExpr)
		if !ok {
			return true
		}
		// 形如 currentSite.Async().Xxx：直接调用的接收者是 CallExpr
		if inner, ok := call.X.(*ast.CallExpr); ok {
			if sel, ok := inner.Fun.(*ast.SelectorExpr); ok && sel.Sel.Name == "Async" {
				return true
			}
		}
		goroutines = append(goroutines, call.Sel.Name)
		return true
	})

	for _, name := range goroutines {
		switch name {
		case "ChangeAnchor", "ReplaceAnchor", "ReplaceAnchors":
			t.Errorf("异步任务 %s 没有走 Async()：请求返回后 ctx 会被 cancel，任务会静默失效", name)
		}
	}
}

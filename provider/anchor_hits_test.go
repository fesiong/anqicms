package provider

import (
	"go/ast"
	"go/parser"
	"go/token"
	"os"
	"strings"
	"testing"

	"kandaoni.com/anqicms/library"
	"kandaoni.com/anqicms/model"
)

// testAnchorSite 复用一个已初始化的站点，专用于锚文本相关断言。
//
// 与 TestReplaceContentText 一样连真实库（provider 包的既有测试风格），
// 但只读取站点结构，不写任何数据。
func testAnchorSite(t *testing.T) *Website {
	t.Helper()
	dbSite, err := GetDBWebsiteInfo(1)
	if err != nil {
		t.Skipf("跳过：无法读取站点 1（%v）", err)
	}
	InitWebsite(dbSite)
	w := GetWebsite(1)
	if w == nil {
		t.Fatal("站点 1 未初始化")
	}
	// 全局站点跨测试共享，Cache 必须补齐（否则后续测试 panic）
	if w.Cache == nil {
		w.Cache = library.InitMemoryCache()
	}
	// 密度最小 20（<20 会被强制改成 200），保证短样本也能替换多个
	w.PluginAnchor.AnchorDensity = 20
	w.PluginAnchor.NoStrongTag = 1
	return w
}

// TestReplaceContentTextReportsHitAnchorsMarkdown 锁定「markdown 正文替换后要能
// 报出命中了哪些锚文本」。
//
// 背景：ReplaceContent 原来靠正则扫 `data-anchor="(\d+)"` 回填 anchor_data 表，
// 但 markdown 分支生成的是 `[关键词](链接)`——**没有 data-anchor 属性**。
// 于是 markdown 站点（editor=markdown，也是本项目的主力形态）：
//   - anchor_data 永远为空 → 后台「替换次数」恒为 0；
//   - 改锚文本链接时 ChangeAnchor 找不到受影响文档 → 已发布文章里的旧链接永远不更新。
func TestReplaceContentTextReportsHitAnchorsMarkdown(t *testing.T) {
	w := testAnchorSite(t)

	anchors := []*model.Anchor{
		{Id: 101, Title: "锚文本甲", Link: "/a"},
		{Id: 102, Title: "锚文本乙", Link: "/b"},
	}
	content := "第一段里提到了锚文本甲这个说法，接着又补充了一些无关的说明文字。" +
		"第二段里提到了锚文本乙，同样夹在一段足够长的普通叙述中间。"

	out, hits, modified := w.replaceContentTextWithHits(anchors, content, "/doc")
	if !modified {
		t.Fatal("内容含关键词却没有被替换")
	}
	if !strings.Contains(out, "[锚文本甲](/a)") {
		t.Fatalf("markdown 分支应生成 [关键词](链接)，实得：%s", out)
	}
	if len(hits) != 2 {
		t.Fatalf("应报出 2 个命中锚文本，实得 %d 个：%v", len(hits), hits)
	}
	if hits[0] != 101 || hits[1] != 102 {
		t.Fatalf("命中顺序应为 [101 102]，实得 %v", hits)
	}
}

// TestReplaceContentTextReportsHitAnchorsHTML HTML 分支同样要报命中的 id，
// 否则 ReplaceContent 只能退回正则扫描（老路径），两种形态的数据会各记一套。
func TestReplaceContentTextReportsHitAnchorsHTML(t *testing.T) {
	w := testAnchorSite(t)

	anchors := []*model.Anchor{
		{Id: 201, Title: "锚文本丙", Link: "/c"},
	}
	content := library.MarkdownToHTML("第一段里提到了锚文本丙这个说法，接着又补充了一些无关的说明文字。", "", 0)

	out, hits, modified := w.replaceContentTextWithHits(anchors, content, "/doc")
	if !modified {
		t.Fatal("html 内容含关键词却没有被替换")
	}
	if !strings.Contains(out, `data-anchor="201"`) {
		t.Fatalf("html 分支应生成带 data-anchor 的链接，实得：%s", out)
	}
	if len(hits) != 1 || hits[0] != 201 {
		t.Fatalf("应报出命中 [201]，实得 %v", hits)
	}
}

// TestReplaceContentRecordsAnchorDataWithoutDataAnchor 锁定端到端行为：
// ReplaceContent 必须用「命中 id」而不是「正则扫 data-anchor」来写 anchor_data。
//
// 用 AST 静态断言即可——真正写库的断言需要造文章+锚文本，成本高且污染真实库；
// 而这里要防的是「有人把命中 id 的逻辑删回正则扫描」这种回退。
func TestReplaceContentRecordsAnchorDataWithoutDataAnchor(t *testing.T) {
	fset := token.NewFileSet()
	parsed, err := parser.ParseFile(fset, "anchor.go", nil, 0)
	if err != nil {
		t.Fatalf("解析 anchor.go 失败：%v", err)
	}

	var fn *ast.FuncDecl
	for _, decl := range parsed.Decls {
		if f, ok := decl.(*ast.FuncDecl); ok && f.Name.Name == "ReplaceContent" {
			fn = f
			break
		}
	}
	if fn == nil {
		t.Fatal("anchor.go 里找不到 ReplaceContent，测试前提不成立")
	}

	idents := map[string]bool{}
	ast.Inspect(fn, func(n ast.Node) bool {
		if id, ok := n.(*ast.Ident); ok {
			idents[id.Name] = true
		}
		return true
	})
	if !idents["replaceContentTextWithHits"] {
		t.Error("ReplaceContent 必须调用 replaceContentTextWithHits 取命中 id（否则 markdown 正文又不会写 anchor_data）")
	}

	// 反向对照：锚文本 handler 保存后必须让缓存失效，否则「刚加的锚文本」发布时不生效
	if !handlerMentionsAnchorCache(t, "../controller/manageController/pluginAnchor.go") {
		t.Error("PluginAnchorDetailForm 保存锚文本后没有失效锚文本缓存：新增的词 1 小时内不会参与替换")
	}
}

// handlerMentionsAnchorCache 检查锚文本保存 handler 是否触碰了缓存失效。
func handlerMentionsAnchorCache(t *testing.T, path string) bool {
	t.Helper()
	src, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("读取 %s 失败：%v", path, err)
	}
	text := string(src)
	// 只截取 PluginAnchorDetailForm 的函数体（到下一个顶层 func 为止）
	start := strings.Index(text, "func PluginAnchorDetailForm(ctx iris.Context) {")
	if start < 0 {
		t.Fatalf("%s 里找不到 PluginAnchorDetailForm", path)
	}
	rest := text[start:]
	if next := strings.Index(rest[1:], "\nfunc "); next > 0 {
		rest = rest[:next+1]
	}
	return strings.Contains(rest, "SaveAnchor") || strings.Contains(rest, "AnchorCacheKey")
}

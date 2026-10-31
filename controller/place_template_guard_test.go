package controller

import (
	"strings"
	"testing"
)

// 2026-10-30 的现场：站点切到 shop 模板（该模板没有 place/ 目录）后，
// 访问城市站页面得到 `[Error (where: fromfile) in place/detail.html] unable to resolve template`。
//
// 根因不是模板写错，而是这两个 handler 直接 ctx.View 一个可能不存在的模板：
// 模板缺失时视图引擎会渲染出"模板错误页"，而错误页渲染是成功的，
// 于是它被当成正常内容写进 HTML 缓存（cache/pc/index.html 至今是那 104 字节错误页），
// 首页对所有访客都返回 200 + 这段 Go 错误文本。
//
// 正确做法与 archive/guestbook/category 一致：先 TemplateExist，取不到就 NotFound。
// 只断言"调用了 View"锁不住这个形态——缺陷正是"调用了 View 但没先确认模板存在"。
func TestPlacePagesCheckTemplateBeforeView(t *testing.T) {
	cases := []struct {
		file string
		fn   string
	}{
		{"place.go", "PlacePage"},
		{"place.go", "PlaceIndex"},
	}

	for _, c := range cases {
		src := readFuncSource(t, c.file, c.fn)
		if !strings.Contains(src, "TemplateExist(") {
			t.Errorf("%s 未先检查模板是否存在：模板缺失时会渲染出错误页并被写进 HTML 缓存", c.fn)
			continue
		}
		if !strings.Contains(src, "NotFound(ctx)") {
			t.Errorf("%s 检查到模板缺失后必须 NotFound，不能继续 View", c.fn)
			continue
		}

		checkAt := strings.Index(src, "TemplateExist(")
		viewAt := strings.Index(src, "ctx.View(")
		if viewAt < 0 {
			t.Errorf("%s 里找不到 ctx.View，断言失效", c.fn)
			continue
		}
		if checkAt > viewAt {
			t.Errorf("%s 的模板存在性检查必须放在 ctx.View 之前（当前在第 %d 字符，View 在第 %d 字符）",
				c.fn, checkAt, viewAt)
		}
	}
}

// 生成静态缓存时同样要判模板存在：城市站批量生成会走内部 HTTP 请求，
// 模板缺失时那一次请求只会拿到 404 页，而 404 页会被写进缓存。
func TestBuildSinglePlaceCacheSkipsMissingTemplate(t *testing.T) {
	src := readFuncSource(t, "../provider/htmlCache.go", "BuildSinglePlaceCache")

	if !strings.Contains(src, "TemplateExist(") {
		t.Fatal("BuildSinglePlaceCache 未检查模板是否存在：模板缺失时会把 404 页写进 HTML 缓存")
	}
	viewAt := strings.Index(src, "Application().View(")
	if viewAt < 0 {
		t.Fatal("BuildSinglePlaceCache 里找不到渲染调用，断言失效")
	}
	if strings.Index(src, "TemplateExist(") > viewAt {
		t.Fatal("模板存在性检查必须放在渲染之前")
	}
}

// 城市站规则的 pattern 是 `/{filename}`，能匹配任何路径。它只应在
// 「已经确认这是城市站 URL」时参与路由（子域名解析、或目录模式的主循环）。
// 一旦它落进 ParseRoute 末尾那段「其余规则都当 common 兜底」的循环，
// 任何没命中的 URL 都会被判成城市站详情页：/article/blog、/product/gloves
// 全部 404（实测整站分类、产品列表页全挂）。
func TestParseRouteDoesNotFallbackToPlaceRule(t *testing.T) {
	src := readFuncSource(t, "common.go", "ParseRoute")

	// 兜底循环从「处理common」这段注释开始，用它作锚点，
	// 避免把主循环里那处（带记录存在校验的）place 判断误当成护栏。
	fallbackAt := strings.Index(src, "// 处理common")
	if fallbackAt < 0 {
		t.Fatal("找不到 common 兜底循环的锚点注释，断言失效")
	}
	guard := "ruleName == provider.PatternPlace || ruleName == provider.PatternPlaceIndex"
	tail := src[fallbackAt:]
	guardAt := strings.Index(tail, guard)
	if guardAt < 0 {
		t.Fatal("common 兜底循环没有排除 place 规则：任何未命中的 URL 都会被判成城市站详情页")
	}
	if !strings.Contains(tail[guardAt:], "continue") {
		t.Fatal("排除 place 规则后必须 continue，否则仍会走到兜底赋值")
	}
}

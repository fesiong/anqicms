package provider

import (
	"regexp"
	"testing"

	"github.com/kataras/iris/v12/i18n"
	"kandaoni.com/anqicms/config"
)

// newTplI18nForTest 造一个只含指定词条的语言包。
func newTplI18nForTest(tb testing.TB, lang string, kv map[string]interface{}) *i18n.I18n {
	tb.Helper()
	i := i18n.New()
	if err := i.LoadKV(i18n.LangMap{lang: kv}, lang); err != nil {
		tb.Fatalf("装载语言包失败：%v", err)
	}
	return i
}

// 模板翻译缓存必须真的「命中」。
//
// 验证手法：换掉站点语言包但不清缓存，第二次调用必须还能拿到旧译文。
// 只要缓存被绕开（比如把 TplTr 退回直调 i18n.Tr），就会立刻拿到新译文而变红——
// 这是缓存是否生效的唯一可观测证据，光比对两次结果相同是测不出来的。
func TestTplTrCacheHitsUntilReset(t *testing.T) {
	ResetTplTrCache()
	t.Cleanup(ResetTplTrCache)

	site := &Website{
		Id:      1,
		System:  &config.SystemConfig{Language: "zh-CN"},
		TplI18n: newTplI18nForTest(t, "zh-CN", map[string]interface{}{"hello": "第一次"}),
	}

	if got := site.TplTr("hello"); got != "第一次" {
		t.Fatalf("首次翻译 = %q，期望 %q", got, "第一次")
	}

	// 换语言包，故意不调 ResetTplTrCache
	site.TplI18n = newTplI18nForTest(t, "zh-CN", map[string]interface{}{"hello": "第二次"})
	if got := site.TplTr("hello"); got != "第一次" {
		t.Fatalf("缓存没生效：第二次拿到 %q，期望仍命中旧译文 %q", got, "第一次")
	}

	// 正常的重载路径会清缓存，清完必须看到新译文
	ResetTplTrCache()
	if got := site.TplTr("hello"); got != "第二次" {
		t.Fatalf("ResetTplTrCache 之后 = %q，期望 %q", got, "第二次")
	}
}

// 缓存键必须区分站点与语言：同一个原文在不同站点/语言下译文不同，
// 多站点共用进程时串了就是满屏错语种。
func TestTplTrCacheKeyIsolatesSitesAndLangs(t *testing.T) {
	ResetTplTrCache()
	t.Cleanup(ResetTplTrCache)

	siteA := &Website{
		Id:      1,
		System:  &config.SystemConfig{Language: "zh-CN"},
		TplI18n: newTplI18nForTest(t, "zh-CN", map[string]interface{}{"hello": "站点一"}),
	}
	siteB := &Website{
		Id:      2,
		System:  &config.SystemConfig{Language: "zh-CN"},
		TplI18n: newTplI18nForTest(t, "zh-CN", map[string]interface{}{"hello": "站点二"}),
	}

	if got := siteA.TplTr("hello"); got != "站点一" {
		t.Fatalf("站点 1 译文 = %q", got)
	}
	if got := siteB.TplTr("hello"); got != "站点二" {
		t.Fatalf("站点 2 被站点 1 的缓存污染：%q", got)
	}

	// 同站点换语言，键里带了语言，必须重新查
	siteA.System.Language = "en-US"
	siteA.TplI18n = newTplI18nForTest(t, "en-US", map[string]interface{}{"hello": "site-one"})
	if got := siteA.TplTr("hello"); got != "site-one" {
		t.Fatalf("切换语言后 = %q，期望 %q（键里必须含语言）", got, "site-one")
	}
}

// 带格式化参数的调用不进缓存（结果依赖参数），但译文本身必须仍然正确。
func TestTplTrWithArgsIsNotCached(t *testing.T) {
	ResetTplTrCache()
	t.Cleanup(ResetTplTrCache)

	site := &Website{
		Id:      9,
		System:  &config.SystemConfig{Language: "zh-CN"},
		TplI18n: newTplI18nForTest(t, "zh-CN", map[string]interface{}{"hi %s": "你好 %s"}),
	}

	if got := site.TplTr("hi %s", "A"); got != "你好 A" {
		t.Fatalf("带参翻译 = %q，期望 %q", got, "你好 A")
	}
	if got := site.TplTr("hi %s", "B"); got != "你好 B" {
		t.Fatalf("带参翻译 = %q，期望 %q（参数不能被缓存吞掉）", got, "你好 B")
	}
}

// 列名/方向校验走了带缓存的快路径，判定必须与原来的正则逐字一致。
// 这是 ORDER BY 注入的唯一闸门，等价性不能靠肉眼保证。
func TestOrderValidationMatchesRegex(t *testing.T) {
	dirRegex := regexp.MustCompile("^(?i)(asc|desc)$")

	allow := []string{
		"id", "`id`", "archives.id", "`archives`.`id`", "sort", "created_time",
		"a", "A1", "t.c", "`t`.`c`", "_hidden", "user_name",
		// 存量语义（不是本次引入，也不在这次改动里收紧）：
		// 正则允许纯数字（MySQL 的 ORDER BY 1 表示按第 1 列排），
		// 也允许反引号不成对（`a`.b` 这种，仍是合法标识符形态）。
		"1", "`a`.b`",
	}
	deny := []string{
		"", " ", "id;", "a b", "id--", "id/*x*/", "(select 1)", "id`=`x`",
		"id desc", "`id` `x`", "a..b", ".a", "a.", "id||x", "ümlaut",
		"id\x00", " id", "id ", "archives.id.", "max(id)",
	}

	for _, s := range allow {
		if !isValidOrderColumn(s) {
			t.Errorf("列名 %q 应放行", s)
		}
	}
	for _, s := range deny {
		if isValidOrderColumn(s) {
			t.Errorf("列名 %q 应拒绝", s)
		}
	}

	// 全量等价性：无论放行还是拒绝，快速路径都必须与正则得出同一结论
	for _, s := range append(append([]string{}, allow...), deny...) {
		if got, want := isValidOrderColumn(s), orderColumnRegex.MatchString(s); got != want {
			t.Errorf("isValidOrderColumn(%q) = %v，正则 = %v", s, got, want)
		}
	}

	for _, s := range []string{"asc", "desc", "ASC", "Desc", "aSc"} {
		if !isValidOrderDir(s) {
			t.Errorf("方向 %q 应放行", s)
		}
	}
	for _, s := range []string{"", " ", "asc ", " asc", "ascx", "asc1", "asc,desc", "asc;", "десц"} {
		if isValidOrderDir(s) {
			t.Errorf("方向 %q 应拒绝", s)
		}
	}
	for _, s := range []string{"asc", "desc", "ASC", "Desc", "aSc", "", " ", "asc ", " asc", "ascx", "asc1", "asc,desc", "asc;"} {
		if got, want := isValidOrderDir(s), dirRegex.MatchString(s); got != want {
			t.Errorf("isValidOrderDir(%q) = %v，正则 = %v", s, got, want)
		}
	}
}

// 量化缓存收益：模板里每个 tr 标签都会走一次 TplTr，命中路径必须比
// 直调 i18n.Tr（内部 language.Parse + 匹配）便宜一个数量级，否则不值得加这层。
func BenchmarkTplTrCachedVsUncached(b *testing.B) {
	ResetTplTrCache()
	b.Cleanup(ResetTplTrCache)

	site := &Website{
		Id:      1,
		System:  &config.SystemConfig{Language: "zh-CN"},
		TplI18n: newTplI18nForTest(b, "zh-CN", map[string]interface{}{"hello": "你好"}),
	}
	// 预热，让后续循环全部走命中路径
	site.TplTr("hello")

	b.Run("cached", func(b *testing.B) {
		b.ReportAllocs()
		for i := 0; i < b.N; i++ {
			_ = site.TplTr("hello")
		}
	})
	b.Run("uncached", func(b *testing.B) {
		b.ReportAllocs()
		for i := 0; i < b.N; i++ {
			_ = site.tplTr("hello")
		}
	})
}

package provider

import (
	"fmt"
	"testing"

	"kandaoni.com/anqicms/config"
)

func newTestSitemapGenerator() *SitemapGenerator {
	w := &Website{PluginSitemap: &config.PluginSitemapConfig{Type: "xml"}}
	return NewSitemapGenerator(w, "", "https://cn.example.com", false)
}

// TestSitemapAddLocKeepsAlternates 防止 hreflang 切片被后续条目覆写。
//
// BuildSitemap/AddonSitemap 都采用「同一份 alternates 反复 [:0] 复位后 append」
// 的写法（见 sitemap.go 中 alternates = alternates[:0]）。复位只改长度、不换底层数组，
// 而 AddLoc 若直接保存传入的切片头，后续 append 会原地改写已写入条目的 hreflang ——
// 结果是一个 sitemap 文件里所有 url 都指向最后一条的 alternate 地址。
//
// 这里刻意复刻调用方的复位-复用写法：改回「直接赋值」时本测试必红。
func TestSitemapAddLocKeepsAlternates(t *testing.T) {
	g := newTestSitemapGenerator()

	var alternates []AlternateLink
	for i := 0; i < 3; i++ {
		alternates = alternates[:0]
		alternates = append(alternates, AlternateLink{
			Href:     fmt.Sprintf("https://en.example.com/%d", i),
			Hreflang: "en",
		})
		g.AddLoc(fmt.Sprintf("https://cn.example.com/%d", i), "2026-01-01", alternates)
	}

	if len(g.Urls) != 3 {
		t.Fatalf("urls = %d, want 3", len(g.Urls))
	}
	for i := 0; i < 3; i++ {
		want := fmt.Sprintf("https://en.example.com/%d", i)
		if len(g.Urls[i].Alternates) != 1 {
			t.Fatalf("url[%d] alternates = %d, want 1", i, len(g.Urls[i].Alternates))
		}
		if got := g.Urls[i].Alternates[0].Href; got != want {
			t.Errorf("url[%d] alternate = %q, want %q (alternates 被后续条目覆写)", i, got, want)
		}
	}
}

// TestSitemapAddLocEmptyAlternatesStayOmitted 确认复制不会把空切片变成非空切片：
// SitemapUrl.Alternates 是 omitempty，空切片会渲染出多余的 xhtml:link。
func TestSitemapAddLocEmptyAlternatesStayOmitted(t *testing.T) {
	g := newTestSitemapGenerator()
	g.AddLoc("https://cn.example.com/a", "2026-01-01", nil)
	if got := len(g.Urls[0].Alternates); got != 0 {
		t.Fatalf("alternates = %d, want 0", got)
	}
}

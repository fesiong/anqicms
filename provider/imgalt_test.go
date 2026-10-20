package provider

import (
	"strings"
	"testing"
)

const altTestTitle = `5L 单层玻璃反应釜 "Pro" 型`

// 无 alt 的图片补文档标题；多张图追加序号避免 SEO 重复。
func TestFillContentImgAltFillsMissing(t *testing.T) {
	content := `<p><img src="/a.jpg"><img src="/b.jpg" /></p>`
	got := FillContentImgAlt(content, altTestTitle)
	want1 := `alt="5L 单层玻璃反应釜 &#34;Pro&#34; 型"`
	if !strings.Contains(got, want1) {
		t.Errorf("第1张图应补标题 alt，得到 %s", got)
	}
	if !strings.Contains(got, `alt="5L 单层玻璃反应釜 &#34;Pro&#34; 型-2"`) {
		t.Errorf("第2张图应带序号 alt，得到 %s", got)
	}
	// 确认是插入而非破坏标签
	if !strings.Contains(got, `<img alt=`) || !strings.Contains(got, `src="/a.jpg"`) {
		t.Errorf("标签结构被破坏: %s", got)
	}
}

// 已有非空 alt 的图片必须原样保留。
func TestFillContentImgAltKeepsExisting(t *testing.T) {
	content := `<img src="/a.jpg" alt="用户手动设置" /><img src='/b.jpg' alt='  ' />`
	got := FillContentImgAlt(content, altTestTitle)
	if !strings.Contains(got, `alt="用户手动设置"`) {
		t.Errorf("非空 alt 不应被覆盖: %s", got)
	}
	// 第二张是空白 alt（含空格），应被填充
	if !strings.Contains(got, `alt='`) || !strings.Contains(got, "反应釜") {
		t.Errorf("空白 alt 应被填充: %s", got)
	}
}

// 无 alt 属性（大小写混合标签）应正确插入。
func TestFillContentImgAltInsertsAttribute(t *testing.T) {
	content := `<IMG SRC='/a.jpg' class='pic'>`
	got := FillContentImgAlt(content, "标题")
	if !strings.Contains(got, `<IMG alt="标题" SRC=`) {
		t.Errorf("应紧跟 <img 插入 alt 且保留原大小写: %s", got)
	}
}

// title 为空或 content 为空时原样返回。
func TestFillContentImgAltNoop(t *testing.T) {
	if got := FillContentImgAlt("", "标题"); got != "" {
		t.Errorf("空 content 应原样返回")
	}
	c := `<img src="/a.jpg">`
	if got := FillContentImgAlt(c, "  "); got != c {
		t.Errorf("空 title 应原样返回")
	}
}

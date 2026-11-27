package provider

import (
	"strings"
	"testing"

	"kandaoni.com/anqicms/response"
)

// 帮助面板是后台里品牌面积最大的一块：87 条链接指向官方文档站，
// 客户点「帮助」看到的就是官网。这里钉死重写行为，防止哪天被“优化”掉。

func sampleDocs() []response.DesignDocGroup {
	return []response.DesignDocGroup{
		{
			Title: "模板制作",
			Docs: []response.DesignDoc{
				{Title: "基本约定", Link: officialDocsHost + "/help-design/116.html"},
				{Title: "目录结构", Link: officialDocsHost + "/help-design/117.html"},
				{Title: "本地内容", Content: "无链接的条目"},
			},
		},
		{
			Title: "模板标签",
			Docs: []response.DesignDoc{
				{
					Title: "系统设置",
					Link:  officialDocsHost + "/manual-normal/73.html",
					Docs: []response.DesignDoc{
						// 嵌套一层：标签文档里还挂着子标签文档的链接。
						{Title: "网站名称", Link: officialDocsHost + "/manual-normal/74.html"},
					},
				},
			},
		},
	}
}

func TestRewriteDocsLinksKeepsOfficial(t *testing.T) {
	got := rewriteDocsLinks(sampleDocs(), officialDocsHost)
	if got[0].Docs[0].Link != officialDocsHost+"/help-design/116.html" {
		t.Errorf("官方版链接被改动：%s", got[0].Docs[0].Link)
	}
	if got[1].Docs[0].Docs[0].Link != officialDocsHost+"/manual-normal/74.html" {
		t.Errorf("官方版嵌套链接被改动：%s", got[1].Docs[0].Docs[0].Link)
	}
}

func TestRewriteDocsLinksToOEMHost(t *testing.T) {
	const base = "https://help.acme.test"
	got := rewriteDocsLinks(sampleDocs(), base+"/") // 尾部斜杠必须被吃掉，否则出现双斜杠

	if want := base + "/help-design/116.html"; got[0].Docs[0].Link != want {
		t.Errorf("顶层链接：got %s want %s", got[0].Docs[0].Link, want)
	}
	if want := base + "/manual-normal/74.html"; got[1].Docs[0].Docs[0].Link != want {
		t.Errorf("嵌套链接未重写（递归缺失）：got %s want %s", got[1].Docs[0].Docs[0].Link, want)
	}
	for _, g := range got {
		for _, d := range g.Docs {
			if strings.Contains(d.Link, "anqicms.com") {
				t.Errorf("仍残留官方域名：%s", d.Link)
			}
		}
	}
	// 没有链接的条目不能被改动。
	if got[0].Docs[2].Link != "" || got[0].Docs[2].Content != "无链接的条目" {
		t.Errorf("无链接条目被改动：%+v", got[0].Docs[2])
	}
}

// 品牌没有独立文档站时清空链接，而不是保留官方地址：
// 留着官方链接等于把客户导到竞品文档站上。
func TestRewriteDocsLinksClearsWhenNoBase(t *testing.T) {
	got := rewriteDocsLinks(sampleDocs(), "")
	if got[0].Docs[0].Link != "" {
		t.Errorf("应清空链接，got %q", got[0].Docs[0].Link)
	}
	if got[1].Docs[0].Docs[0].Link != "" {
		t.Errorf("嵌套链接应清空，got %q", got[1].Docs[0].Docs[0].Link)
	}
	// 面板结构本身要保留，否则标签文档面板会整块消失。
	if len(got) != 2 || len(got[0].Docs) != 3 {
		t.Errorf("结构被破坏：%d 组 / %d 条", len(got), len(got[0].Docs))
	}
}

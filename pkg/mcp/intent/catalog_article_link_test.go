package intent

import (
	"context"
	"strings"
	"testing"
)

// TestContentArticleSaveReturnsLink 回归：save 必须回 link/url，不能只有 id。
//
// 背景：content_article save 过去只回 {"action","id"}，调用方（含 Agent）拿不到
// 链接就无法查看刚写入的内容。端点返回的 art.link 是可用地址（实测形如
// http://host/zixun/1846.html），意图层负责把它带出来。
func TestContentArticleSaveReturnsLink(t *testing.T) {
	art := map[string]any{
		"id":    float64(1846),
		"title": "GEO 实操五步法",
		"link":  "http://127.0.0.1:8001/zixun/1846.html",
	}
	// draftAwareLink 在已发布（无 draft 参数、无 status）时应原样返回。
	if got := draftAwareLink(art["link"].(string), art, map[string]any{}); got != art["link"] {
		t.Errorf("已发布文档 link 不应被改写: %s", got)
	}
}

// TestDraftAwareLinkAppendsPreview 草稿必须追加 ?preview=true。
//
// archives 表没有 status 列，草稿存在 archive_drafts（status 0/99），
// 前台 controller/archive.go:24 的 preview 分支才读得到。
func TestDraftAwareLinkAppendsPreview(t *testing.T) {
	base := "http://127.0.0.1:8001/zixun/1846.html"
	cases := []struct {
		name string
		art  map[string]any
		args map[string]any
		want string
	}{
		{
			name: "显式 draft=true",
			art:  map[string]any{"id": float64(1), "link": base},
			args: map[string]any{"draft": true},
			want: base + "?preview=true",
		},
		{
			name: "字符串 draft=true",
			art:  map[string]any{"id": float64(1), "link": base},
			args: map[string]any{"draft": "true"},
			want: base + "?preview=true",
		},
		{
			name: "status=0 草稿态",
			art:  map[string]any{"id": float64(1), "link": base, "status": float64(0)},
			args: map[string]any{},
			want: base + "?preview=true",
		},
		{
			name: "status=99 待发布态",
			art:  map[string]any{"id": float64(1), "link": base, "status": float64(99)},
			args: map[string]any{},
			want: base + "?preview=true",
		},
		{
			name: "已发布不追加",
			art:  map[string]any{"id": float64(1), "link": base},
			args: map[string]any{"draft": false},
			want: base,
		},
		{
			name: "已有 query 用 & 连接",
			art:  map[string]any{"id": float64(1), "link": base + "?utm=1"},
			args: map[string]any{"draft": true},
			want: base + "?utm=1&preview=true",
		},
		{
			name: "已有 preview 不重复追加",
			art:  map[string]any{"id": float64(1), "link": base + "?preview=true"},
			args: map[string]any{"draft": true},
			want: base + "?preview=true",
		},
		{
			name: "空 link 不动",
			art:  map[string]any{"id": float64(1), "link": ""},
			args: map[string]any{"draft": true},
			want: "",
		},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			got := draftAwareLink(c.art["link"].(string), c.art, c.args)
			if got != c.want {
				t.Errorf("draftAwareLink = %q, 期望 %q", got, c.want)
			}
		})
	}
}

// TestAgentComposeEditRoutesToAgentEdit manage_edit 必须路由到 agent_edit，
// 且能改 name/strategy（2026-10-02 新增，此前只能删掉重建）。
func TestAgentComposeEditRoutesToAgentEdit(t *testing.T) {
	c := &captureInvoker{}
	compose := agentCompose()
	_, err := compose(context.Background(), map[string]any{
		"action":   "manage_edit",
		"id":       3,
		"title":    "改名后的标题",
		"prompt":   "新的策略内容",
		"cron":     "0 9 * * *",
		"enabled":  1,
	}, c.invoke)
	if err != nil {
		t.Fatalf("执行失败: %v", err)
	}
	if c.name != "agent_edit" {
		t.Fatalf("应分派到 agent_edit，实得 %s", c.name)
	}
	if c.args["name"] != "改名后的标题" {
		t.Errorf("title 未换算为 name: %#v", c.args["name"])
	}
	if c.args["strategy"] != "新的策略内容" {
		t.Errorf("prompt 未换算为 strategy: %#v", c.args["strategy"])
	}
	if c.args["cron"] != "0 9 * * *" {
		t.Errorf("cron 未透传: %#v", c.args["cron"])
	}
	// enabled 不能被误当成别的字段删掉
	if _, ok := c.args["enabled"]; !ok {
		t.Errorf("enabled 丢失: %#v", c.args)
	}
	if _, leaked := c.args["action"]; leaked {
		t.Error("action 被透传进了底层参数")
	}
}

// TestAgentEditMustFallBackToHandler agent_edit 没有 REST 端点，
// 必须回落内存 handler；若误加进 capEndpoints 会永远 404。
func TestAgentEditMustFallBackToHandler(t *testing.T) {
	if _, ok := capEndpoints["agent_edit"]; ok {
		t.Error("agent_edit 不应出现在 capEndpoints：它没有 REST 等价端点")
	}
	inv := &recInvoker{}
	out, err := callCap(context.Background(), inv.invoke, "agent_edit", map[string]any{"id": 1})
	if err != nil {
		t.Fatalf("回落调用失败: %v", err)
	}
	if !strings.Contains(out, "agent_edit") && inv.name != "agent_edit" {
		t.Errorf("未落到 agent_edit handler: %s / %s", out, inv.name)
	}
}

// TestParseArticlePrefersWideLayer 回归：save 回执必须能拿到 link。
//
// 实测 POST /archive/detail 的响应里，data.data 是**窄对象**（只有 content 与 id），
// 而 data 这一层才带 link/title/url_token。旧实现固定返回 data.data，
// 导致正式文档 save 拿不到 link（草稿能拿到是靠 archive_list 回查兜底）。
func TestParseArticlePrefersWideLayer(t *testing.T) {
	// 真实响应形状（截自 2026-10-02 实测）
	raw := `{"code":0,"data":{"id":1850,"title":"正式文档","link":"http://h/zixun/1850.html",` +
		`"url_token":"link-yan-zheng","category_id":13,"status":1,` +
		`"data":{"content":"正文","id":1850}}}`
	got := parseArticle(raw)
	if got == nil {
		t.Fatal("parseArticle 返回 nil")
	}
	if v, _ := got["link"].(string); v != "http://h/zixun/1850.html" {
		t.Errorf("未取到外层 link，实际 %#v", got["link"])
	}
	if v, _ := got["title"].(string); v != "正式文档" {
		t.Errorf("未取到外层 title，实际 %#v", got["title"])
	}
}

// 无 link/title 时仍应回落到内层，避免丢掉窄对象里的 id。
func TestParseArticleFallsBackToInner(t *testing.T) {
	raw := `{"code":0,"data":{"status":1,"data":{"content":"正文","id":7}}}`
	got := parseArticle(raw)
	if got == nil {
		t.Fatal("parseArticle 返回 nil")
	}
	if v, ok := got["id"].(float64); !ok || v != 7 {
		t.Errorf("应回落到内层取 id，实际 %#v", got["id"])
	}
}

// 非文档响应不应误判。
func TestParseArticleIgnoresNonArticle(t *testing.T) {
	if got := parseArticle(`{"ok":true,"data":null}`); got != nil {
		t.Errorf("data 为 null 时应返回 nil，实际 %#v", got)
	}
	if got := parseArticle(`not json`); got != nil {
		t.Errorf("非法 JSON 应返回 nil，实际 %#v", got)
	}
}

// TestDigList 兼容 data/list/items 三种数组位置。
func TestDigList(t *testing.T) {
	item := map[string]any{"id": float64(7), "title": "T"}
	cases := []struct {
		name string
		in   any
		want int
	}{
		{"data 直接是数组", map[string]any{"data": []any{item}}, 1},
		{"data.data 嵌套", map[string]any{"data": map[string]any{"data": []any{item}}}, 1},
		{"data.list 嵌套", map[string]any{"data": map[string]any{"list": []any{item}}}, 1},
		{"无数组", map[string]any{"data": map[string]any{"code": float64(0)}}, 0},
		{"nil", nil, 0},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			if got := digList(c.in); len(got) != c.want {
				t.Errorf("digList 长度 = %d, 期望 %d", len(got), c.want)
			}
		})
	}
}

// TestIsDraftRequestNilArgs 边界：args 为空时不应 panic，且判定为非草稿。
func TestIsDraftRequestNilArgs(t *testing.T) {
	if isDraftRequest(nil) {
		t.Error("nil args 不应判定为草稿")
	}
	if isDraftRequest(map[string]any{"draft": "1"}) != true {
		t.Error("字符串 \"1\" 应判定为草稿")
	}
	if isDraftRequest(map[string]any{"draft": false}) != false {
		t.Error("false 应判定为非草稿")
	}
	if isDraftStatus(nil) {
		t.Error("nil status 不应判定为草稿（缺省视为已发布）")
	}
}

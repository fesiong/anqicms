package provider

import "testing"

// TestClampListPageSize 列表分页参数必须被夹到安全范围。
//
// 背景（2026-10-27 实测）：content_article list 传 page_size=100000 时
// 端点照单全收，一次返回 3550 篇文档、6.73MB structuredContent（约 150 万 token），
// 足以一次性打爆任何模型的上下文。
func TestClampListPageSize(t *testing.T) {
	cases := []struct {
		name string
		in   map[string]any
		want any
	}{
		{"超大 pageSize 夹到上限", map[string]any{"pageSize": float64(100000)}, maxListPageSize},
		{"字符串数字也认", map[string]any{"pageSize": "5000"}, maxListPageSize},
		{"0 归一到 1", map[string]any{"pageSize": float64(0)}, 1},
		{"负数归一到 1", map[string]any{"pageSize": -3}, 1},
		{"正常值不动", map[string]any{"pageSize": float64(50)}, 50},
		{"current=0 归一到 1", map[string]any{"current": 0}, 1},
	}
	for _, c := range cases {
		got := clampListPageSize(c.in)
		key := "pageSize"
		if _, ok := c.in["current"]; ok {
			key = "current"
		}
		if got[key] != c.want {
			t.Errorf("%s: %s = %#v，期望 %#v", c.name, key, got[key], c.want)
		}
	}
}

// TestClampListPageSizeLeavesBusinessField page_size 是业务字段时不能被当分页夹紧。
//
// sitemap 配置的 page_size 表示「每页 URL 数」，夹到 200 会把用户配置改小。
// 分页词表在意图层已换算成端点的 pageSize，故这里只管 camelCase 键。
func TestClampListPageSizeLeavesBusinessField(t *testing.T) {
	in := map[string]any{"page_size": float64(5000)}
	got := clampListPageSize(in)
	if got["page_size"] != float64(5000) {
		t.Errorf("业务字段 page_size 不该被改动，实际 %#v", got["page_size"])
	}
}

// TestClampListPageSizeNoopWithoutPaging 没有分页参数时原样返回（含 nil）。
func TestClampListPageSizeNoopWithoutPaging(t *testing.T) {
	if got := clampListPageSize(nil); got != nil {
		t.Errorf("nil 应原样返回，实际 %#v", got)
	}
	in := map[string]any{"id": float64(1)}
	if got := clampListPageSize(in); got["id"] != float64(1) || len(got) != 1 {
		t.Errorf("非分页参数不应被改动，实际 %#v", got)
	}
}

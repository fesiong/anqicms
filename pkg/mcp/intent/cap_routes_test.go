package intent

import (
	"context"
	"testing"
)

// recInvoker 记录最后一次能力调用，用于断言 callCap 的路由结果。
type recInvoker struct {
	name string
	args map[string]any
}

func (r *recInvoker) invoke(ctx context.Context, name string, args map[string]any) (string, error) {
	r.name = name
	r.args = args
	return "{}", nil
}

// TestCallCapRoutesToEndpoint 已映射的能力必须走 api_invoke，且参数换算正确。
func TestCallCapRoutesToEndpoint(t *testing.T) {
	inv := &recInvoker{}
	if _, err := callCap(context.Background(), inv.invoke, "archive_list", map[string]any{
		"page":    2,
		"keyword": "SEO",
	}); err != nil {
		t.Fatalf("archive_list 调用失败: %v", err)
	}
	if inv.name != "api_invoke" {
		t.Fatalf("archive_list 应路由到 api_invoke，实际 %q", inv.name)
	}
	method, _ := inv.args["method"].(string)
	path, _ := inv.args["path"].(string)
	if method != "GET" || path != "/system/api/archive/list" {
		t.Fatalf("端点解析错误: %s %s", method, path)
	}
	params, _ := inv.args["params"].(map[string]any)
	if params["current"] != 2 {
		t.Errorf("page 应重命名为 current，实际 params=%v", params)
	}
	if params["title"] != "SEO" {
		t.Errorf("keyword 应重命名为 title，实际 params=%v", params)
	}
}

// TestArchiveListRename 验证列表排序/状态参数正确重命名为端点字段，
// 避免 order_by/order_dir 原样透传被端点忽略（此前 content_list_articles 排序失效）。
func TestArchiveListRename(t *testing.T) {
	inv := &recInvoker{}
	if _, err := callCap(context.Background(), inv.invoke, "archive_list", map[string]any{
		"order_by":  "created_time",
		"order_dir": "asc",
		"status":    "draft",
	}); err != nil {
		t.Fatalf("archive_list 调用失败: %v", err)
	}
	params, _ := inv.args["params"].(map[string]any)
	if params["sort"] != "created_time" {
		t.Errorf("order_by 应重命名为 sort，实际 params=%v", params)
	}
	if params["order"] != "asc" {
		t.Errorf("order_dir 应重命名为 order，实际 params=%v", params)
	}
	if params["status"] != "draft" {
		t.Errorf("status 应原样透传（端点接受 ok/draft/plan），实际 %v", params["status"])
	}
	if _, has := params["order_by"]; has {
		t.Errorf("order_by 不应原样出现在端点参数里，应已重命名为 sort")
	}
	if _, has := params["order_dir"]; has {
		t.Errorf("order_dir 不应原样出现在端点参数里，应已重命名为 order")
	}
}

// TestCallCapKeepsUnmappedCap 未映射的能力必须回落真实 handler，不能被吞掉。
func TestCallCapKeepsUnmappedCap(t *testing.T) {
	// 这 11 个是确认没有 REST 等价物的能力，必须保持回落。
	kept := []string{
		"attachment_upload", "template_reload", "skill_search", "skill_install",
		"agent_create", "agent_list", "agent_delete", "agent_toggle", "agent_run", "agent_chat",
		"task",
	}
	for _, name := range kept {
		if _, ok := capEndpoints[name]; ok {
			t.Errorf("%s 不应出现在 capEndpoints：它没有 REST 等价端点", name)
		}
		inv := &recInvoker{}
		if _, err := callCap(context.Background(), inv.invoke, name, map[string]any{}); err != nil {
			t.Fatalf("%s 调用失败: %v", name, err)
		}
		if inv.name != name {
			t.Errorf("%s 应回落到能力 handler，实际调用 %q", name, inv.name)
		}
	}
}

// TestCallCapAppliesFixedDefaultsAndArrays 常量注入、缺省值与数组化都要生效。
func TestCallCapAppliesFixedDefaultsAndArrays(t *testing.T) {
	// Fixed：单页复用 category 端点，靠 type 常量区分
	inv := &recInvoker{}
	if _, err := callCap(context.Background(), inv.invoke, "page_list", map[string]any{}); err != nil {
		t.Fatalf("page_list 调用失败: %v", err)
	}
	params, _ := inv.args["params"].(map[string]any)
	if params["type"] != CategoryTypePage {
		t.Errorf("page_list 应注入 type=%d，实际 params=%v", CategoryTypePage, params)
	}

	// Defaults：approve 未传 status 时补 1
	inv2 := &recInvoker{}
	if _, err := callCap(context.Background(), inv2.invoke, "comment_approve", map[string]any{"id": 7}); err != nil {
		t.Fatalf("comment_approve 调用失败: %v", err)
	}
	p2, _ := inv2.args["params"].(map[string]any)
	if p2["status"] != 1 {
		t.Errorf("comment_approve 缺省 status 应为 1，实际 %v", p2["status"])
	}
	// 显式传入时不应被覆盖
	inv3 := &recInvoker{}
	if _, err := callCap(context.Background(), inv3.invoke, "comment_approve", map[string]any{"id": 7, "status": 3}); err != nil {
		t.Fatalf("comment_approve 调用失败: %v", err)
	}
	p3, _ := inv3.args["params"].(map[string]any)
	if p3["status"] != 3 {
		t.Errorf("显式 status=3 不应被缺省值覆盖，实际 %v", p3["status"])
	}

	// Arrays：端点按切片接收的字段，标量要包一层
	inv4 := &recInvoker{}
	if _, err := callCap(context.Background(), inv4.invoke, "archive_publish", map[string]any{"id": 12, "status": 1}); err != nil {
		t.Fatalf("archive_publish 调用失败: %v", err)
	}
	p4, _ := inv4.args["params"].(map[string]any)
	ids, ok := p4["ids"].([]any)
	if !ok || len(ids) != 1 || ids[0] != 12 {
		t.Errorf("archive_publish 的 id 应包成 ids 数组，实际 %#v", p4["ids"])
	}
	// 已是切片时不重复包裹
	inv5 := &recInvoker{}
	if _, err := callCap(context.Background(), inv5.invoke, "archive_publish", map[string]any{"ids": []any{12, 13}, "status": 1}); err != nil {
		t.Fatalf("archive_publish 调用失败: %v", err)
	}
	p5, _ := inv5.args["params"].(map[string]any)
	if got, ok := p5["ids"].([]any); !ok || len(got) != 2 {
		t.Errorf("已是切片的 ids 不应重复包裹，实际 %#v", p5["ids"])
	}
}

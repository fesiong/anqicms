package intent

import (
	"context"
	"encoding/json"
	"fmt"
	"reflect"
	"sort"
	"strings"
	"testing"
)

// marshalToJSONObject 复现 MCP 客户端的严格校验：
// vendored SDK 要求 CallToolResult.StructuredContent "must marshal to a JSON object"，
// 裸数组/标量会让客户端报 "expected record, received array"。
func marshalToJSONObject(t *testing.T, v any) map[string]any {
	t.Helper()
	b, err := json.Marshal(v)
	if err != nil {
		t.Fatalf("序列化失败: %v", err)
	}
	var m map[string]any
	if err := json.Unmarshal(b, &m); err != nil {
		t.Fatalf("StructuredContent 必须序列化为 JSON 对象，实际得到的首字节=%q 原文=%.120s",
			string(b[0]), string(b))
	}
	return m
}

// TestStructuredObjectAlwaysObject 锁住适配器边界的兜底：任何输入都不会产出非对象。
func TestStructuredObjectAlwaysObject(t *testing.T) {
	cases := []struct {
		name string
		in   any
	}{
		{"切片", []string{"a", "b"}},
		{"int切片", []int{1, 2}},
		{"map切片", []map[string]any{{"k": "v"}}},
		{"标量字符串", "hello"},
		{"标量数字", 42},
		{"布尔", true},
		{"字节切片", []byte("abc")},
	}
	for _, c := range cases {
		got := structuredObject(c.in)
		obj := marshalToJSONObject(t, got)
		if _, ok := obj["result"]; !ok {
			t.Fatalf("%s: 期望被包进 {\"result\":...}，实际=%v", c.name, obj)
		}
	}

	// 本来就是对象的必须原样透传，不能被多包一层
	// 注意：map 不可直接用 != 比较（会 panic），故统一用 reflect.DeepEqual
	orig := map[string]any{"id": 42}
	if got := structuredObject(orig); !reflect.DeepEqual(got, any(orig)) {
		t.Fatalf("对象应原样透传，实际=%#v", got)
	}
	type s struct{ A int }
	in := s{A: 1}
	if got := structuredObject(in); !reflect.DeepEqual(got, any(in)) {
		t.Fatalf("结构体应原样透传，实际=%#v", got)
	}
	if got := structuredObject(nil); got != nil {
		t.Fatalf("nil 应保持 nil，实际=%#v", got)
	}
}

// TestListIntentsPayloadIsObject 是本次线上问题的回归用例：
// mcp_list_intents 曾把意图数组裸塞进 StructuredContent 导致调用失败。
func TestListIntentsPayloadIsObject(t *testing.T) {
	k := NewKernel(Config{}, nil, nil)
	payload := k.listIntentsPayload()

	obj := marshalToJSONObject(t, payload)
	if _, ok := obj["intents"]; !ok {
		t.Fatalf("返回体应含 intents 字段: %v", obj)
	}
	intents, ok := obj["intents"].([]any)
	if !ok {
		t.Fatalf("intents 应为数组: %#v", obj["intents"])
	}
	if total, _ := obj["total"].(float64); int(total) != len(intents) {
		t.Fatalf("total=%v 与 intents 长度=%d 不一致", obj["total"], len(intents))
	}
	// 意图摘要字段完整，客户端可直接按 name 发起调用
	if len(intents) > 0 {
		first, _ := intents[0].(map[string]any)
		for _, field := range []string{"name", "title", "domain", "risk", "desc"} {
			if _, ok := first[field]; !ok {
				t.Fatalf("摘要缺少字段 %s: %v", field, first)
			}
		}
	}
}

// TestBuildOutputSchemaRejectsNonObject 防止切片/标量样例产出非 object 的 outputSchema
// （mcp.Server.AddTool 对非 object 的 outputSchema 会 panic，导致服务启动崩溃）。
func TestBuildOutputSchemaRejectsNonObject(t *testing.T) {
	if got := BuildOutputSchema([]map[string]any{{"a": "b"}}); got != nil {
		t.Fatalf("切片样例应降级为 nil，实际=%#v", got)
	}
	if got := BuildOutputSchema("scalar"); got != nil {
		t.Fatalf("标量样例应降级为 nil，实际=%#v", got)
	}
	type obj struct{ A int }
	sample := obj{A: 1}
	s := BuildOutputSchema(sample)
	if s == nil {
		t.Fatal("结构体样例应产出 object schema")
	}
	m, ok := s.(map[string]any)
	if !ok || m["type"] != "object" {
		t.Fatalf("结构体样例应产出 type=object，实际=%#v", s)
	}
}

// TestSwitchComposeIntentsReadAction 锁死「声明参数 == Compose 实际读取的键」这一契约。
//
// seo_statistics / system_site_info 曾把参数声明为 kind，而 switchCompose 读的是 action，
// 导致按 schema 传参会报「不支持的操作」。这里用 fakeCap 从行为上验证必须传 action。
func TestSwitchComposeIntentsReadAction(t *testing.T) {
	// 端点型能力已解析成真实端点，这里断言的是 action → 端点的对应关系。
	cases := []struct {
		intent       string
		action       string
		wantEndpoint string
	}{
		{"traffic_statistics", "dashboard", "GET /system/api/statistic/summary"},
		{"traffic_statistics", "spider", "GET /system/api/statistic/spider"},
		{"traffic_statistics", "traffic", "GET /system/api/statistic/traffic"},
		{"system_config", "site_info", "GET /system/api/siteinfo"},
		{"system_config", "site_version", "GET /system/api/version/info"},
		{"system_config", "site_anqi", "GET /system/api/anqi/info"},
	}
	for _, c := range cases {
		f := &fakeCap{}
		k := NewKernel(Config{}, f.invoker(), nil)
		if _, err := k.Execute(context.Background(), c.intent, fmt.Sprintf(`{"action":%q}`, c.action)); err != nil {
			t.Fatalf("%s action=%s 执行失败（参数名错配的典型症状）: %v", c.intent, c.action, err)
		}
		if got := f.lastEndpoint(); got != c.wantEndpoint {
			t.Fatalf("%s action=%s 应路由到 %s，实际=%q", c.intent, c.action, c.wantEndpoint, got)
		}
	}
}

// TestSwitchComposeIntentsDeclareActionInSchema 保证对外暴露的 input schema 里
// 确实有 action（而非历史上写错的 kind），且 Required 与 Params 自洽。
func TestSwitchComposeIntentsDeclareActionInSchema(t *testing.T) {
	for _, name := range []string{"traffic_statistics", "system_config"} {
		spec := mustSpec(t, name)
		props := schemaPropsOf(spec)
		if _, ok := props["action"]; !ok {
			t.Fatalf("%s 的 schema 缺少 action，实际=%v", name, keysOfProps(props))
		}
		if _, ok := props["kind"]; ok {
			t.Fatalf("%s 的 schema 仍残留 kind（应为 action）", name)
		}
		for _, r := range spec.Required {
			if _, ok := props[r]; !ok {
				t.Fatalf("%s: Required 里的 %q 未在 Params 声明", name, r)
			}
		}
	}
}

// TestAgentSkillAndTaskParamsMatchCaps 对齐 agent_skill / agent_task 与底层 cap 的参数名：
// skill_search 收 query/limit、skill_install 收 slug/force、task 收 tasks。
// 历史上分别误写为 keyword/name 与 type/args，会造成静默空结果或执行失败。
func TestAgentSkillAndTaskParamsMatchCaps(t *testing.T) {
	// 合并后的 agent 意图覆盖原 agent_skill / agent_task 的参数：
	// skill_search/skill_install 收 query/limit/slug/force，task 收 tasks。
	props := schemaPropsOf(mustSpec(t, "agent"))
	for _, want := range []string{"query", "limit", "slug", "force", "tasks"} {
		if _, ok := props[want]; !ok {
			t.Fatalf("agent schema 缺少 %s，实际=%v", want, keysOfProps(props))
		}
	}
	for _, stale := range []string{"keyword", "name", "type", "args"} {
		if _, ok := props[stale]; ok {
			t.Fatalf("agent schema 仍残留 %s（cap 并不识别）", stale)
		}
	}
	if req := mustSpec(t, "agent").Required; len(req) < 1 || req[0] != "action" {
		t.Fatalf("agent Required 应以 action 开头，实际=%v", req)
	}
}

// TestListIntentsTextFallbackHasDetails 保证文本 fallback 不是只有一句计数：
// 部分宿主会因 structuredContent 校验失败回退到 Content，此时必须仍能拿到意图明细。
func TestListIntentsTextFallbackHasDetails(t *testing.T) {
	k := NewKernel(Config{}, nil, nil)
	txt := textifyIntents(k.listIntentsPayload())
	for _, name := range []string{"traffic_statistics", "system_config"} {
		if !strings.Contains(txt, name) {
			t.Fatalf("文本 fallback 未包含意图 %s: %s", name, txt)
		}
	}
}

// mustSpec 复用 kernel_test.go 里已有的 specByName，查不到直接中断用例。
func mustSpec(t *testing.T, name string) *IntentSpec {
	t.Helper()
	spec, ok := specByName(name)
	if !ok {
		t.Fatalf("未找到意图 %s", name)
	}
	return spec
}

func schemaPropsOf(s *IntentSpec) map[string]any {
	props, _ := BuildInputSchema(s.Params, s.Required)["properties"].(map[string]any)
	return props
}

func keysOfProps(m map[string]any) []string {
	out := make([]string, 0, len(m))
	for k := range m {
		out = append(out, k)
	}
	sort.Strings(out)
	return out
}

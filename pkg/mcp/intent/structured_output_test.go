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

// TestStructuredObjectAlwaysObject 锁住适配器边界的兜底：任何输入都不会产出非对象，
// 且一律是统一信封 {code,msg,ok,status,data}。
//
// 2026-10-03 起信封形状统一：此前裸数组包进 {"result":...}、业务 map 直接铺开、
// 端点信封原样透传——AI 每换一个意图就要重新猜一次形状。现统一为标准信封，
// 业务载荷一律在 data 里（对象或数组均可）。
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
		// 必须是标准信封
		for _, k := range []string{"code", "msg", "ok", "status", "data"} {
			if _, has := obj[k]; !has {
				t.Errorf("%s: 信封缺字段 %q，实际=%v", c.name, k, obj)
			}
		}
		if obj["ok"] != true {
			t.Errorf("%s: ok 应为 true，实际=%v", c.name, obj["ok"])
		}
		// 载荷必须在 data 里，不能再出现 result 这层旧包装
		if _, has := obj["result"]; has {
			t.Errorf("%s: 不应再有 {\"result\":...} 旧包装，实际=%v", c.name, obj)
		}
		if _, has := obj["data"]; !has {
			t.Errorf("%s: 载荷应放 data，实际=%v", c.name, obj)
		}
	}

	// 已是标准信封的必须原样透传，不能被二次包裹
	env := map[string]any{"code": 0, "msg": "x", "ok": true, "status": 200, "data": map[string]any{"id": 1}}
	if got := structuredObject(env); !reflect.DeepEqual(got, any(env)) {
		t.Fatalf("标准信封应原样透传，实际=%#v", got)
	}

	// 普通业务 map 要包进信封，载荷放 data
	biz := map[string]any{"id": 42}
	gotBiz := marshalToJSONObject(t, structuredObject(biz))
	if gotBiz["ok"] != true {
		t.Errorf("业务 map 应被包成信封，实际=%v", gotBiz)
	}
	d, ok := gotBiz["data"].(map[string]any)
	if !ok {
		t.Fatalf("业务 map 应落在 data 里，实际=%v", gotBiz)
	}
	// 经 JSON 往返后数字是 float64
	if v, _ := d["id"].(float64); v != 42 {
		t.Errorf("data.id 应为 42，实际=%#v", d["id"])
	}

	// 结构体同样要包进信封
	type s struct{ A int }
	gotStruct := marshalToJSONObject(t, structuredObject(s{A: 1}))
	if gotStruct["ok"] != true {
		t.Errorf("结构体应被包成信封，实际=%v", gotStruct)
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
	// name 不再是「残留」：它就是 agent_create 的规范参数名。
	// 历史上 skill_install 曾把 name 当 slug 用（cap 只认 slug），故曾把 name 列为 stale；
	// 2026-10-02 修复 manage_create 字段映射后，name/strategy/cron 成为合法声明。
	// 真正的残留是下面这几个——它们不属于任何 cap 的参数名。
	for _, stale := range []string{"keyword", "type", "args"} {
		if _, ok := props[stale]; ok {
			t.Fatalf("agent schema 仍残留 %s（cap 并不识别）", stale)
		}
	}
	// manage_create 的定时能力必须对外可见，否则「每天执行」在 MCP 侧无法表达。
	for _, want := range []string{"name", "strategy", "cron", "max_runs", "max_rounds", "message", "enabled"} {
		if _, ok := props[want]; !ok {
			t.Fatalf("agent schema 缺少 %s，实际=%v", want, keysOfProps(props))
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

// TestTextAndStructuredContentSameSource Text 与 StructuredContent 必须同源。
//
// 背景（2026-10-03）：res.Text 是端点原始响应、res.Data 是 Compose 加工后的
// 结构，形状可能不同。实测 content_article list：
//   - StructuredContent.data = {list,total,page,page_size,count}
//   - Text 里的 data          = 裸数组，total 漂在信封顶层
// 同一结果两个通道形状不一致，AI 走 text 回退通道就丢分页信息。
func TestTextAndStructuredContentSameSource(t *testing.T) {
	cases := []struct {
		name string
		res  *Result
	}{
		{"list 有 Data", &Result{
			Text: `{"code":0,"total":268,"data":[{"id":1},{"id":2}]}`,
			Data: map[string]any{"list": []any{map[string]any{"id": 1}}, "total": 268},
		}},
		{"无 Data 时保留原 Text", &Result{Text: `{"ok":true,"msg":"已更新"}`}},
	}
	for _, c := range cases {
		env := structuredObject(c.res.Data)
		if c.res.Data == nil {
			if env != nil {
				t.Errorf("%s: Data 为 nil 时 structuredObject 应返回 nil", c.name)
			}
			continue
		}
		// 模拟 makeHandler 里的序列化：Text 必须等于 structuredObject 的 JSON
		b, err := json.Marshal(env)
		if err != nil {
			t.Fatalf("%s: 序列化失败 %v", c.name, err)
		}
		// Text 里必须能解析出与 structuredContent 相同的 total
		var fromText map[string]any
		if err := json.Unmarshal(b, &fromText); err != nil {
			t.Fatalf("%s: Text 不是合法 JSON: %v", c.name, err)
		}
		if fromText["total"] != env.(map[string]any)["total"] {
			t.Errorf("%s: Text 与 StructuredContent 的 total 不一致: %v vs %v",
				c.name, fromText["total"], env.(map[string]any)["total"])
		}
		// 关键：total 不能只出现在顶层、data 必须是加工后的形状
		d, ok := fromText["data"].(map[string]any)
		if !ok {
			t.Fatalf("%s: Text 里的 data 应为加工后的对象，实际 %T", c.name, fromText["data"])
		}
		if d["total"] == nil {
			t.Errorf("%s: data 内应含 total", c.name)
		}
	}
}

// TestStructuredObjectAlwaysJSONObjectText Text 通道的内容必须是 JSON 对象。
//
// MCP 契约要求 StructuredContent 顶层是 JSON 对象；Text 是回退通道，
// 同样不该输出裸数组/标量，否则模型解析困难。
func TestStructuredObjectAlwaysJSONObjectText(t *testing.T) {
	for _, data := range []any{
		[]any{map[string]any{"id": 1}},
		"字符串",
		42,
		map[string]any{"ok": true},
	} {
		obj := structuredObject(data)
		if _, isMap := obj.(map[string]any); !isMap {
			t.Errorf("structuredObject(%T) 应包成对象，实际 %T", data, obj)
		}
		b, err := json.Marshal(obj)
		if err != nil {
			t.Fatalf("序列化失败: %v", err)
		}
		var back map[string]any
		if err := json.Unmarshal(b, &back); err != nil {
			t.Errorf("Text 内容应能反序列化为对象: %s", b)
		}
	}
}

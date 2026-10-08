package intent

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"
	"testing"

	"github.com/modelcontextprotocol/go-sdk/mcp"
)

func textOf(r *mcp.CallToolResult) string {
	for _, c := range r.Content {
		if t, ok := c.(*mcp.TextContent); ok {
			return t.Text
		}
	}
	return ""
}

// fakeCap 记录调用并返回可预测的响应，用于验证组合逻辑。
//
// 端点型能力现在经 cap_routes.go 解析成真实的 method+path 后交给 api_invoke，
// 因此桩要按"端点"给响应，断言也应落在端点上 —— 断言能力名已经没有意义，
// 因为能力名在这一层已经被解析掉了。
type fakeCap struct {
	calls []string
	args  []map[string]any
}

func (f *fakeCap) invoker() CapInvoker {
	return func(ctx context.Context, name string, args map[string]any) (string, error) {
		f.calls = append(f.calls, name)
		f.args = append(f.args, args)
		if name != "api_invoke" {
			// 无 REST 等价端点的能力仍按能力名回落
			return "ok", nil
		}
		method, _ := args["method"].(string)
		path, _ := args["path"].(string)
		params, _ := args["params"].(map[string]any)
		switch method + " " + path {
		case "POST /system/api/archive/detail":
			if id := toInt64(params["id"]); id > 0 {
				return fmt.Sprintf("文档已更新！ID: %d", id), nil
			}
			// 模拟真实 api_invoke 的双层 JSON 包络：api_invoke 包一层，
			// 控制器 ArchiveDetailForm 再包一层，文档实体在 data.data。
			return `{"ok":true,"code":0,"msg":"","data":{"code":0,"msg":"","data":{"id":42,"title":"测试文章","link":"/a/42.html","status":"ok"}}}`, nil
		case "GET /system/api/setting/system":
			return "系统设置：\n站点名称: demo", nil
		case "POST /system/api/setting/system":
			b, _ := json.Marshal(params)
			return "系统设置已更新，收到: " + string(b), nil
		default:
			return "ok", nil
		}
	}
}

// lastEndpoint 返回最后一次调用的目标："METHOD PATH"（端点调用）
// 或能力名（无端点的能力回落）。
func (f *fakeCap) lastEndpoint() string {
	if len(f.args) == 0 {
		return ""
	}
	a := f.args[len(f.args)-1]
	if m, ok := a["method"].(string); ok {
		p, _ := a["path"].(string)
		return m + " " + p
	}
	return f.calls[len(f.calls)-1]
}

func TestContentSaveArticleCreate(t *testing.T) {
	f := &fakeCap{}
	k := NewKernel(Config{}, f.invoker(), nil)
	got, err := k.Dispatch(context.Background(), "content_article",
		`{"action":"save","title":"测试文章","content":"正文","category_id":1}`)
	if err != nil {
		t.Fatalf("unexpected err: %v", err)
	}
	if !strings.Contains(textOf(got), `"id":42`) {
		t.Fatalf("unexpected text: %s", textOf(got))
	}
	env, ok := got.StructuredContent.(map[string]any)
	if !ok {
		t.Fatalf("expected structured content, got %#v", got.StructuredContent)
	}
	// 2026-10-03 起统一信封 {code,msg,ok,status,data}，业务载荷一律在 data 里。
	if env["ok"] != true {
		t.Fatalf("信封 ok 应为 true，实际=%v", env)
	}
	data, ok := env["data"].(map[string]any)
	if !ok {
		t.Fatalf("信封 data 应为对象，实际=%#v", env["data"])
	}
	if data["id"] != int64(42) {
		t.Fatalf("expected id 42, got %v", data["id"])
	}
	// 新建回执应回填标题/链接/状态（此前因 extractID 只认大写
	// "ID:" 而对 JSON 包络失效，新建 id 恒为 0）。
	if data["title"] != "测试文章" {
		t.Fatalf("expected title 测试文章, got %v", data["title"])
	}
	if data["url"] != "/a/42.html" {
		t.Fatalf("expected url /a/42.html, got %v", data["url"])
	}
	if data["status"] != "ok" {
		t.Fatalf("expected status ok, got %v", data["status"])
	}
	// 新建与更新共用同一个端点，靠 id 是否为空区分
	if got := f.lastEndpoint(); got != "POST /system/api/archive/detail" {
		t.Fatalf("应路由到 POST /system/api/archive/detail，实际 %q", got)
	}
}

func TestContentSaveArticleUpdate(t *testing.T) {
	f := &fakeCap{}
	k := NewKernel(Config{}, f.invoker(), nil)
	got, err := k.Dispatch(context.Background(), "content_article",
		`{"action":"save","id":5,"title":"改","content":"新"}`)
	if err != nil {
		t.Fatalf("unexpected err: %v", err)
	}
	if ep := f.lastEndpoint(); ep != "POST /system/api/archive/detail" {
		t.Fatalf("应路由到 POST /system/api/archive/detail，实际 %q", ep)
	}
	params := f.args[len(f.args)-1]["params"].(map[string]any)
	if id := toInt64(params["id"]); id != 5 {
		t.Fatalf("端点参数应带上 id=5，实际 %v", params["id"])
	}
	env := got.StructuredContent.(map[string]any)
	data, ok := env["data"].(map[string]any)
	if !ok {
		t.Fatalf("信封 data 应为对象，实际=%#v", env["data"])
	}
	if data["id"] != int64(5) {
		t.Fatalf("expected id 5, got %v", data["id"])
	}
}

// TestExtractID 验证 extractID 能解析 api_invoke 的 JSON 包络（此前只认大写 "ID:"，
// 对 JSON 包络失效导致新建文档回执 id 恒为 0），并能在文本 handler 下回退正则。
func TestExtractID(t *testing.T) {
	// 双层 JSON 包络（真实 api_invoke 形态）：文档 id 在 data.data.id
	env := `{"ok":true,"code":0,"msg":"","data":{"code":0,"msg":"","data":{"id":123,"title":"x"}}}`
	if got := extractID(env); got != 123 {
		t.Fatalf("JSON 包络应解析出 id=123，实际 %d", got)
	}
	// 单包络
	single := `{"code":0,"msg":"","data":{"id":77}}`
	if got := extractID(single); got != 77 {
		t.Fatalf("单包络应解析出 id=77，实际 %d", got)
	}
	// 顶层 id
	if got := extractID(`{"id":9}`); got != 9 {
		t.Fatalf("顶层 id 应解析出 9，实际 %d", got)
	}
	// 文本 handler 回退正则（英文冒号）
	if got := extractID("附件上传成功！ID: 99"); got != 99 {
		t.Fatalf("文本应回退正则解析出 id=99，实际 %d", got)
	}
	// 文本 handler 回退正则（中文冒号）
	if got := extractID("结果：ID：55"); got != 55 {
		t.Fatalf("中文冒号应解析出 id=55，实际 %d", got)
	}
}

// TestIntentParamConsistency 验证意图参数与真实端点对齐：删除端点不支持的幽灵参数，
// 且列表筛选的 status 语义必须与端点（ok/draft/plan/delete）一致。
func TestIntentParamConsistency(t *testing.T) {
	// structure 不应声明端点不支持的 status 参数
	if spec, ok := SpecByName("structure"); ok {
		if _, has := spec.Params["status"]; has {
			t.Errorf("structure 声明了端点不支持的 status 参数（端点会静默忽略），应移除")
		}
		if _, has := spec.Params["type"]; has {
			t.Errorf("structure 声明了端点不支持的 type 参数（AnQiCMS 重定向无 type 字段），应移除")
		}
	}
	// content_article 的 status 语义必须与端点一致：ok/draft/plan/delete。
	// delete 是回收站（DeleteArchive 把正式文档移到 archive_drafts status=99），
	// 端点 ArchiveList 认这个值；少了它，AI 删完就再也找不到这篇文档、也无法确认。
	spec, _ := SpecByName("content_article")
	st := spec.Params["status"]
	wantEnum := []string{"ok", "draft", "plan", "delete"}
	if len(st.Enum) != len(wantEnum) {
		t.Fatalf("content_article.status 枚举应为 %v，实际 %v", wantEnum, st.Enum)
	}
	for i, w := range wantEnum {
		if st.Enum[i] != w {
			t.Errorf("content_article.status 枚举第 %d 项应为 %q，实际 %q（完整枚举 %v）",
				i, w, st.Enum[i], st.Enum)
		}
	}
	if _, has := spec.Params["order_by"]; !has {
		t.Errorf("content_article 应保留 order_by 参数（由 cap 层重命名为 sort）")
	}
	if _, has := spec.Params["order_dir"]; !has {
		t.Errorf("content_article 应保留 order_dir 参数（由 cap 层重命名为 order）")
	}
}

func TestSystemSettingReadWrite(t *testing.T) {
	f := &fakeCap{}
	k := NewKernel(Config{}, f.invoker(), nil)
	// 读取
	_, err := k.Dispatch(context.Background(), "system_config", `{"action":"setting","section":"system"}`)
	if err != nil {
		t.Fatalf("read err: %v", err)
	}
	if ep := f.lastEndpoint(); ep != "GET /system/api/setting/system" {
		t.Fatalf("读取应路由到 GET /system/api/setting/system，实际 %q", ep)
	}
	// 更新
	_, err = k.Dispatch(context.Background(), "system_config",
		`{"action":"setting","section":"system","values":{"site_name":"new"}}`)
	if err != nil {
		t.Fatalf("write err: %v", err)
	}
	if ep := f.lastEndpoint(); ep != "POST /system/api/setting/system" {
		t.Fatalf("写入应路由到 POST /system/api/setting/system，实际 %q", ep)
	}
}

func TestSwitchUnknownAction(t *testing.T) {
	f := &fakeCap{}
	k := NewKernel(Config{}, f.invoker(), nil)
	_, err := k.Dispatch(context.Background(), "content_manage", `{"action":"explode"}`)
	if err == nil {
		t.Fatal("expected error for unknown action")
	}
	if !strings.Contains(err.Error(), "explode") {
		t.Fatalf("error should mention action: %v", err)
	}
}

func TestAllowedFiltering(t *testing.T) {
	f := &fakeCap{}
	// ExposedIntents 仅 content 域 → 其余域意图不暴露
	k := NewKernel(Config{ExposedIntents: []string{"content"}}, f.invoker(), nil)
	if _, ok := k.intents["content_article"]; !ok {
		t.Fatal("content intent should exist in catalog")
	}
	// allowed 判定
	if !k.allowed(k.intents["content_article"]) {
		t.Fatal("content intent should be allowed")
	}
	if k.allowed(k.intents["shell_exec"]) {
		t.Fatal("shell_exec should be filtered out by content-only scope")
	}
}

func TestDomainHelpers(t *testing.T) {
	if len(ByDomain(nil, DomainContent)) != 0 {
		t.Fatal("empty specs should yield no names")
	}
	if len(ByMaxRisk(nil, RiskSystem)) != 0 {
		t.Fatal("empty specs should yield no names")
	}
	if DomainLabel(DomainSeo) == "" {
		t.Fatal("domain label should not be empty")
	}
}

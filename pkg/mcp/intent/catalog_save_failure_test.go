package intent

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"
	"testing"
)

// TestEndpointFailureDetectsBusinessError 端点业务失败不是 Go error，
// 必须从 JSON 包络里识别出来，否则 Compose 会把失败报成成功。
func TestEndpointFailureDetectsBusinessError(t *testing.T) {
	cases := []struct {
		name string
		in   string
		want string // 空表示不应判为失败
	}{
		{"外层 ok=false", `{"ok":false,"code":-1,"msg":"未定义模型","data":null}`, "未定义模型"},
		{"双层信封 code 非 0", `{"ok":true,"data":{"code":-1,"msg":"record not found","data":null}}`, "record not found"},
		{"双层信封 code=0 视为成功", `{"ok":true,"data":{"code":0,"msg":"","data":{"id":1847}}}`, ""},
		{"成功无 ok 字段", `{"data":{"id":1}}`, ""},
		{"非 JSON", `plain text`, ""},
		{"空串", ``, ""},
		{"ok=true 不算失败", `{"ok":true,"msg":"","data":{}}`, ""},
	}
	for _, c := range cases {
		got := endpointFailure(c.in)
		if c.want == "" {
			if got != "" {
				t.Errorf("%s: 不应判为失败，却得到 %q", c.name, got)
			}
			continue
		}
		if !strings.Contains(got, c.want) {
			t.Errorf("%s: 期望包含 %q，实际 %q", c.name, c.want, got)
		}
	}
}

// TestContentArticleSaveReportsFailure 端点失败时 save 必须返回 error，
// 而不是在 StructuredContent 里给 {"ok":true,"data":{"id":N}}。
//
// 实测踩过（2026-10-03）：更新 module_id 为空的文章，端点回「未定义模型」且
// 什么都没改，但回执是 ok=true + id=1847，AI 会据此认为保存成功。
func TestContentArticleSaveReportsFailure(t *testing.T) {
	failed := `{"ok":false,"code":-1,"msg":"未定义模型","data":null,"status":200}`
	inv := func(_ context.Context, name string, m map[string]any) (string, error) {
		// archive_update 在 capEndpoints 里有 REST 等价物（POST /archive/detail），
		// callCap 会把它展开成 api_invoke，所以断言落在 api_invoke 上。
		if name != "api_invoke" {
			t.Errorf("应经 api_invoke 落到 POST /archive/detail，实际 %q", name)
		}
		if m["path"] != "/system/api/archive/detail" {
			t.Errorf("path = %v，期望 /system/api/archive/detail", m["path"])
		}
		return failed, nil
	}
	spec := mustSpec(t, "content_article")
	res, err := spec.Compose(context.Background(),
		map[string]any{"action": "save", "id": 1847, "title": "x", "content": "y"}, inv)
	if err == nil {
		t.Fatalf("端点失败时 save 应返回 error，实际 res=%+v", res)
	}
	if !strings.Contains(err.Error(), "未定义模型") {
		t.Errorf("错误信息应含端点原因，实际=%v", err)
	}
	if !strings.Contains(err.Error(), "未做任何修改") {
		t.Errorf("错误信息应说明未做修改，避免 AI 重试造成混乱，实际=%v", err)
	}
}

// TestContentArticleSaveSucceeds 端点成功时必须正常返回 id 与 link，
// 防止修复把正常路径一起拦掉。
func TestContentArticleSaveSucceeds(t *testing.T) {
	ok := `{"ok":true,"data":{"code":0,"msg":"","data":{"id":1847,"title":"T","link":"http://h/zixun/1847.html"}}}`
	inv := func(_ context.Context, _ string, _ map[string]any) (string, error) { return ok, nil }
	spec := mustSpec(t, "content_article")
	res, err := spec.Compose(context.Background(),
		map[string]any{"action": "save", "id": 1847, "title": "T", "content": "c"}, inv)
	if err != nil {
		t.Fatalf("成功路径不应报错：%v", err)
	}
	d, _ := res.Data.(map[string]any)
	if d == nil {
		t.Fatalf("Data 应为 map，实际 %T", res.Data)
	}
	if d["id"] != int64(1847) {
		t.Errorf("id = %v，期望 1847", d["id"])
	}
	if d["link"] == nil || d["link"] == "" {
		t.Error("成功时应回填 link")
	}
}

// TestArticleStatusCodeNormalize publish 的 status 要转成端点的 uint。
//
// 端点是 ArchiveStatusRequest.Status uint（0=草稿 1=正式），意图层对外是枚举字符串，
// 实测直接透传会报「cannot unmarshal string into ... of type uint」，publish 完全不可用。
func TestArticleStatusCodeNormalize(t *testing.T) {
	cases := []struct {
		in   any
		want int64
	}{
		{"ok", 1}, {"OK", 1}, {"1", 1}, {"publish", 1},
		{"draft", 0}, {"plan", 0}, {"", 0}, {"未知", 0},
		{float64(1), 1}, {float64(0), 0}, {1, 1},
		{true, 0}, {false, 1},
	}
	for _, c := range cases {
		got := articleStatusCode(c.in)
		if g, ok := got.(int64); !ok || g != c.want {
			t.Errorf("articleStatusCode(%#v) = %#v，期望 int64(%d)", c.in, got, c.want)
		}
	}
	if got := articleStatusCode(nil); got != nil {
		t.Errorf("nil 应原样返回，实际 %#v", got)
	}
}

// TestContentArticlePublishConvertsParams publish 必须把 status 转 uint、id 转 ids 数组。
func TestContentArticlePublishConvertsParams(t *testing.T) {
	var gotParams map[string]any
	// archive_publish 在 capEndpoints 里有 REST 等价物（POST /archive/status），
	// callCap 展开成 api_invoke，参数包在 params 里；id→ids 由 Rename/Arrays 处理。
	inv := func(_ context.Context, _ string, m map[string]any) (string, error) {
		if p, ok := m["params"].(map[string]any); ok {
			gotParams = p
		} else {
			gotParams = m
		}
		return `{"ok":true,"msg":"文章已更新","data":null}`, nil
	}
	spec := mustSpec(t, "content_article")
	if _, err := spec.Compose(context.Background(),
		map[string]any{"action": "publish", "id": 1847, "status": "draft"}, inv); err != nil {
		t.Fatalf("publish 应成功：%v", err)
	}
	if gotParams["status"] != int64(0) {
		t.Errorf("status 应为 int64(0)，实际 %#v", gotParams["status"])
	}
	// id 由 capEndpoints 的 Rename 换算成 ids，并由 Arrays 包成数组
	ids, ok := gotParams["ids"].([]any)
	if !ok || len(ids) != 1 {
		t.Fatalf("ids 应为单元素数组，实际 %#v", gotParams["ids"])
	}
	// 不绑死元素类型：意图参数可能是 int 或 int64，序列化时都会正确发出 1847。
	if fmt.Sprintf("%v", ids[0]) != "1847" {
		t.Errorf("ids[0] 应为 1847，实际 %#v（类型 %T）", ids[0], ids[0])
	}
}

// TestParseScopeDomains mcp_set_scope 的 domains 要兼容数组与字符串两种形态，
// 且无法识别时报错而不是静默回退成「全部」。
//
// 实测踩过：传 "content"（字符串）时旧实现 Unmarshal 失败即忽略，回「已应用：全部」，
// 调用方以为设置生效，实际 scope 完全没变。
func TestParseScopeDomains(t *testing.T) {
	cases := []struct {
		in      string
		want    []string
		wantErr bool
	}{
		{`["content","seo"]`, []string{"content", "seo"}, false},
		{`"content"`, []string{"content"}, false},
		{`"content,seo"`, []string{"content", "seo"}, false},
		{`" content , seo "`, []string{"content", "seo"}, false},
		{`[]`, nil, false},
		{`""`, nil, false},
		{`null`, nil, false},
		{``, nil, false},
		{`123`, nil, true},
		{`{"a":1}`, nil, true},
	}
	for _, c := range cases {
		got, err := parseScopeDomains(json.RawMessage(c.in))
		if c.wantErr {
			if err == nil {
				t.Errorf("parseScopeDomains(%s) 应报错，实际得到 %v", c.in, got)
			}
			continue
		}
		if err != nil {
			t.Errorf("parseScopeDomains(%s) 不应报错：%v", c.in, err)
			continue
		}
		if len(got) != len(c.want) {
			t.Errorf("parseScopeDomains(%s) = %v，期望 %v", c.in, got, c.want)
			continue
		}
		for i := range got {
			if got[i] != c.want[i] {
				t.Errorf("parseScopeDomains(%s) = %v，期望 %v", c.in, got, c.want)
				break
			}
		}
	}
}

// TestEndpointFailureDetectsHTTP5xx 控制器没写响应体时 iris 返回默认 500，
// api_invoke 会表达成 ok=true + status=500 + msg 空 —— 必须识别成失败。
func TestEndpointFailureDetectsHTTP5xx(t *testing.T) {
	got := endpointFailure(`{"code":0,"data":null,"hint":"...","msg":"","ok":true,"status":500}`)
	if got == "" {
		t.Fatal("status=500 应判为失败")
	}
	if !strings.Contains(got, "500") {
		t.Errorf("错误信息应含状态码，实际=%q", got)
	}
	// 有 msg 时优先用 msg
	got2 := endpointFailure(`{"ok":true,"status":503,"msg":"服务不可用"}`)
	if got2 != "服务不可用" {
		t.Errorf("应优先返回 msg，实际=%q", got2)
	}
	// 正常 200 不算失败
	if s := endpointFailure(`{"ok":true,"status":200,"msg":"Sitemap已更新"}`); s != "" {
		t.Errorf("status=200 不应判为失败，实际=%q", s)
	}
}

// TestSeoSitemapSuppliesDefaultType seo action=sitemap 不传参数时端点会 500
// （type 空拼出 /sitemap. 畸形路径），意图层要补默认 type=xml。
func TestSeoSitemapSuppliesDefaultType(t *testing.T) {
	var got []map[string]any
	inv := func(_ context.Context, name string, m map[string]any) (string, error) {
		if p, ok := m["params"].(map[string]any); ok {
			got = append(got, p)
			return `{"ok":true,"msg":"Sitemap已更新","data":{}}`, nil
		}
		return `{"ok":true,"msg":"推送URL成功","data":null}`, nil
	}
	spec := mustSpec(t, "seo")
	if _, err := spec.Compose(context.Background(), map[string]any{"action": "sitemap"}, inv); err != nil {
		t.Fatalf("sitemap 不应报错：%v", err)
	}
	if len(got) == 0 || got[0]["type"] != "xml" {
		t.Fatalf("应给 sitemap_rebuild 传 type=xml，实际=%v", got)
	}
	// 端点报 500 时必须转成 error，不能静默
	inv500 := func(_ context.Context, _ string, _ map[string]any) (string, error) {
		return `{"ok":true,"msg":"","status":500}`, nil
	}
	if _, err := spec.Compose(context.Background(), map[string]any{"action": "sitemap"}, inv500); err == nil {
		t.Error("端点 500 时 sitemap 应返回 error")
	}
}

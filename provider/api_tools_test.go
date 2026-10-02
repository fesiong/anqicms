package provider

import (
	"context"
	"fmt"
	"strings"
	"testing"

	"kandaoni.com/anqicms/pkg/ai/eino"
)

func newAPITestService() *AiChatService {
	svc := testService()
	svc.capHandlers = map[string]toolHandler{}
	for n, h := range svc.Handlers {
		svc.capHandlers[n] = h
	}
	svc.capHandlers[capAPIList] = svc.capAPIList
	svc.capHandlers[capAPISchema] = svc.capAPISchema
	svc.capHandlers[capAPIInvoke] = svc.capAPIInvoke
	// 测试基线：全开放模式（硬规则仍然生效）。
	// 默认策略是 fail closed（mode=off 全拒），不设基线则 api_list 恒为空。
	all := eino.ApiExposureConfig{Mode: ExposureModeAll}
	SetApiExposureOverride(&all)
	return svc
}

// TestCapAPIList 校验端点检索与过滤能力。
func TestCapAPIList(t *testing.T) {
	svc := newAPITestService()
	ctx := context.Background()

	out, err := svc.capAPIList(ctx, `{"ns":"plugin","limit":5}`)
	if err != nil {
		t.Fatal(err)
	}
	for _, want := range []string{`"matched"`, `"returned":5`, `"namespaces"`} {
		if !strings.Contains(out, want) {
			t.Fatalf("api_list 输出缺少 %s\n%s", want, out)
		}
	}

	// 关键词过滤应命中具体端点
	out2, err := svc.capAPIList(ctx, `{"keyword":"favicon"}`)
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(out2, "/system/api/setting/favicon") {
		t.Fatalf("关键词过滤未命中 favicon 端点:\n%s", out2)
	}

	// 上传类端点过滤
	out3, err := svc.capAPIList(ctx, `{"source":"multipart","limit":3}`)
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(out3, "attachment/upload") {
		t.Fatalf("multipart 过滤未命中上传端点:\n%s", out3)
	}
}

// TestCapAPIListByDomain 校验按能力域过滤。
//
// 这条能力的价值在于：/plugin/* 一个 ns 装了 229 个端点，按 ns 过滤等于没过滤；
// 只有按 domain 才能一次收敛到"备份缓存"或"内容生产"这类可决策的范围。
func TestCapAPIListByDomain(t *testing.T) {
	svc := newAPITestService()
	ctx := context.Background()

	out, err := svc.capAPIList(ctx, `{"domain":"siteops","limit":50,"only_allowed":false}`)
	if err != nil {
		t.Fatal(err)
	}
	// 域过滤必须精确：结果里不该出现别的域的路径
	for _, path := range []string{"/system/api/plugin/backup", "/system/api/plugin/htmlcache"} {
		if !strings.Contains(out, path) {
			t.Fatalf("siteops 域过滤未包含 %s:\n%s", path, out)
		}
	}
	if strings.Contains(out, "/system/api/archive/list") {
		t.Fatalf("siteops 域过滤混入了内容域端点:\n%s", out)
	}
	// 输出应带域分布统计
	if !strings.Contains(out, `"domains"`) {
		t.Fatalf("api_list 输出缺少 domains 统计:\n%s", out)
	}

	// 域统计里 siteops 的计数应与过滤结果一致（50 条上限内）
	if !strings.Contains(out, `"domain":"siteops"`) {
		t.Fatalf("domains 统计缺少 siteops:\n%s", out)
	}
}

// TestCapAPISchemaHasDomain 校验 schema 输出携带能力域，便于调用前判断该端点属哪一类。
func TestCapAPISchemaHasDomain(t *testing.T) {
	svc := newAPITestService()
	ctx := context.Background()

	out, err := svc.capAPISchema(ctx, `{"method":"GET","path":"/system/api/archive/list"}`)
	if err != nil {
		t.Fatal(err)
	}
	fmt.Printf("api_schema 输出:\n%s", out)
	if !strings.Contains(out, `"domain":"content"`) {
		t.Fatalf("api_schema 输出缺少 domain:\n%s", out)
	}
}

// TestCapAPISchema 校验参数定义返回，尤其上传端点的 base64 提示。
func TestCapAPISchema(t *testing.T) {
	svc := newAPITestService()
	ctx := context.Background()

	out, err := svc.capAPISchema(ctx, `{"method":"POST","path":"/archive/detail"}`)
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(out, "request.Archive") {
		t.Fatalf("应返回结构体类型:\n%s", out)
	}

	// 上传端点必须给出 file 字段与 base64 提示
	out2, err := svc.capAPISchema(ctx, `{"method":"POST","path":"/attachment/upload"}`)
	if err != nil {
		t.Fatal(err)
	}
	for _, want := range []string{`"name":"file"`, "category_id", "data URI", "base64"} {
		if !strings.Contains(out2, want) {
			t.Fatalf("上传端点 schema 缺少 %s\n%s", want, out2)
		}
	}

	if _, err := svc.capAPISchema(ctx, `{"path":"/nope/nothing"}`); err == nil {
		t.Fatal("未知端点应报错")
	}
}

// TestCapAPIInvokeRejectsUnknown 校验不存在路径不会被盲调。
func TestCapAPIInvokeRejectsUnknown(t *testing.T) {
	svc := newAPITestService()
	if _, err := svc.capAPIInvoke(context.Background(), `{"method":"GET","path":"/definitely/not/exist"}`); err == nil {
		t.Fatal("未登记的端点应被拒绝，不应盲调")
	}
}

// TestCapAPIInvokeBlocksCredentialEndpoints 校验认证类端点被硬拒绝。
func TestCapAPIInvokeBlocksCredentialEndpoints(t *testing.T) {
	svc := newAPITestService()
	cat, err := BuildAPICatalog()
	if err != nil {
		t.Fatal(err)
	}
	blocked := 0
	for _, ep := range cat.Endpoints {
		if reason := invokeBlockReason(ep); reason == "" {
			continue
		}
		blocked++
		if blocked > 3 {
			break
		}
		_, err := svc.capAPIInvoke(context.Background(), `{"method":"`+ep.Method+`","path":"`+ep.Path+`"}`)
		if err == nil {
			t.Fatalf("被禁止的命名空间端点竟被调用成功: %s", ep.Path)
		}
		if !strings.Contains(err.Error(), "不允许通用调用") {
			t.Fatalf("应以「不允许通用调用」拒绝，实际: %v", err)
		}
	}
	if blocked == 0 {
		t.Fatal("目录中应存在被硬编码拒绝的认证类端点")
	}
}

// TestResolveInvokeAdminIDNeverImplicitSuperAdmin 守住最关键的安全边界：
// 未配置身份时不得隐式使用 adminId==1（AdminPermission 对该账号直接放行）。
func TestResolveInvokeAdminIDNeverImplicitSuperAdmin(t *testing.T) {
	id, err := resolveInvokeAdminID(nil)
	if err == nil {
		t.Fatalf("site 为 nil 时应报错，实际返回 adminId=%d", id)
	}
	if id != 0 {
		t.Fatalf("未配置身份时不得返回非零 adminId，实际 %d", id)
	}
	if _, err := resolveInvokeAdminID(&Website{}); err == nil {
		t.Fatal("未配置 InvokeAdminId 时应拒绝，避免隐式使用超级管理员")
	}
}

// TestResolveCapPrefersCapHandlers 守住 E 阶段踩过的递归坑：
// 同名意图会覆盖同名 cap，因此委托必须优先取纯 cap 表。
func TestResolveCapPrefersCapHandlers(t *testing.T) {
	svc := &AiChatService{}
	real := func(context.Context, string) (string, error) { return "real-cap", nil }
	fake := func(context.Context, string) (string, error) { return "intent-self", nil }
	svc.capHandlers = map[string]toolHandler{"web_fetch": real}
	svc.Handlers = map[string]toolHandler{"web_fetch": fake} // 模拟意图覆盖同名 cap

	h, ok := svc.ResolveCap("web_fetch")
	if !ok {
		t.Fatal("应能解析出能力")
	}
	got, _ := h(context.Background(), "{}")
	if got != "real-cap" {
		t.Fatalf("应取到纯 cap 表里的 handler（否则会无限递归），实际 %s", got)
	}
}

// TestNormalizeAPIPath 校验路径补全，允许简写。
func TestNormalizeAPIPath(t *testing.T) {
	cases := map[string]string{
		"/system/api/archive/list": "/system/api/archive/list",
		"/archive/list":            "/system/api/archive/list",
		"archive/list":             "/system/api/archive/list",
		"":                         "/system/api",
	}
	for in, want := range cases {
		if got := normalizeAPIPath(in); got != want {
			t.Fatalf("normalizeAPIPath(%q)=%q want %q", in, got, want)
		}
	}
}

package provider

import (
	"encoding/json"
	"os"
	"strings"
	"testing"

	"kandaoni.com/anqicms/config"
)

// TestEmbeddedCatalogLoadable 验证编译进二进制的端点表可用。
//
// 这是「二进制部署」场景的地基：只要这张表在，api_list/api_schema/api_invoke
// 就不依赖任何 .go 源码。
func TestEmbeddedCatalogLoadable(t *testing.T) {
	ResetAPICatalogCache()
	cat, err := BuildAPICatalog()
	if err != nil {
		t.Fatalf("构建目录失败: %v", err)
	}
	if !cat.Embedded {
		t.Fatalf("应命中嵌入表，实际来源: %s", cat.CatalogSource())
	}
	if len(cat.Endpoints) == 0 {
		t.Fatal("嵌入表端点数为 0")
	}
	if cat.GeneratedAt == "" {
		t.Error("嵌入表缺少 generated_at，无法判断是否过期")
	}
	t.Logf("嵌入表：%d 个端点，来源 %s", len(cat.Endpoints), cat.CatalogSource())

	// 嵌入表里的每条卡片都应带域与风险标记，否则门禁判断会漏
	for _, ep := range cat.Endpoints {
		if ep.Method == "" || ep.Path == "" {
			t.Fatalf("卡片缺少 method/path: %+v", ep)
		}
		if ep.Domain == "" {
			t.Errorf("端点 %s %s 缺少 domain", ep.Method, ep.Path)
		}
		if ep.Risk == "" {
			t.Errorf("端点 %s %s 缺少 risk", ep.Method, ep.Path)
		}
	}
}

// TestCatalogAvailableWithoutSource 决定性验证：模拟只发布二进制、没有任何 .go 源码的环境。
//
// 做法是切到空临时目录并屏蔽 config.ExecPath —— 这正是生产部署的形态。
// 此用例一旦失败，意味着 api_list/api_schema/api_invoke 在服务器上全线不可用。
func TestCatalogAvailableWithoutSource(t *testing.T) {
	tmp := t.TempDir() // 空目录，向上 5 层也不可能有 route/manage.go
	oldWD, err := os.Getwd()
	if err != nil {
		t.Fatalf("取当前目录失败: %v", err)
	}
	oldExec := config.ExecPath
	defer func() {
		_ = os.Chdir(oldWD)
		config.ExecPath = oldExec
		ResetAPICatalogCache()
	}()

	if err := os.Chdir(tmp); err != nil {
		t.Fatalf("切换目录失败: %v", err)
	}
	config.ExecPath = tmp

	// 先确认这个环境里源码解析确实走不通，否则本用例没有模拟到位
	if _, serr := BuildAPICatalogFromSource(); serr == nil {
		t.Skipf("临时目录下仍能定位到源码，无法模拟二进制部署环境，跳过")
	} else {
		t.Logf("源码解析按预期失败: %v", serr)
	}

	ResetAPICatalogCache()
	cat, err := BuildAPICatalog()
	if err != nil {
		t.Fatalf("无源码环境下目录应来自嵌入表，实际失败: %v", err)
	}
	if !cat.Embedded {
		t.Fatalf("无源码环境下应命中嵌入表，实际: %s", cat.CatalogSource())
	}
	if len(cat.Endpoints) == 0 {
		t.Fatal("无源码环境下端点数为 0")
	}
	t.Logf("无源码环境下仍可加载 %d 个端点（%s）", len(cat.Endpoints), cat.CatalogSource())
}

// TestAPIMetaToolsWorkWithoutSource 验证 api_list / api_schema 在无源码环境下可用。
//
// api_invoke 需要真实站点与管理员身份，不在单测范围内；但它与这两个 cap 共用
// BuildAPICatalog，目录能加载即说明它的准入校验不会再因缺源码而拒执行。
func TestAPIMetaToolsWorkWithoutSource(t *testing.T) {
	tmp := t.TempDir()
	oldWD, err := os.Getwd()
	if err != nil {
		t.Fatalf("取当前目录失败: %v", err)
	}
	oldExec := config.ExecPath
	defer func() {
		_ = os.Chdir(oldWD)
		config.ExecPath = oldExec
		ResetAPICatalogCache()
	}()
	if err := os.Chdir(tmp); err != nil {
		t.Fatalf("切换目录失败: %v", err)
	}
	config.ExecPath = tmp
	ResetAPICatalogCache()

	svc := &AiChatService{}
	// only_allowed=false：默认策略是 fail closed（未配置白名单即全部拒绝），
	// 只看 allowed 端点会返回空集，掩盖"目录本身能不能加载"这个问题。
	list, err := svc.capAPIList(nil, `{"limit":3,"only_allowed":false}`)
	if err != nil {
		t.Fatalf("无源码环境下 api_list 失败: %v", err)
	}
	// 不写死端点总数：路由增删是常态，这里只要求目录完整可用。
	var listed struct {
		Total   int `json:"total"`
		Domains []struct {
			Domain string `json:"domain"`
			Count  int    `json:"count"`
		} `json:"domains"`
	}
	if err := json.Unmarshal([]byte(list), &listed); err != nil {
		t.Fatalf("api_list 返回不是合法 JSON: %v", err)
	}
	if listed.Total == 0 {
		t.Fatal("api_list 在无源码环境下返回 0 个端点")
	}
	if len(listed.Domains) == 0 {
		t.Fatal("api_list 未返回域分布统计")
	}
	schema, err := svc.capAPISchema(nil, `{"method":"GET","path":"/archive/list"}`)
	if err != nil {
		t.Fatalf("无源码环境下 api_schema 失败: %v", err)
	}
	if !strings.Contains(schema, `"params"`) {
		t.Fatalf("api_schema 未返回参数定义: %s", truncateCatalogMsg(schema, 300))
	}
	t.Logf("无源码环境：api_list 与 api_schema 均正常返回")
}

func truncateCatalogMsg(s string, n int) string {
	if len(s) <= n {
		return s
	}
	return s[:n] + "…"
}

// TestCatalogFreshness 检查嵌入表是否落后于源码。
//
// 手工编辑 api_catalog.json 是一等用例（补 desc、覆盖 risk），所以默认只报告不失败；
// 需要 CI 严格卡口时设 APICATALOG_STRICT=1。
func TestCatalogFreshness(t *testing.T) {
	ResetAPICatalogCache()
	embedded, err := BuildAPICatalog()
	if err != nil || !embedded.Embedded {
		t.Skipf("未使用嵌入表，跳过漂移检查 (%v)", err)
	}
	src, err := BuildAPICatalogFromSource()
	if err != nil {
		t.Skipf("当前环境无源码，无法比对 (%v)", err)
	}
	d := DiffCatalogs(embedded, src)
	if d.Empty() {
		t.Logf("端点表与源码一致（%d 个端点）", len(embedded.Endpoints))
		return
	}
	for _, k := range d.Missing {
		t.Logf("源码新增、表中缺失: %s", k)
	}
	for _, k := range d.Extra {
		t.Logf("表中有、源码已无（或手工补充）: %s", k)
	}
	for _, k := range d.Changed {
		t.Logf("元数据变化: %s", k)
	}
	if os.Getenv("APICATALOG_STRICT") == "1" {
		t.Fatalf("端点表已过期，请运行 `go run ./cmd/apicatalog` 重新生成"+
			"（若为有意的手工编辑，请取消 APICATALOG_STRICT）: missing=%d extra=%d changed=%d",
			len(d.Missing), len(d.Extra), len(d.Changed))
	}
	t.Logf("存在漂移但未开启严格模式（APICATALOG_STRICT=1 可转失败）: missing=%d extra=%d changed=%d",
		len(d.Missing), len(d.Extra), len(d.Changed))
}

package provider

import (
	"context"
	"strings"
	"testing"

	"kandaoni.com/anqicms/pkg/ai/eino"
)

// withExposure 在用例内临时注入策略，退出即恢复，避免污染同包其他用例。
func withExposure(t *testing.T, cfg eino.ApiExposureConfig) {
	t.Helper()
	prev := apiExposureOverride
	SetApiExposureOverride(&cfg)
	t.Cleanup(func() { SetApiExposureOverride(prev) })
}

// TestExposureDefaultIsFailClosed 守住最关键的默认行为：
// 未配置任何策略时必须全部拒绝，绝不能"没配就全开"。
func TestExposureDefaultIsFailClosed(t *testing.T) {
	withExposure(t, eino.ApiExposureConfig{})

	cat, err := BuildAPICatalog()
	if err != nil {
		t.Fatal(err)
	}
	for _, ep := range cat.Endpoints {
		if d := EvaluateExposure(CurrentApiExposure(), ep); d.Allowed {
			t.Fatalf("默认策略下端点不应被放行: %s %s", ep.Method, ep.Path)
		}
	}
	summary := ExposureSummary(CurrentApiExposure(), cat)
	if summary["allowed"] != 0 {
		t.Fatalf("默认策略应放行 0 个端点，实得 %v", summary["allowed"])
	}
	if summary["blocked"] != len(cat.Endpoints) {
		t.Fatalf("默认策略应拦截全部 %d 个端点，实得 %v", len(cat.Endpoints), summary["blocked"])
	}
	if summary["mode"] != ExposureModeOff {
		t.Fatalf("未配置时 mode 应归一化为 off，实得 %v", summary["mode"])
	}
	t.Logf("默认 fail closed：%d 个端点全部拒绝，其中硬规则 %v 个",
		summary["total"], summary["blocked_by_hard"])
}

// TestExposureModeRiskMatrix 校验 mode 与风险等级的对应关系：
// read 只放行读，read_write 拒绝删除，all 才放行 destructive。
func TestExposureModeRiskMatrix(t *testing.T) {
	eps := []EndpointMeta{
		{Method: "GET", Path: "/system/api/x/list", NS: "archive", Risk: "read"},
		{Method: "POST", Path: "/system/api/x/detail", NS: "archive", Risk: "write"},
		{Method: "POST", Path: "/system/api/x/delete", NS: "archive", Risk: "destructive"},
	}
	cases := []struct {
		mode string
		want []bool // read / write / destructive 是否放行
	}{
		{"", []bool{false, false, false}},
		{"off", []bool{false, false, false}},
		{"read", []bool{true, false, false}},
		{"read_write", []bool{true, true, false}},
		{"all", []bool{true, true, true}},
		{"READ", []bool{true, false, false}},          // 大小写无关
		{"unknown-mode", []bool{false, false, false}}, // 未知模式按 off
	}
	for _, c := range cases {
		withExposure(t, eino.ApiExposureConfig{Mode: c.mode})
		for i, ep := range eps {
			got := EvaluateExposure(CurrentApiExposure(), ep).Allowed
			if got != c.want[i] {
				t.Fatalf("mode=%s risk=%s 期望 allowed=%v，实得 %v", c.mode, ep.Risk, c.want[i], got)
			}
		}
	}
	t.Log("mode × risk 矩阵全部符合预期（含未知模式按 off）")
}

// TestExposureHardRulesCannotBeConfiguredAway 校验硬规则不可被配置解除。
// 这是有意的设计：凭证流程与未修复的鉴权缺陷属于代码级风险，不是偏好问题。
func TestExposureHardRulesCannotBeConfiguredAway(t *testing.T) {
	// 即使配成 all + 白名单包含这些命名空间，也必须拒绝
	withExposure(t, eino.ApiExposureConfig{
		Mode:    ExposureModeAll,
		AllowNS: []string{"login", "captcha", "password", "aigenerate", "admin"},
	})
	for _, ep := range []EndpointMeta{
		{Method: "POST", Path: "/system/api/login", NS: "login", Risk: "write"},
		{Method: "GET", Path: "/system/api/captcha", NS: "captcha", Risk: "read"},
		{Method: "POST", Path: "/system/api/password/reset", NS: "password", Risk: "write"},
		// VA-012：该模块完全免登录且会泄露 AI 密钥，修复前一律拒绝
		{Method: "GET", Path: "/system/api/aigenerate/setting", NS: "aigenerate", Risk: "read"},
		// 管理员变更是提权路径：读可以，写不行
		{Method: "POST", Path: "/system/api/admin/detail", NS: "admin", Risk: "write"},
	} {
		d := EvaluateExposure(CurrentApiExposure(), ep)
		if d.Allowed {
			t.Fatalf("硬规则端点不应被配置解除: %s %s", ep.Method, ep.Path)
		}
		if d.Stage != "hard_block" {
			t.Fatalf("应以 hard_block 拒绝，实得 stage=%s", d.Stage)
		}
	}
	// 管理员只读应当放行
	read := EndpointMeta{Method: "GET", Path: "/system/api/admin/list", NS: "admin", Risk: "read"}
	if d := EvaluateExposure(CurrentApiExposure(), read); !d.Allowed {
		t.Fatalf("管理员只读端点应放行，实际被拦: %s", d.Reason)
	}
	t.Log("硬规则不可配置解除；管理员命名空间仅放行只读")
}

// TestExposureNamespaceRules 校验命名空间的白/黑名单与前缀匹配语义。
func TestExposureNamespaceRules(t *testing.T) {
	withExposure(t, eino.ApiExposureConfig{
		Mode:    ExposureModeAll,
		AllowNS: []string{"archive", "plugin/keyword"},
		DenyNS:  []string{"plugin/keyword/import"},
	})

	cases := []struct {
		name string
		ep   EndpointMeta
		want bool
	}{
		{"白名单命中", EndpointMeta{NS: "archive", Risk: "read"}, true},
		{"白名单二级命中", EndpointMeta{NS: "plugin/keyword", Resource: "keyword", Risk: "read"}, true},
		{"白名单未命中", EndpointMeta{NS: "setting", Risk: "read"}, false},
		{"白名单前缀不可越界", EndpointMeta{NS: "archivex", Risk: "read"}, false},
		{"黑名单优先于白名单", EndpointMeta{NS: "plugin/keyword/import", Risk: "read"}, false},
	}
	for _, c := range cases {
		if got := EvaluateExposure(CurrentApiExposure(), c.ep).Allowed; got != c.want {
			t.Fatalf("%s: 期望 allowed=%v，实得 %v", c.name, c.want, got)
		}
	}
	t.Log("命名空间白/黑名单与前缀边界（archive 不误匹配 archivex）均正确")
}

// TestExposureDenyEndpoint 校验端点级黑名单，含方法维度与方法省略两种写法。
func TestExposureDenyEndpoint(t *testing.T) {
	withExposure(t, eino.ApiExposureConfig{
		Mode:          ExposureModeAll,
		DenyEndpoints: []string{"POST /system/api/archive/delete", "/system/api/design/use"},
	})

	cases := []struct {
		ep   EndpointMeta
		want bool
	}{
		{EndpointMeta{Method: "POST", Path: "/system/api/archive/delete", NS: "archive", Risk: "destructive"}, false},
		{EndpointMeta{Method: "GET", Path: "/system/api/archive/delete", NS: "archive", Risk: "destructive"}, true},
		{EndpointMeta{Method: "POST", Path: "/system/api/design/use", NS: "design", Risk: "write"}, false},
		{EndpointMeta{Method: "GET", Path: "/system/api/design/use", NS: "design", Risk: "read"}, false},
		{EndpointMeta{Method: "POST", Path: "/system/api/archive/detail", NS: "archive", Risk: "write"}, true},
	}
	for i, c := range cases {
		if got := EvaluateExposure(CurrentApiExposure(), c.ep).Allowed; got != c.want {
			t.Fatalf("用例 %d (%s %s): 期望 allowed=%v，实得 %v", i, c.ep.Method, c.ep.Path, c.want, got)
		}
	}
	t.Log("端点黑名单：区分方法与全方法屏蔽均生效，且可省略 /system/api 前缀")
}

// TestExposureUnknownRiskIsFailClosed 未知风险等级按 destructive 处理，避免漏判。
func TestExposureUnknownRiskIsFailClosed(t *testing.T) {
	withExposure(t, eino.ApiExposureConfig{Mode: ExposureModeReadWrite})
	ep := EndpointMeta{Method: "POST", Path: "/system/api/weird/action", NS: "weird", Risk: ""}
	if d := EvaluateExposure(CurrentApiExposure(), ep); d.Allowed {
		t.Fatal("风险等级未知的端点不应被放行")
	}
	t.Log("未知风险等级按最高风险处理")
}

// TestExposureRealCatalogStats 用真实目录跑一遍推荐策略，把开放面固化下来：
// 一旦推荐策略或目录发生大变，数字会立刻反映出来。
func TestExposureRealCatalogStats(t *testing.T) {
	cat, err := BuildAPICatalog()
	if err != nil {
		t.Fatal(err)
	}
	withExposure(t, RecommendedExposure())

	allowed := 0
	byNS := map[string]int{}
	riskSet := map[string]int{}
	for _, ep := range cat.Endpoints {
		d := EvaluateExposure(CurrentApiExposure(), ep)
		if !d.Allowed {
			continue
		}
		allowed++
		byNS[ep.NS]++
		riskSet[normalizeRisk(ep.Risk)]++
	}
	if allowed == 0 {
		t.Fatal("推荐策略（只读 + 内容命名空间）不应放行 0 个端点")
	}
	if riskSet["write"] > 0 || riskSet["destructive"] > 0 {
		t.Fatalf("只读策略不应放行写或删除端点: %v", riskSet)
	}
	if len(byNS) == 0 {
		t.Fatal("应有命名空间被放行")
	}
	t.Logf("推荐策略在真实目录上放行 %d/%d 个端点，风险分布 %v", allowed, len(cat.Endpoints), riskSet)
}

// TestCapAPIListHonorsExposure 校验 api_list 默认只列可调用端点，
// 且被拦端点可通过 only_allowed=false 连同原因一起查看。
func TestCapAPIListHonorsExposure(t *testing.T) {
	svc := newAPITestService()

	// 默认策略（off）：不应列出任何端点，但要给出可执行的原因
	withExposure(t, eino.ApiExposureConfig{})
	empty, err := svc.capAPIList(context.Background(), `{"ns":"archive","limit":5}`)
	if err != nil {
		t.Fatal(err)
	}
	if strings.Contains(empty, `"path":"/system/api/archive/list"`) {
		t.Fatalf("策略未开放时不应列出端点:\n%s", empty)
	}
	if !strings.Contains(empty, "hint") {
		t.Fatalf("应给出「策略未开放」的可执行提示:\n%s", empty)
	}

	// only_allowed=false：能看到被拦原因
	shown, err := svc.capAPIList(context.Background(), `{"ns":"archive","limit":3,"only_allowed":false}`)
	if err != nil {
		t.Fatal(err)
	}
	for _, want := range []string{`"allowed":false`, `"stage":"mode_off"`, `"reason"`} {
		if !strings.Contains(shown, want) {
			t.Fatalf("only_allowed=false 时应暴露 %s:\n%s", want, shown)
		}
	}

	// 切到推荐策略后应能列出只读端点
	withExposure(t, RecommendedExposure())
	listed, err := svc.capAPIList(context.Background(), `{"ns":"archive","limit":5}`)
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(listed, `/system/api/archive/list`) {
		t.Fatalf("推荐策略下应能列出 archive 只读端点:\n%s", listed)
	}
	t.Log("api_list 正确遵循开放策略，且能回传拦截原因")
}

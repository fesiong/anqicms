package intent

import (
	"context"
	"testing"

	"kandaoni.com/anqicms/pkg/mcp/server"
)

// 捕获型 CapInvoker：记录被调用的能力名与参数，供断言使用。
type captureInvoker struct {
	name string
	args map[string]any
}

func (c *captureInvoker) invoke(ctx context.Context, name string, args map[string]any) (string, error) {
	c.name = name
	c.args = args
	return "{}", nil
}

// TestInvokeRoutesDispatchesToAPIInvoke 验证 action → method/path 的分派与参数组装。
//
// 关键断言是 action **不能**混进端点参数：它是分派用字段，
// 若透传下去，端点会收到一个自己不认识的字段（多数接口直接忽略，但会污染 multipart/struct 绑定）。
func TestInvokeRoutesDispatchesToAPIInvoke(t *testing.T) {
	c := &captureInvoker{}
	compose := invokeRoutes("", map[string]string{
		"list": "GET /system/api/plugin/push",
		"push": "POST /system/api/plugin/push",
	})
	if _, err := compose(context.Background(), map[string]any{"action": "push", "urls": []string{"https://a"}}, c.invoke); err != nil {
		t.Fatalf("执行失败: %v", err)
	}
	if c.name != "api_invoke" {
		t.Fatalf("应经 api_invoke 执行，实得 %s", c.name)
	}
	if c.args["method"] != "POST" || c.args["path"] != "/system/api/plugin/push" {
		t.Fatalf("分派错误: %#v", c.args)
	}
	params, _ := c.args["params"].(map[string]any)
	if _, leaked := params["action"]; leaked {
		t.Error("action 被透传进了端点参数")
	}
	if _, ok := params["urls"]; !ok {
		t.Errorf("业务参数应原样透传: %#v", params)
	}
}

// TestInvokeRoutesRejectsUnknownAction 未知 action 必须报错，不能静默落到某个默认路由。
func TestInvokeRoutesRejectsUnknownAction(t *testing.T) {
	c := &captureInvoker{}
	compose := invokeRoutes("", map[string]string{"list": "GET /system/api/plugin/push"})
	if _, err := compose(context.Background(), map[string]any{"action": "nope"}, c.invoke); err == nil {
		t.Fatal("未知 action 应报错")
	}
}

// TestInvokeRoutesRejectsMalformedTarget 路由声明格式错误应显式报错。
// 静默分派会让调用落到错误方法上（例如把写操作当读操作执行），代价很高。
func TestInvokeRoutesRejectsMalformedTarget(t *testing.T) {
	c := &captureInvoker{}
	for _, bad := range []string{"", "/system/api/plugin/push", "GET"} {
		compose := invokeRoutes("", map[string]string{"x": bad})
		if _, err := compose(context.Background(), map[string]any{"action": "x"}, c.invoke); err == nil {
			t.Errorf("格式错误的目标 %q 应报错", bad)
		}
	}
}

// TestDomainIntentsDeclareEveryAction 声明的 action 枚举必须都能分派。
//
// 与 provider 侧的端点存在性校验互补：这里在 intent 包内单跑即可守住
// 「Enum 与 routes 不一致」这类声明错配，不依赖 provider 的目录构建。
func TestDomainIntentsDeclareEveryAction(t *testing.T) {
	checked := 0
	for _, s := range domainIntentCatalog {
		for _, action := range s.Params["action"].Enum {
			c := &captureInvoker{}
			if _, err := s.Compose(context.Background(), map[string]any{"action": action}, c.invoke); err != nil {
				t.Errorf("意图 %s 声明了 action=%s 却无法分派: %v", s.Name, action, err)
				continue
			}
			if c.name != "api_invoke" {
				t.Errorf("意图 %s 应经 api_invoke 执行，实得 %s", s.Name, c.name)
			}
			checked++
		}
	}
	if checked < 150 {
		t.Fatalf("仅校验 %d 条，判定条件可能已失效", checked)
	}
}

// TestDomainIntentsUseDeclaredDomains 新意图的域必须是已声明域（防拼写造域）。
func TestDomainIntentsUseDeclaredDomains(t *testing.T) {
	known := map[Domain]bool{}
	for _, d := range AllDomains() {
		known[d] = true
	}
	for _, s := range domainIntentCatalog {
		if !known[s.Domain] {
			t.Errorf("意图 %s 的域 %q 未声明", s.Name, s.Domain)
		}
		if s.Risk == "" {
			t.Errorf("意图 %s 未声明风险等级", s.Name)
		}
		if len(s.Caps) != 1 || s.Caps[0] != "api_invoke" {
			t.Errorf("意图 %s 的 Caps 应恰为 [api_invoke]，实得 %v", s.Name, s.Caps)
		}
	}
}

// TestExecuteRejectsUnopenedDefaultOff 未开放的 DefaultOff 意图必须被 Execute 拒绝。
//
// 这条断言曾经失败过：准入原本只在"注册到模型面"时判定，Execute 不检查，
// 于是任何绕过注册直接调 Execute 的内部路径都能执行备份/升级/迁移这类高危能力。
// 现在 Execute 自己再拦一次（纵深防御），这里把它钉住。
func TestExecuteRejectsUnopenedDefaultOff(t *testing.T) {
	inv := func(ctx context.Context, name string, args map[string]any) (string, error) { return "{}", nil }

	// 只开了 design_manage
	k := NewKernel(Config{ExposedIntents: []string{"design_manage"}}, inv, nil)
	if _, err := k.Execute(context.Background(), "design_manage", `{"action":"list"}`); err != nil {
		t.Fatalf("已开放意图应可执行: %v", err)
	}
	if _, err := k.Execute(context.Background(), "siteops_backup", `{"action":"list"}`); err == nil {
		t.Fatal("未开放的 siteops_backup 竟能执行，DefaultOff 在 Execute 层失效")
	}

	// 白名单写域名可整域放行（与 matchIntentPattern 的约定一致）
	k2 := NewKernel(Config{ExposedIntents: []string{"siteops"}}, inv, nil)
	if _, err := k2.Execute(context.Background(), "siteops_backup", `{"action":"list"}`); err != nil {
		t.Errorf("域级白名单应放行整个 siteops 域: %v", err)
	}

	// 白名单为空时 DefaultOff 仍不开放（不能因"没配就全开"而暴露）
	k3 := NewKernel(Config{}, inv, nil)
	if _, err := k3.Execute(context.Background(), "siteops_backup", `{"action":"list"}`); err == nil {
		t.Fatal("白名单为空时 DefaultOff 意图仍不应开放")
	}
}

// TestRecommendedExposedIsConservative 推荐白名单不得包含高风险运维类意图。
// 这份清单会被一键填入配置，混进备份/升级/迁移等于绕过了人工确认环节。
func TestRecommendedExposedIsConservative(t *testing.T) {
	forbidden := map[string]bool{
		"siteops_backup": true, "siteops_upgrade": true, "siteops_website": true,
		"contentops_transfer": true, "api_invoke": true, "api": true,
	}
	for _, n := range RecommendedExposed() {
		if forbidden[n] {
			t.Errorf("推荐白名单不应包含高风险意图 %s", n)
		}
	}
}

// TestDomainIntentExposureIsDeliberate 每个补齐域意图都必须显式表态：
// 要么默认开放，要么登记进 gatedDomainIntents 并写明理由。
//
// 双向校验的意义在于把"高危能力默认关闭"从人的记忆变成机器约束：
// 漏标一行 DefaultOff 会让备份/升级直接出现在模型面上，
// 多标一行则让日常功能凭空消失，两种错都没有编译告警。
func TestDomainIntentExposureIsDeliberate(t *testing.T) {
	open, gated := 0, 0
	for _, s := range domainIntentCatalog {
		reason, listed := gatedDomainIntents[s.Name]
		switch {
		case s.DefaultOff && !listed:
			t.Errorf("意图 %s 标记了 DefaultOff 却未在 gatedDomainIntents 登记理由", s.Name)
		case listed && !s.DefaultOff:
			t.Errorf("意图 %s 已登记为高危却漏标 DefaultOff", s.Name)
		case listed && reason == "":
			t.Errorf("意图 %s 的高危理由为空", s.Name)
		}
		if s.DefaultOff {
			gated++
		} else {
			open++
		}
	}
	if len(gatedDomainIntents) != gated {
		t.Fatalf("gatedDomainIntents 登记 %d 条，实际 DefaultOff 意图 %d 个", len(gatedDomainIntents), gated)
	}
	if open < 10 {
		t.Fatalf("补齐域默认开放仅 %d 个，默认可见面可能又被整体关回去了", open)
	}
	t.Logf("补齐域意图 %d 个：默认开放 %d，需显式开启 %d", len(domainIntentCatalog), open, gated)
}

// TestCommonIntentsVisibleWithoutWhitelist 站点未配置白名单时，常用意图必须出现在工具面，
// 高危意图必须仍然缺席 —— 这条是"默认开放常用能力"改造的正反两面。
func TestCommonIntentsVisibleWithoutWhitelist(t *testing.T) {
	f := &fakeCap{}
	k := NewKernel(Config{}, f.invoker(), nil)
	srv, err := server.New(server.DefaultConfig())
	if err != nil {
		t.Fatalf("create server: %v", err)
	}
	k.RegisterAll(srv.GetServer())

	for _, name := range defaultOpenDomainIntents {
		if !k.IsRegistered(name) {
			t.Errorf("常用意图 %s 未默认可见", name)
		}
	}
	for name := range gatedDomainIntents {
		if k.IsRegistered(name) {
			t.Errorf("高危意图 %s 在未配置白名单时被暴露了", name)
		}
	}
	// 两个清单必须刚好覆盖全部补齐域意图：新增意图不表态就会在这里失败。
	declared := len(defaultOpenDomainIntents) + len(gatedDomainIntents)
	if declared != len(domainIntentCatalog) {
		t.Fatalf("默认开放 %d + 需显式开启 %d = %d，与补齐域意图总数 %d 不符（新增意图未表态）",
			len(defaultOpenDomainIntents), len(gatedDomainIntents), declared, len(domainIntentCatalog))
	}
}

// TestRecommendedExposedDoesNotNarrowDefaults 推荐清单不得要求显式开启高危意图。
// 它的定位是"日常运营起步面"，若混进一个 DefaultOff 意图，填入它反而把用户领到
// 备份/资金/对外发信这类高风险能力上，与"保守起步"的说明正好相反。
func TestRecommendedExposedDoesNotNarrowDefaults(t *testing.T) {
	for _, n := range RecommendedExposed() {
		spec, ok := specByName(n)
		if !ok {
			continue // 名字是否真实存在由 provider 侧 TestDomainIntentDefaultExposure 负责
		}
		if spec.DefaultOff {
			t.Errorf("推荐白名单包含默认关闭的意图 %s", n)
		}
	}
}

// defaultOpenDomainIntents 是补齐域中必须默认可见的意图（人工维护的策略表述）。
// 与 gatedDomainIntents 互补：两者之和必须等于补齐域意图总数，新增意图不表态即失败。
var defaultOpenDomainIntents = []string{
	"content_place", "seo", "seo_jsonld", "seo_llms", "interaction",
	"contentops_material", "contentops_translate", "commerce", "commerce_order",
	"siteops_maintain", "account", "design_manage",
}

package intent

import (
	"context"
	"testing"
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

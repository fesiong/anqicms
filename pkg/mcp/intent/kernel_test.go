package intent

import (
	"context"
	"testing"

	"kandaoni.com/anqicms/pkg/mcp/server"
)

// TestRegisterAll 把全部意图注册进真实 mcp.Server，验证声明式 schema
// 能被 SDK 成功校验（AddTool 会对非法 JSON Schema 报错）。
func TestRegisterAll(t *testing.T) {
	f := &fakeCap{}
	k := NewKernel(Config{}, f.invoker(), nil)

	srv, err := server.New(server.DefaultConfig())
	if err != nil {
		t.Fatalf("create server: %v", err)
	}
	// 不应 panic，且应注册全部领域意图 + meta 意图。
	// 注意 DefaultOff 意图不计入：它们默认不暴露，故需从 IntentCatalog 中扣除。
	//
	// 2026-10-07：meta 工具只有mcp_list_intents 默认注册（+1）——
	// mcp_set_scope 受 Config.EnableSetScope 门控且默认关闭，
	// 理由见 mcp.go 的 registerMeta（scope 是进程级全局，会影响所有客户端）。
	metaCount := 1
	if k.cfg.EnableSetScope {
		metaCount = 2
	}
	k.RegisterAll(srv.GetServer())
	if got := k.RegisteredCount(); got != countExposedByDefault()+metaCount {
		t.Fatalf("expected %d registered, got %d", countExposedByDefault()+metaCount, got)
	}
}

// TestSetScopeNotRegisteredByDefault 钉住「mcp_set_scope 默认不出现」这条修复期策略。
//
// 它必须被单独锁住：mcp_set_scope 的 SetScope 是**进程级全局**的，
// 一个客户端设窄会让所有并发客户端的 tools/list 一起变窄且新会话恢复不了。
// 改成会话级之前，这条工具不能回到默认暴露面。
//
// 同时验证 mcp_list_intents 仍在——它是只读发现，无副作用，
// 客户端不知道有哪些能力时仍需要这条安全路径（见 mcp.go 的 registerListIntents）。
func TestSetScopeNotRegisteredByDefault(t *testing.T) {
	f := &fakeCap{}
	k := NewKernel(Config{}, f.invoker(), nil)
	srv, err := server.New(server.DefaultConfig())
	if err != nil {
		t.Fatalf("create server: %v", err)
	}
	k.RegisterAll(srv.GetServer())

	if k.IsRegistered("mcp_set_scope") {
		t.Error("mcp_set_scope 不应默认注册：它的 scope 是进程级全局的，" +
			"一个客户端调用会影响所有并发客户端（修复前默认关闭）")
	}
	if !k.IsRegistered("mcp_list_intents") {
		t.Error("mcp_list_intents 必须始终注册：只读发现无副作用，是客户端唯一安全的探测路径")
	}
}

// TestSetScopeRegistersWhenEnabled 反向对照：开关打开时它必须真的能被注册，
// 否则「默认关闭」会退化成「永久关闭」——即修复后没人能重新开启它。
func TestSetScopeRegistersWhenEnabled(t *testing.T) {
	f := &fakeCap{}
	k := NewKernel(Config{EnableSetScope: true}, f.invoker(), nil)
	srv, err := server.New(server.DefaultConfig())
	if err != nil {
		t.Fatalf("create server: %v", err)
	}
	k.RegisterAll(srv.GetServer())
	if !k.IsRegistered("mcp_set_scope") {
		t.Error("EnableSetScope=true 时 mcp_set_scope 应被注册，否则它将永久不可用")
	}
}

// countExposedByDefault 统计未标记 DefaultOff 的意图数量。
func countExposedByDefault() int {
	n := 0
	for _, s := range IntentCatalog {
		if !s.DefaultOff {
			n++
		}
	}
	return n
}

// TestDefaultOffIntentsHidden 校验通用调用类意图的安全默认值：
// 白名单为空时不暴露，只有被 ExposedIntents 显式点名才出现。
//
// 这条是安全边界，不能退化 —— api_invoke 一旦默认暴露，等于把 394 个后台端点
// 连同写操作一起交给模型，而 ExposedTools 这类能力名粒度白名单也拦不住它。
func TestDefaultOffIntentsHidden(t *testing.T) {
	const intentName = "api"
	spec, ok := specByName(intentName)
	if !ok {
		t.Fatalf("意图 %s 未存在于 IntentCatalog", intentName)
	}
	if !spec.DefaultOff {
		t.Fatalf("%s 必须标记 DefaultOff", intentName)
	}

	// 1) 白名单为空：不得注册
	k := NewKernel(Config{}, (&fakeCap{}).invoker(), nil)
	srv, _ := server.New(server.DefaultConfig())
	k.RegisterAll(srv.GetServer())
	for _, name := range k.reg {
		if name == intentName {
			t.Fatalf("%s 在白名单为空时被暴露了", intentName)
		}
	}

	// 2) ExposedTools 命中其能力名：仍然不得注册（DefaultOff 不接受能力名粒度解锁）
	k2 := NewKernel(Config{ExposedTools: []string{"api_invoke"}}, (&fakeCap{}).invoker(), nil)
	srv2, _ := server.New(server.DefaultConfig())
	k2.RegisterAll(srv2.GetServer())
	for _, name := range k2.reg {
		if name == intentName {
			t.Fatalf("%s 不应被 ExposedTools 解锁", intentName)
		}
	}

	// 3) ExposedIntents 显式点名：应当注册
	k3 := NewKernel(Config{ExposedIntents: []string{intentName}}, (&fakeCap{}).invoker(), nil)
	srv3, _ := server.New(server.DefaultConfig())
	k3.RegisterAll(srv3.GetServer())
	found := false
	for _, name := range k3.reg {
		if name == intentName {
			found = true
		}
	}
	if !found {
		t.Fatalf("%s 应能被 ExposedIntents 显式开放", intentName)
	}
}

func specByName(name string) (*IntentSpec, bool) {
	for _, s := range IntentCatalog {
		if s.Name == name {
			return s, true
		}
	}
	return nil, false
}

// TestRegisterSummaryMode 验证 summary 模式下仍注册成功（轻量 schema）。
func TestRegisterSummaryMode(t *testing.T) {
	f := &fakeCap{}
	k := NewKernel(Config{ToolListMode: "summary"}, f.invoker(), nil)
	srv, err := server.New(server.DefaultConfig())
	if err != nil {
		t.Fatalf("create server: %v", err)
	}
	k.RegisterAll(srv.GetServer())
	if k.RegisteredCount() == 0 {
		t.Fatal("summary mode should still register intents")
	}
}

// TestSetScopeTwoStage 验证两阶段：SetScope 仅设置范围（中性，不改变注册数），
// 真正的移除/重注册由 Reregister 完成。
func TestSetScopeTwoStage(t *testing.T) {
	f := &fakeCap{}
	k := NewKernel(Config{}, f.invoker(), nil)
	srv, _ := server.New(server.DefaultConfig())
	k.RegisterAll(srv.GetServer())
	full := k.RegisteredCount()

	// SetScope 是中性操作：仅记录 scope，不触发重注册
	k.SetScope([]string{"content"})
	if k.RegisteredCount() != full {
		t.Fatalf("SetScope 应仅设置范围、不改变注册数: before=%d after=%d", full, k.RegisteredCount())
	}

	// Reregister 按新 scope 重注册
	k.Reregister(srv.GetServer())
	afterScope := k.RegisteredCount()
	if afterScope >= full {
		t.Fatalf("scoped register should be fewer than full: full=%d after=%d", full, afterScope)
	}

	// 恢复 scope 后重注册应回到全量
	k.SetScope(nil)
	k.Reregister(srv.GetServer())
	if k.RegisteredCount() != full {
		t.Fatalf("reset scope should restore full: got %d want %d", k.RegisteredCount(), full)
	}
}

// TestExecuteNeutral 验证中性 Execute 直接路由到意图组合逻辑（不依赖任何通道 SDK），
// 且未知意图返回错误。
func TestExecuteNeutral(t *testing.T) {
	f := &fakeCap{}
	k := NewKernel(Config{}, f.invoker(), nil)

	// 已知意图：委托到底层能力 cap（web action=fetch → cap web_fetch）
	res, err := k.Execute(context.Background(), "web", `{"action":"fetch","url":"https://example.com"}`)
	if err != nil {
		t.Fatalf("Execute web error: %v", err)
	}
	if res == nil || res.Text == "" {
		t.Fatal("expected non-empty result text")
	}
	if !contains(f.calls, "web_fetch") {
		t.Fatalf("expected cap web_fetch to be invoked, got calls=%v", f.calls)
	}

	// 未知意图：返回错误
	if _, err := k.Execute(context.Background(), "no_such_intent", "{}"); err == nil {
		t.Fatal("expected error for unknown intent")
	}
}

func contains(ss []string, s string) bool {
	for _, v := range ss {
		if v == s {
			return true
		}
	}
	return false
}

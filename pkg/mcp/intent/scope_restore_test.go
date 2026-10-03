package intent

import (
	"context"
	"testing"
	"time"
)

// 这组测试锁住 scope 的两个已知缺陷（2026-10-03 实测）：
//
//  1. scope 是**进程级全局**，不是会话级。Reregister 摘除/重注册的是共享
//     mcp.Server 的工具表，一个客户端设窄后所有并发客户端的 tools/list
//     都被收窄，且新会话也恢复不了（须重启服务）。
//  2. 没有恢复机制：一次临时设置会永久生效。
//
// 修复策略：scope 只影响 tools/list 的视图裁剪（省 token），不影响可调用性
// （tools/call 不校验 scope），安全性由 allowed() 负责。在此基础上加
// 空闲自动恢复（ScopeStaleAfter），并把「全局生效」写进返回文案让调用方知晓。

// TestScopeStaleAutoRestore 空闲超期后应能恢复为全部工具。
func TestScopeStaleAutoRestore(t *testing.T) {
	k := NewKernel(Config{}, nil, nil)

	// 从未设置过 → 无需恢复
	if k.RestoreScopeIfStale() {
		t.Error("从未设置 scope 时不应触发恢复")
	}

	// 设置成非空 scope
	k.SetScope([]string{"content"})
	k.mu.Lock()
	hasScope := k.scope != nil
	k.mu.Unlock()
	if !hasScope {
		t.Fatal("SetScope 后 scope 应为非 nil")
	}

	// 刚设置过 → 未超期，不该恢复（否则连续会话会被立刻还原）
	if k.RestoreScopeIfStale() {
		t.Error("刚设置 scope 后不应立即触发恢复")
	}

	// 把 scopeTouched 往前推 31 分钟（超过阈值）
	k.mu.Lock()
	k.scopeTouched = time.Now().Add(-ScopeStaleAfter - time.Minute)
	k.mu.Unlock()

	if !k.ScopeExpired(time.Now()) {
		t.Fatal("空闲超过阈值后 ScopeExpired 应为 true")
	}
	if !k.RestoreScopeIfStale() {
		t.Error("空闲超期后 RestoreScopeIfStale 应返回 true")
	}
	k.mu.Lock()
	restored := k.scope == nil
	k.mu.Unlock()
	if !restored {
		t.Error("恢复后 scope 应被清空（回到全部工具）")
	}

	// 恢复只应发生一次
	if k.RestoreScopeIfStale() {
		t.Error("已恢复后不应重复触发恢复")
	}
}

// TestScopeStaleAfterNotTooShort 阈值不能太短。
//
// 30 分钟的依据：足够覆盖一次连续的 AI 会话（会话中间不会空闲半小时），
// 又不至于让一次误设永久生效。设成 1 分钟会让正常使用中的 scope 反复被还原。
func TestScopeStaleAfterNotTooShort(t *testing.T) {
	if ScopeStaleAfter < 10*time.Minute {
		t.Errorf("ScopeStaleAfter=%v 过短，正常使用中会被反复还原", ScopeStaleAfter)
	}
	if ScopeStaleAfter > 2*time.Hour {
		t.Errorf("ScopeStaleAfter=%v 过长，一次误设会长期影响其他客户端", ScopeStaleAfter)
	}
}

// TestSetScopeEmptyRestores 传空数组应恢复为全部。
func TestSetScopeEmptyRestores(t *testing.T) {
	k := NewKernel(Config{}, nil, nil)
	k.SetScope([]string{"seo"})
	k.mu.Lock()
	narrowed := k.scope != nil
	k.mu.Unlock()
	if !narrowed {
		t.Fatal("SetScope([seo]) 后 scope 应为非 nil")
	}
	k.SetScope(nil)
	k.mu.Lock()
	restored := k.scope == nil
	k.mu.Unlock()
	if !restored {
		t.Error("SetScope(nil) 应把 scope 清空（恢复全部）")
	}
	// 清空后不该再被视为需要恢复
	if k.RestoreScopeIfStale() {
		t.Error("scope 已是全部，不应触发恢复")
	}
}

// TestScopeDoesNotAffectExecutability scope 只裁剪视图，不影响调用。
//
// 这是「scope 全局」可接受的前提：被裁掉的工具仍能按名字直接调用
// （Execute 不校验 scope），所以它只省 token、不构成安全边界。
// 真正的门禁是 allowed()（风险 + 暴露白名单）。
//
// 这条断言同时守住一个坏改动：若哪天有人为了「让 scope 真的生效」
// 在 Execute 里加 scope 校验，会把一个视图优化变成安全语义变更。
func TestScopeDoesNotAffectExecutability(t *testing.T) {
	k := NewKernel(Config{}, nil, nil)
	// 设一个与目标意图无关的域
	k.SetScope([]string{"seo"})

	// seo 域之外的意图仍应能 Execute（只是不出现在 tools/list 里）
	spec := &IntentSpec{
		Name:   "scope_probe",
		Domain: DomainContent,
		Risk:   RiskRead,
		Compose: func(ctx context.Context, args map[string]any, cap CapInvoker) (*Result, error) {
			return &Result{Text: `{"ok":true}`}, nil
		},
	}
	k.intents[spec.Name] = spec
	k.order = append(k.order, spec.Name)

	if _, err := k.Execute(context.Background(), "scope_probe", `{"action":"x"}`); err != nil {
		t.Errorf("scope 不应影响可调用性：Execute 报错 %v", err)
	}
}

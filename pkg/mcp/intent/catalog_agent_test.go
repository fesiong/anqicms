package intent

import (
	"context"
	"testing"
)

// 回归：agent 意图的字段名必须换算成底层 agent_* handler 认识的键。
//
// 底层 agent_create/agent_chat 是 provider/aiTools.go 的内存能力（无 REST 端点），
// 参数名是 name/strategy/message/enabled；意图层对外暴露 title/prompt/status。
// switchCompose 只按 action 分派、不改字段名，映射缺失时 agent_create 会直接返回
// 「错误：名称和策略不能为空」——2026-10-02 实际发生过一次。
func TestAgentComposeRenamesFieldsForCreate(t *testing.T) {
	c := &captureInvoker{}
	compose := agentCompose()
	args := map[string]any{
		"action": "manage_create",
		"title":  "GEO 热点关键词日报",
		"prompt": "每天收集 GEO 热门关键词并写 3-5 篇文章发布",
		"cron":   "0 8 * * *",
	}
	if _, err := compose(context.Background(), args, c.invoke); err != nil {
		t.Fatalf("执行失败: %v", err)
	}
	if c.name != "agent_create" {
		t.Fatalf("应分派到 agent_create，实得 %s", c.name)
	}
	if c.args["name"] != "GEO 热点关键词日报" {
		t.Errorf("title 未换算为 name: %#v", c.args["name"])
	}
	if c.args["strategy"] == "" || c.args["strategy"] == nil {
		t.Errorf("prompt 未换算为 strategy: %#v", c.args["strategy"])
	}
	// cron 必须透传，否则「每天定时执行」在 MCP 侧无法表达。
	if c.args["cron"] != "0 8 * * *" {
		t.Errorf("cron 未透传，Agent 无法定时执行: %#v", c.args["cron"])
	}
	// 别名来源键不应残留，避免污染底层 JSON 解析。
	if _, leaked := c.args["title"]; leaked {
		t.Error("title 残留，未被清理")
	}
	if _, leaked := c.args["action"]; leaked {
		t.Error("action 被透传进了底层参数")
	}
}

// 显式传 name/strategy 时不应被 title/prompt 覆盖。
func TestAgentComposePrefersCanonicalNames(t *testing.T) {
	c := &captureInvoker{}
	compose := agentCompose()
	_, err := compose(context.Background(), map[string]any{
		"action":   "manage_create",
		"name":     "规范名",
		"title":    "别名不应覆盖",
		"strategy": "规范策略",
		"prompt":   "别名不应覆盖",
	}, c.invoke)
	if err != nil {
		t.Fatalf("执行失败: %v", err)
	}
	if c.args["name"] != "规范名" {
		t.Errorf("显式 name 被别名覆盖: %#v", c.args["name"])
	}
	if c.args["strategy"] != "规范策略" {
		t.Errorf("显式 strategy 被别名覆盖: %#v", c.args["strategy"])
	}
}

// manage_chat 用 message；prompt 是它的别名。
func TestAgentComposeChatUsesMessage(t *testing.T) {
	c := &captureInvoker{}
	compose := agentCompose()
	_, err := compose(context.Background(), map[string]any{
		"action": "manage_chat",
		"id":     1,
		"prompt": "查一下执行历史",
	}, c.invoke)
	if err != nil {
		t.Fatalf("执行失败: %v", err)
	}
	if c.name != "agent_chat" {
		t.Fatalf("应分派到 agent_chat，实得 %s", c.name)
	}
	if c.args["message"] != "查一下执行历史" {
		t.Errorf("prompt 未换算为 message: %#v", c.args)
	}
}

// manage_toggle 用 enabled；status 是它的别名。
func TestAgentComposeToggleUsesEnabled(t *testing.T) {
	c := &captureInvoker{}
	compose := agentCompose()
	_, err := compose(context.Background(), map[string]any{
		"action": "manage_toggle",
		"id":     3,
		"status": 0,
	}, c.invoke)
	if err != nil {
		t.Fatalf("执行失败: %v", err)
	}
	if c.name != "agent_toggle" {
		t.Fatalf("应分派到 agent_toggle，实得 %s", c.name)
	}
	if c.args["enabled"] != 0 {
		t.Errorf("status 未换算为 enabled: %#v", c.args)
	}
}

// 未知 action 必须报错，不能静默落到某个能力上。
func TestAgentComposeRejectsUnknownAction(t *testing.T) {
	c := &captureInvoker{}
	compose := agentCompose()
	if _, err := compose(context.Background(), map[string]any{"action": "manage_nope"}, c.invoke); err == nil {
		t.Fatal("未知 action 应返回错误")
	}
}

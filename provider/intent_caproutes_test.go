package provider

import (
	"testing"

	"kandaoni.com/anqicms/pkg/mcp/intent"
)

// TestCapEndpointsExist 穷举校验：意图层的「能力→端点」映射表里，
// 每一条 method+path 都必须是真实存在的后台端点。
//
// 这是防"凭空写路径"的唯一防线 —— 声明式路由的编译期不报错，
// 写错只会在运行时落到 404。历史上这个坑出现过多次，所以必须穷举而不是抽样。
func TestCapEndpointsExist(t *testing.T) {
	cat, err := BuildAPICatalog()
	if err != nil {
		t.Fatalf("无法构建端点目录: %v", err)
	}
	if len(cat.Endpoints) == 0 {
		t.Fatal("端点目录为空，嵌入表可能未生成")
	}
	index := make(map[string]EndpointMeta, len(cat.Endpoints))
	for _, e := range cat.Endpoints {
		index[e.Method+" "+e.Path] = e
	}

	caps := intent.CapEndpoints()
	if len(caps) == 0 {
		t.Fatal("CapEndpoints 为空，映射表可能未初始化")
	}
	for name, ep := range caps {
		key := ep.Method + " " + normalizeCapPath(ep.Path)
		meta, ok := index[key]
		if !ok {
			t.Errorf("能力 %s 指向的端点不存在：%s", name, key)
			continue
		}
		// 重命名后的参数名必须是该端点声明过的参数，否则会被静默忽略。
		declared := make(map[string]bool, len(meta.Params))
		for _, p := range meta.Params {
			declared[p.Name] = true
		}
		if len(declared) == 0 {
			continue // 端点未声明参数（如无参 POST），跳过
		}
		for _, target := range ep.Rename {
			if !declared[target] {
				t.Errorf("能力 %s：重命名目标 %q 不是 %s 的声明参数", name, target, key)
			}
		}
	}
}

// TestCapEndpointsKeepNoAPI 确认"刻意保留"的能力没有被误加进端点表。
// 这 11 个没有 REST 等价端点，加进去等于让 AI 调用不存在的接口。
func TestCapEndpointsKeepNoAPI(t *testing.T) {
	caps := intent.CapEndpoints()
	kept := []string{
		"attachment_upload", // 端点要 multipart 文件，AI 侧是 base64/URL/本地路径
		"template_reload",   // RestartChan 重载信号
		"skill_search",      // SkillHub 在线市场
		"skill_install",
		"agent_create", "agent_list", "agent_delete", "agent_toggle", "agent_run", "agent_chat",
		"task",
	}
	for _, name := range kept {
		if _, ok := caps[name]; ok {
			t.Errorf("%s 不应有端点映射：它是刻意保留的无 API 能力", name)
		}
	}
}

// normalizeCapPath 与 intent 包的补全规则保持一致。
func normalizeCapPath(p string) string {
	if len(p) >= len("/system/api") && p[:len("/system/api")] == "/system/api" {
		return p
	}
	if len(p) >= 4 && p[:4] == "/api" {
		return "/system" + p
	}
	return "/system/api" + p
}

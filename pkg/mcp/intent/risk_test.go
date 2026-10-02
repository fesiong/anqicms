package intent

import (
	"strings"
	"testing"
)

func TestNeedsApproval(t *testing.T) {
	cases := map[Risk]bool{
		RiskRead:        false,
		RiskWrite:       true,
		RiskDestructive: true,
		RiskSystem:      true, // 曾经的绕过：只认 write 时 shell/文件写完全不问
	}
	for r, want := range cases {
		if got := NeedsApproval(r); got != want {
			t.Errorf("NeedsApproval(%s) = %v, 期望 %v", r, got, want)
		}
	}
}

func TestActionRisk_Table(t *testing.T) {
	cases := []struct {
		tool string
		args string
		want Risk
	}{
		// 合并意图：读动作不再被整体的 Risk=write 拖累
		{"content_article", `{"action":"list"}`, RiskRead},
		{"content_article", `{"action":"get"}`, RiskRead},
		{"content_article", `{"action":"save"}`, RiskWrite},
		{"content_article", `{"action":"publish"}`, RiskWrite},
		{"content_article", `{"action":"delete"}`, RiskDestructive},
		{"content_manage", `{"action":"category_list"}`, RiskRead},
		{"content_manage", `{"action":"tag_delete"}`, RiskDestructive},
		{"content_manage", `{"action":"module_create"}`, RiskWrite},

		// 统计意图：全部只读
		{"traffic_statistics", `{"action":"dashboard"}`, RiskRead},
		{"traffic_statistics", `{"action":"spider"}`, RiskRead},

		// 系统插件：动词决定档位
		{"system_plugin", `{"action":"robots_get"}`, RiskRead},
		{"system_plugin", `{"action":"robots_set"}`, RiskWrite},
		{"system_plugin", `{"action":"backup_dump"}`, RiskWrite},
		{"system_plugin", `{"action":"migrate_db"}`, RiskWrite},

		// 精选 SEO 意图
		{"seo_keyword", `{"action":"list"}`, RiskRead},
		{"seo_keyword", `{"action":"delete"}`, RiskDestructive},

		// 通用调用：发现类不该弹窗
		{"api", `{"action":"list"}`, RiskRead},
		{"api", `{"action":"schema"}`, RiskRead},
		{"api", `{"action":"invoke","method":"GET","path":"/archive/list"}`, RiskRead},
		{"api", `{"action":"invoke","method":"POST","path":"/archive/detail"}`, RiskWrite},
		{"api", `{"action":"invoke","method":"POST","path":"/archive/delete"}`, RiskDestructive},

		// 主机级内置意图：风险回落到意图声明（无 action 可推）
		{"shell_exec", `{"command":"ls"}`, RiskSystem},
		{"fs_write", `{"path":"a.txt","content":"x"}`, RiskSystem},
		{"fs_read", `{"path":"a.txt"}`, RiskRead},

		// cap 模式：工具名即能力名
		{"archive_list", `{}`, RiskRead},
		{"archive_delete", `{}`, RiskDestructive},
		{"bash", `{"command":"ls"}`, RiskSystem},
		{"read_file", `{}`, RiskRead},
		{"api_invoke", `{"method":"GET","path":"/archive/list"}`, RiskRead},

		// fail closed
		{"content_article", `{"action":"totally_unknown"}`, RiskWrite}, // 回落到意图声明
		{"totally_unknown_tool", `{}`, RiskSystem},
		{"", `{}`, RiskSystem},
	}
	for _, c := range cases {
		t.Run(c.tool+"/"+c.args, func(t *testing.T) {
			if got := ActionRiskFromArgsJSON(c.tool, c.args); got != c.want {
				t.Errorf("ActionRisk(%s, %s) = %s, 期望 %s", c.tool, c.args, got, c.want)
			}
		})
	}
}

// TestActionRisk_RouteDerivedWins 校验已声明端点路由的意图：
// 推导结果必须与端点方法一致（GET→read、DELETE→destructive），
// 这样"路由表就是真相源"不会随执行路径漂移。
func TestActionRisk_RouteDerivedWins(t *testing.T) {
	checked := 0
	for intentName, routes := range intentRouteRegistry {
		for action, target := range routes {
			parts := strings.Fields(target)
			if len(parts) != 2 {
				t.Fatalf("%s action=%s 路由声明格式错误: %q", intentName, action, target)
			}
			want := riskFromEndpoint(parts[0], parts[1])
			got := ActionRisk(intentName, map[string]any{"action": action})
			if got != want {
				t.Errorf("%s action=%s → %s, 与端点 %s 推导的 %s 不符", intentName, action, got, target, want)
			}
			checked++
		}
	}
	if checked == 0 {
		t.Fatal("intentRouteRegistry 为空：invokeRoutes 未登记路由，覆盖审计与审批门都会失真")
	}
}

// TestActionRisk_AuditAllIntents 遍历全部意图的 action 枚举，确保推导不出错、
// 并且"读动作永不审批"这条不变量成立。
func TestActionRisk_AuditAllIntents(t *testing.T) {
	specCount := 0
	for _, spec := range IntentCatalog {
		if spec == nil {
			continue
		}
		specCount++
		param := spec.Params["action"]
		actions := param.Enum
		if len(actions) == 0 {
			actions = []string{""}
		}
		for _, action := range actions {
			args := map[string]any{}
			if action != "" {
				args["action"] = action
			}
			r := ActionRisk(spec.Name, args)
			switch r {
			case RiskRead, RiskWrite, RiskDestructive, RiskSystem:
			default:
				t.Errorf("%s action=%q 推导出未定义的风险 %q", spec.Name, action, r)
			}
			if r == RiskRead && NeedsApproval(r) {
				t.Errorf("%s action=%q 被判为只读却要求审批", spec.Name, action)
			}
			// 声明为只读的意图，其可达端点必须真的全是只读。
			// 这一条查的是"声明漏写"：Risk=read 会让工具标注说它 readOnlyHint，
			// 一旦同时路由了写端点，就等于对外承诺了做不到的事。
			if spec.Risk == RiskRead {
				for _, capName := range spec.Caps {
					if ep, ok := capEndpoints[capName]; ok && !isReadMethod(ep.Method) {
						t.Errorf("只读意图 %s 引用了非只读能力 %s（%s %s）", spec.Name, capName, ep.Method, ep.Path)
					}
				}
				for _, target := range intentRouteRegistry[spec.Name] {
					parts := strings.Fields(target)
					if len(parts) == 2 && !isReadMethod(parts[0]) {
						t.Errorf("只读意图 %s 路由了非只读端点 %q", spec.Name, target)
					}
				}
			}
		}
	}
	if specCount == 0 {
		t.Fatal("IntentCatalog 为空")
	}
}

func TestActionGrantSuffix(t *testing.T) {	if got := ActionGrantSuffix(map[string]any{"action": "save"}); got != "save" {
		t.Errorf("ActionGrantSuffix = %q", got)
	}
	if got := ActionGrantSuffix(map[string]any{"path": "a"}); got != "" {
		t.Errorf("无 action 时应返回空串，得到 %q", got)
	}
	if got := ActionGrantSuffix(nil); got != "" {
		t.Errorf("nil args 时应返回空串，得到 %q", got)
	}
}

func isReadMethod(method string) bool {
	switch strings.ToUpper(strings.TrimSpace(method)) {
	case "GET", "HEAD", "OPTIONS":
		return true
	}
	return false
}

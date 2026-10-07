package intent

import (
	"strings"
	"testing"
)

// TestDeclaredEndpointTargetsRegistered 穷举校验意图层声明的端点归属与格式。
//
// invokeRoutes 的意图名是手写进调用点的字符串，写错（或复制粘贴后忘改）不会
// 编译报错，却会让覆盖审计把端点记到别人的账上——这正是本项目反复踩到的
// "声明式映射必须穷举校验"那一类坑。
func TestDeclaredEndpointTargetsRegistered(t *testing.T) {
	if len(intentRouteRegistry) == 0 {
		t.Fatal("invokeRoutes 未登记任何路由：registry 为空，覆盖审计口径会失真")
	}

	known := map[string]bool{}
	for _, spec := range IntentCatalog {
		if spec != nil {
			known[spec.Name] = true
		}
	}

	for name, routes := range intentRouteRegistry {
		if !known[name] {
			t.Errorf("登记的意图名 %q 不存在于 IntentCatalog（可能在 invokeRoutes 调用点写错）", name)
		}
		if len(routes) == 0 {
			t.Errorf("意图 %q 登记了空的路由表", name)
		}
		for action, target := range routes {
			parts := strings.Fields(strings.TrimSpace(target))
			if len(parts) != 2 {
				t.Errorf("意图 %q 的 action %q 声明格式错误（应为 \"METHOD PATH\"）: %q", name, action, target)
				continue
			}
			if !strings.HasPrefix(normalizeEndpointPath(parts[1]), "/system/api") {
				t.Errorf("意图 %q 的 action %q 路径未规范化: %q", name, action, parts[1])
			}
		}
	}

	targets := DeclaredEndpointTargets()
	if len(targets) == 0 {
		t.Fatal("声明端点汇总为空：capEndpoints 与 invokeRoutes 至少应贡献一批端点")
	}
	for _, d := range DeclaredTargetsList() {
		if len(d.Via) == 0 {
			t.Errorf("端点 %s 缺少声明来源", d.Key)
		}
		if !strings.HasPrefix(d.Path, "/system/api") {
			t.Errorf("端点 %s 路径未带 /system/api 前缀", d.Key)
		}
		if d.Method != "GET" && d.Method != "POST" && d.Method != "DELETE" && d.Method != "PUT" {
			t.Errorf("端点 %s 的 method 非常规值，可能是声明里把路径和 method 写反了", d.Key)
		}
	}
	t.Logf("声明端点 %d 个（invokeRoutes 意图 %d 个，capEndpoints %d 条）",
		len(targets), len(intentRouteRegistry), len(capEndpoints))
}

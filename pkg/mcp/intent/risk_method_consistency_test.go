package intent

import (
	"strings"
	"testing"
)

// TestRiskNeverUnderestimatesHTTPMethod 风险判定不得低估端点的真实 HTTP 方法。
//
// 背景：ActionRisk 有 4 层判定（意图声明的端点路由 → api_invoke 特殊处理 →
// 动词约定兜底 → 意图整体风险）。动词兜底那层靠 action 名里的动词猜风险
// （risk.go 的 readVerbs/writeVerbs/destructiveVerbs），一旦某个 action 名
// 不含动词、却被映射到写端点，就可能被判成 RiskRead —— **审批门会直接放行
// 一次写操作**，这是权限链上的真漏洞。
//
// 这条测试穷举全部意图 × 全部 action 交叉验证，不依赖具体名单。
func TestRiskNeverUnderestimatesHTTPMethod(t *testing.T) {
	var under, over []string
	checked := 0
	for _, spec := range IntentCatalog {
		p, ok := spec.Params["action"]
		if !ok || len(p.Enum) == 0 {
			continue
		}
		routes, hasRoutes := intentRouteRegistry[spec.Name]
		for _, act := range p.Enum {
			r := ActionRisk(spec.Name, map[string]any{"action": act})
			target, ok2 := routes[act]
			if !hasRoutes || !ok2 {
				continue // 非端点类动作（cap/本地能力），由 handler 语义决定
			}
			method := strings.Fields(target)
			if len(method) == 0 {
				continue
			}
			m := strings.ToUpper(method[0])
			checked++
			isWriteHTTP := m == "POST" || m == "PUT" || m == "DELETE" || m == "PATCH"
			switch {
			case isWriteHTTP && r == RiskRead:
				under = append(under, spec.Name+"."+act+" "+m+" → RiskRead（审批门会放行写操作）")
			case m == "GET" && r == RiskDestructive:
				// GET 不该被判成破坏性，否则无谓要求用户确认
				over = append(over, spec.Name+"."+act+" GET → RiskDestructive")
			}
		}
	}
	if checked < 50 {
		t.Fatalf("只校验了 %d 条端点动作，判定条件可能已失效（路由表没被填上）", checked)
	}
	for _, s := range under {
		t.Errorf("风险低估：%s", s)
	}
	for _, s := range over {
		t.Errorf("风险过严：%s", s)
	}
}

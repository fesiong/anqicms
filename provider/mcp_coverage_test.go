package provider

import (
	"os"
	"strings"
	"testing"
)

// TestAuditCapabilityCoverage 运行覆盖审计：把 manage.go 的 REST 端点与
// 意图层声明的可达端点对账，产出缺口报告。报告写入 doc/mcp-coverage-report.md。
//
// 两个真相源都不会随时间漂移：端点来自 route/manage.go 的解析，
// AI 可达性来自 intent.DeclaredEndpointTargets（invokeRoutes + capEndpoints 声明）。
func TestAuditCapabilityCoverage(t *testing.T) {
	svc := testService()

	managePath, err := FindManageRoute()
	if err != nil {
		t.Skipf("跳过：无法定位 route/manage.go（%v）", err)
	}
	rep, err := svc.AuditCapabilityCoverage(managePath)
	if err != nil {
		t.Fatalf("覆盖审计失败: %v", err)
	}

	if rep.TotalEndpint == 0 {
		t.Fatal("manage.go 解析出 0 个端点，解析器可能失效")
	}
	if rep.TotalDeclared == 0 {
		t.Fatal("意图层声明了 0 个可达端点，DeclaredEndpointTargets 可能失效")
	}
	// 悬空声明 = 意图层写了不存在的端点，调用必然 404。声明式映射的唯一防线就在这里。
	if len(rep.OrphanTargets) > 0 {
		t.Errorf("意图层声明了 %d 个不存在的端点: %v", len(rep.OrphanTargets), rep.OrphanTargets)
	}
	// 分层之和必须等于端点总数，否则有端点被漏判（新增分层时最容易忘）。
	if got := rep.Direct + rep.Gated + rep.Generic + rep.Blocked; got != rep.TotalEndpint {
		t.Errorf("分层计数之和 %d != 端点总数 %d（有端点未参与判定）", got, rep.TotalEndpint)
	}

	t.Logf("端点=%d 声明可达=%d 意图=%d 无端点能力=%d 本地能力=%d",
		rep.TotalEndpint, rep.TotalDeclared, rep.TotalIntent, rep.TotalCap, rep.TotalBuiltin)
	t.Logf("direct=%d gated=%d generic=%d blocked=%d 覆盖率=%.1f%%",
		rep.Direct, rep.Gated, rep.Generic, rep.Blocked,
		float64(rep.Direct+rep.Gated)/float64(rep.TotalEndpint)*100)
	for _, s := range rep.NsStats {
		t.Logf("  ns=%-12s total=%-4d direct=%-3d gated=%-3d generic=%-3d blocked=%-3d rate=%.1f%%",
			s.NS, s.Total, s.Direct, s.Gated, s.Generic, s.Blocked, s.CoverRate*100)
	}

	// 产出报告，供 G 阶段补齐能力层时查阅
	md := rep.Markdown(40)
	out := strings.TrimSuffix(managePath, "route/manage.go") + "doc/mcp-coverage-report.md"
	if err := os.WriteFile(out, []byte(md), 0o644); err != nil {
		t.Logf("报告写入失败（不影响审计结论）: %v", err)
	} else {
		t.Logf("报告已写入 %s", out)
	}
}

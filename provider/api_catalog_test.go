package provider

import (
	"fmt"
	"sort"
	"strings"
	"testing"

	"kandaoni.com/anqicms/pkg/mcp/intent"
)

// TestBuildAPICatalog 验证 L0 元数据层能从源码派生出端点卡片与参数 schema。
func TestBuildAPICatalog(t *testing.T) {
	ResetAPICatalogCache()
	cat, err := BuildAPICatalog()
	if err != nil {
		t.Fatalf("构建元数据失败: %v", err)
	}
	if len(cat.Endpoints) == 0 {
		t.Fatal("端点数为 0")
	}
	fmt.Printf("%+v", cat.Endpoints)

	st := cat.CatalogStats()
	fmt.Printf("\n=== L0 元数据层构建结果 ===\n")
	fmt.Printf("端点总数        : %d\n", len(cat.Endpoints))
	fmt.Printf("结构体 schema   : %d\n", st.Struct)
	fmt.Printf("URLParam 提取   : %d\n", st.URLParam)
	fmt.Printf("无参数          : %d\n", st.None)
	fmt.Printf("表单/上传       : %d\n", st.Multipart)
	fmt.Printf("解析到结构体数量 : %d\n", cat.StructTouched)

	// 端点总数与 ParseManageRoutes 交叉校验。
	// 刻意不写死数字：manage.go 增删路由是常态，写死会让每次路由变更都假失败。
	managePath, perr := FindManageRoute()
	if perr != nil {
		t.Fatalf("定位 route/manage.go 失败: %v", perr)
	}
	routes, rerr := ParseManageRoutes(managePath)
	if rerr != nil {
		t.Fatalf("解析路由失败: %v", rerr)
	}
	if len(cat.Endpoints) != len(routes) {
		t.Fatalf("端点总数期望 %d（ParseManageRoutes），实得 %d", len(routes), len(cat.Endpoints))
	}

	// 必须有一定比例的端点能拿到字段级 schema，否则这层没有价值
	if st.Struct == 0 {
		t.Fatal("没有任何端点解析出结构体 schema，元数据层无效")
	}

	// 抽样展示：找几个已知的端点
	samples := []struct{ method, path string }{
		{"POST", "/system/api/archive/detail"},
		{"GET", "/system/api/archive/list"},
		{"GET", "/system/api/module/detail"},
	}
	fmt.Printf("\n=== 抽样 ===")
	for _, s := range samples {
		ep, ok := cat.FindEndpoint(s.method, s.path)
		if !ok {
			fmt.Printf("\n%s %s : 未找到\n", s.method, s.path)
			continue
		}
		fmt.Printf("\n%s %s\n", ep.Method, ep.Path)
		fmt.Printf("  handler=%s ns=%s resource=%s risk=%s source=%s struct=%s\n",
			ep.Handler, ep.NS, ep.Resource, ep.Risk, ep.ParamSource, ep.StructType)
		fmt.Printf("  参数 %d 个: ", len(ep.Params))
		n := 0
		for _, p := range ep.Params {
			if n >= 6 {
				fmt.Printf(" ...")
				break
			}
			req := ""
			if p.Required {
				req = "*"
			}
			fmt.Printf("%s%s:%s ", p.Name, req, p.Type)
			n++
		}
		fmt.Println()
	}
}

// TestAllEndpointsHaveDomain 穷举校验：394 个端点必须全部落到某个能力域。
//
// 这是 ns→域 映射表不漂移的唯一保障。新增后台模块而忘记登记时，
// DomainOfPath 会返回 unknown，本测试立即失败——而不是让该端点悄悄失去按域裁剪的能力。
func TestAllEndpointsHaveDomain(t *testing.T) {
	ResetAPICatalogCache()
	cat, err := BuildAPICatalog()
	if err != nil {
		t.Fatalf("构建元数据失败: %v", err)
	}

	byDomain := map[string]int{}
	unmapped := make([]string, 0)
	for _, ep := range cat.Endpoints {
		if ep.Domain == "" || ep.Domain == string(intent.DomainUnknown) {
			unmapped = append(unmapped, ep.Method+" "+ep.Path)
			continue
		}
		byDomain[ep.Domain]++
	}

	if len(unmapped) > 0 {
		t.Fatalf("有 %d 个端点未登记能力域，需在 pkg/mcp/intent/domain.go 补映射：\n  %s",
			len(unmapped), strings.Join(unmapped, "\n  "))
	}

	total := 0
	for _, n := range byDomain {
		total += n
	}
	if total != len(cat.Endpoints) {
		t.Fatalf("归域合计 %d != 端点总数 %d", total, len(cat.Endpoints))
	}

	fmt.Printf("\n=== 端点按能力域分布（合计 %d）===\n", total)
	keys := make([]string, 0, len(byDomain))
	for k := range byDomain {
		keys = append(keys, k)
	}
	sort.Strings(keys)
	for _, k := range keys {
		fmt.Printf("  %-12s %3d  %s\n", k, byDomain[k], intent.DomainLabel(intent.Domain(k)))
	}
}

// TestDomainOfKnownPaths 锁定几处有代表性的归域判定，防止映射表被误改。
// 尤其 setting/nav——它与 setting 共享一级 ns，只能靠二级路径区分。
func TestDomainOfKnownPaths(t *testing.T) {
	cases := []struct {
		path string
		want intent.Domain
	}{
		{"/system/api/setting/system", intent.DomainSystem},      // 一级 ns 判定
		{"/system/api/setting/nav/list", intent.DomainStructure}, // 二级例外：导航属站点结构
		{"/system/api/plugin/push/baidu", intent.DomainSeo},      // plugin 必须看二级
		{"/system/api/plugin/backup/restore", intent.DomainSiteOps},
		{"/system/api/plugin/user/list", intent.DomainCommerce},
		{"/system/api/plugin/material/list", intent.DomainContentOps},
		{"/system/api/admin/list", intent.DomainAccount},
		{"/system/api/statistic/spider", intent.DomainTraffic},
		{"/system/api/archive/list", intent.DomainContent},
		{"/system/api/attachment/list", intent.DomainMedia},
		{"plugin/push/baidu", intent.DomainSeo}, // 相对路径同样可判定
	}
	for _, c := range cases {
		if got := intent.DomainOfPath(c.path); got != c.want {
			t.Errorf("%s 归域错误：期望 %s，实得 %s", c.path, c.want, got)
		}
	}

	// 未登记的 ns 必须暴露为 unknown，而不是被兜底成某个域掩盖问题
	if got := intent.DomainOfPath("/system/api/no_such_ns/foo"); got != intent.DomainUnknown {
		t.Errorf("未登记 ns 应返回 unknown，实得 %s", got)
	}
}

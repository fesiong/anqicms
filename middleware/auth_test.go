package middleware

import (
	"go/ast"
	"go/parser"
	"go/printer"
	"go/token"
	"strings"
	"testing"

	"kandaoni.com/anqicms/config"
)

// 本文件锁住 T9a 的「严格菜单权限」契约（R82）：
//
//  1. /channels party 必须挂 AdminPermissionRequired —— 挂回 AdminPermission 就等于
//     把「未知菜单默认放行」这条既有行为带进渠道配置，而渠道端点是最该 fail closed 的一类；
//  2. 负对照：注入一份**不含**渠道菜单的菜单清单，AdminMenuPathForURI 必须返回 mapped=false。
//     「加了菜单就能映射」单独成立不算证明 —— 它无法区分「解析器在工作」与「解析器恒返回 true」；
//  3. 正对照：注入含渠道菜单的清单 → /plugin/channels；
//  4. 生产清单必须覆盖 T9a 的全部 URI，否则严格版会把它们全部拒掉。
func TestChannelRoutesRequireChannelMenuPermission(t *testing.T) {
	// ---- 1) 路由必须使用严格版中间件 ----
	middlewares := channelPartyMiddlewares(t)
	if len(middlewares) == 0 {
		t.Fatal("route/manage.go 里没有 channels party 的中间件列表，断言已空转")
	}
	hasStrict := false
	for _, mw := range middlewares {
		// 精确比较：AdminPermissionRequired 含 AdminPermission 前缀，用 Contains 会假通过。
		if mw == "middleware.AdminPermissionRequired" {
			hasStrict = true
		}
		if mw == "middleware.AdminPermission" {
			t.Fatalf("/channels party 仍挂着宽松版 middleware.AdminPermission（实得 %v）", middlewares)
		}
	}
	if !hasStrict {
		t.Fatalf("/channels party 未挂 middleware.AdminPermissionRequired（实得 %v）", middlewares)
	}

	// ---- 2) 负对照：菜单清单里没有渠道条目时必须 mapped=false ----
	withoutChannelMenu := []*config.MenuGroup{
		{
			Key:  "plugin",
			Name: "功能资源",
			Menus: []*config.Menu{
				{Path: "/plugin/index", GroupKey: "plugin", Name: "功能列表", Backend: "/plugin/index"},
			},
		},
	}
	if front, mapped := AdminMenuPathForURI("/channels/detail", withoutChannelMenu); mapped || front != "" {
		t.Fatalf("菜单里没有 /channels 时仍解析出了 %q（mapped=%v），严格版会把权限检查变成摆设", front, mapped)
	}

	// ---- 3) 正对照：同一份注入清单加回渠道菜单后必须能映射 ----
	withChannelMenu := append([]*config.MenuGroup{}, withoutChannelMenu...)
	withChannelMenu[0].Menus = append(withChannelMenu[0].Menus, &config.Menu{
		Path: "/plugin/channels", GroupKey: "plugin", Name: "消息渠道", Backend: "/channels",
	})
	if front, mapped := AdminMenuPathForURI("/channels/detail", withChannelMenu); !mapped || front != "/plugin/channels" {
		t.Fatalf("注入渠道菜单后解析结果为 %q（mapped=%v），期望 /plugin/channels", front, mapped)
	}

	// ---- 4) 生产清单覆盖 T9a 的全部 URI ----
	for _, uri := range []string{
		"/channels",
		"/channels/detail",
		"/channels/delete",
		"/channels/status",
		"/channels/capabilities",
	} {
		front, mapped := AdminMenuPathForURI(uri, config.MenuGroups())
		if !mapped {
			t.Errorf("生产菜单清单无法映射 %s：严格版权限检查会把它整条拒掉", uri)
			continue
		}
		if front != "/plugin/channels" {
			t.Errorf("%s 映射到 %s，期望 /plugin/channels", uri, front)
		}
	}

	// ---- 5) 宽松版行为不变：未映射 → 放行（R82「现有 AdminPermission 行为不变」）----
	body := adminPermissionBody(t)
	if !strings.Contains(body, "AdminMenuPathForURI(") {
		t.Error("AdminPermission 未复用 AdminMenuPathForURI，两套解析迟早漂移")
	}
	if !strings.Contains(body, "if !mapped {") || !strings.Contains(body, "ctx.Next()") {
		t.Error("AdminPermission 的未映射分支必须仍然是 ctx.Next()（放行）")
	}
}

// channelPartyMiddlewares 取出 route/manage.go 里 `channels := ...Party("/channels", ...)` 的中间件实参文本。
func channelPartyMiddlewares(t *testing.T) []string {
	t.Helper()
	fset := token.NewFileSet()
	file, err := parser.ParseFile(fset, "../route/manage.go", nil, parser.ParseComments)
	if err != nil {
		t.Fatalf("解析 ../route/manage.go 失败：%v", err)
	}
	var middlewares []string
	ast.Inspect(file, func(n ast.Node) bool {
		assign, ok := n.(*ast.AssignStmt)
		if !ok || len(assign.Lhs) != 1 || len(assign.Rhs) != 1 {
			return true
		}
		ident, ok := assign.Lhs[0].(*ast.Ident)
		if !ok || ident.Name != "channels" {
			return true
		}
		call, ok := assign.Rhs[0].(*ast.CallExpr)
		if !ok || len(call.Args) == 0 {
			return true
		}
		selector, ok := call.Fun.(*ast.SelectorExpr)
		if !ok || selector.Sel.Name != "Party" {
			return true
		}
		lit, ok := call.Args[0].(*ast.BasicLit)
		if !ok || lit.Value != `"/channels"` {
			return true
		}
		for _, arg := range call.Args[1:] {
			var sb strings.Builder
			if err := printer.Fprint(&sb, fset, arg); err == nil {
				middlewares = append(middlewares, sb.String())
			}
		}
		return true
	})
	return middlewares
}

// adminPermissionBody 返回 middleware/auth.go 里 AdminPermission 的函数体源码。
func adminPermissionBody(t *testing.T) string {
	t.Helper()
	fset := token.NewFileSet()
	file, err := parser.ParseFile(fset, "auth.go", nil, parser.ParseComments)
	if err != nil {
		t.Fatalf("解析 auth.go 失败：%v", err)
	}
	var body string
	ast.Inspect(file, func(n ast.Node) bool {
		decl, ok := n.(*ast.FuncDecl)
		if !ok || decl.Name == nil || decl.Name.Name != "AdminPermission" || decl.Body == nil {
			return true
		}
		var sb strings.Builder
		if err := printer.Fprint(&sb, fset, decl.Body); err == nil {
			body = sb.String()
		}
		return true
	})
	if body == "" {
		t.Fatal("没有找到 AdminPermission 函数体")
	}
	return body
}

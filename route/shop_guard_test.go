package route

import (
	"go/ast"
	"go/parser"
	"go/token"
	"os"
	"strings"
	"testing"
)

// 守卫路由清单：普通模式（ShopMode=0）下必须返回 404 的商城前台 API。
// 新增商城前台路由时必须：① 挂 middleware.ShopModeGuard；② 在此清单登记。
// 本测试用 AST 解析 route/base.go（声明式路由必须穷举校验，禁止正则断言总数）。
var expectedShopGuardRoutes = map[string]bool{
	"GET /orders/statistics":           true,
	"GET /orders/chart":                true,
	"POST /order/checkout":             true,
	"GET /order/services":              true,
	"GET /order/custom_form":           true,
	"POST /order/custom_form":          true,
	"GET /place/countries":             true,
	"GET /place/country/states":        true,
	"GET /place/country/cities":        true,
	"POST /place/autocomplete":         true,
	"GET /activity/list":               true,
	"GET /activity/detail":             true,
	"GET /archive/sku":                 true,
	"GET /archive/activity":            true,
	"GET /wishlist/list":               true,
	"POST /wishlist/add":               true,
	"POST /wishlist/delete":            true,
	"GET /coupon/valid":                true,
	"GET /coupon/codes":                true,
	"POST /coupon/code":                true,
	"POST /coupon/check":               true,
	"GET /shipping/info":               true,
	"GET /cart/list":                   true,
	"POST /cart/add":                   true,
	"POST /cart/update":                true,
	"POST /cart/remove":                true,
	"GET /communication/fields":        true,
	"GET /communication/orders":        true,
	"POST /communication/order/create": true,
	"GET /communication/order/detail":  true,
	"POST /communication/order/update": true,
	"POST /communication/order/accept": true,
	"GET /communication/messages":      true,
	"POST /communication/message/send": true,
	"GET /review/list":                 true,
	"POST /review/publish":             true,
	"POST /review/praise":              true,
	"POST /stripe/pay":                 true, // 注册于 notify Party，实际完整路径 /notify/stripe/pay
}

// TestShopGuardRoutesLocked 解析 route/base.go，收集所有挂了 ShopModeGuard 的路由，
// 断言与清单完全一致：防止 ① 守卫被误删 ② 清单与实现漂移。
func TestShopGuardRoutesLocked(t *testing.T) {
	src, err := os.ReadFile("base.go")
	if err != nil {
		t.Fatalf("读取 base.go 失败: %v", err)
	}
	fset := token.NewFileSet()
	f, err := parser.ParseFile(fset, "base.go", src, 0)
	if err != nil {
		t.Fatalf("解析 base.go 失败: %v", err)
	}

	guarded := map[string]bool{}
	ast.Inspect(f, func(n ast.Node) bool {
		call, ok := n.(*ast.CallExpr)
		if !ok {
			return true
		}
		// 形如 xxx.Get/xxx.Post(...)，且参数中出现 ShopModeGuard
		sel, ok := call.Fun.(*ast.SelectorExpr)
		if !ok {
			return true
		}
		method := ""
		switch sel.Sel.Name {
		case "Get":
			method = "GET"
		case "Post":
			method = "POST"
		default:
			return true
		}
		if len(call.Args) < 2 {
			return true
		}
		pathLit, ok := call.Args[0].(*ast.BasicLit)
		if !ok {
			return true
		}
		hasGuard := false
		for _, arg := range call.Args[1:] {
			if id, ok := arg.(*ast.SelectorExpr); ok {
				if id.Sel != nil && id.Sel.Name == "ShopModeGuard" {
					hasGuard = true
					break
				}
			}
		}
		if !hasGuard {
			return true
		}
		path := strings.Trim(pathLit.Value, "`\"")
		guarded[method+" "+path] = true
		return true
	})

	for route := range expectedShopGuardRoutes {
		if !guarded[route] {
			t.Errorf("路由 %s 在清单中但未挂 ShopModeGuard（或路径/方法不匹配）", route)
		}
	}
	for route := range guarded {
		if !expectedShopGuardRoutes[route] {
			t.Errorf("路由 %s 挂了 ShopModeGuard 但未登记清单（新商城路由必须两处同步）", route)
		}
	}
}

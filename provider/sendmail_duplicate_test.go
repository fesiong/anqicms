package provider

import (
	"go/ast"
	"testing"
)

// SuccessPaidOrder 只能发一次支付成功通知。
//
// 2026-10-24 商城合并（8ed8d9dd）在函数尾部又加了一次 SendPayOrderEmail，
// 与 2024 年就在的调用重复：同一订单的管理员通知邮箱会收到两封“已支付”，
// 间隔 3-4 秒（两次 SMTP 的耗时）。这里用 AST 数调用次数，防止合并/重构再引入。
func TestSuccessPaidOrderSendsPayEmailOnce(t *testing.T) {
	fn := mustFindProviderFunc(t, "order.go", "SuccessPaidOrder")

	count := 0
	ast.Inspect(fn, func(n ast.Node) bool {
		call, ok := n.(*ast.CallExpr)
		if !ok {
			return true
		}
		sel, ok := call.Fun.(*ast.SelectorExpr)
		if ok && sel.Sel.Name == "SendPayOrderEmail" {
			count++
		}
		return true
	})

	if count != 1 {
		t.Fatalf("SuccessPaidOrder 调用了 %d 次 SendPayOrderEmail，期望恰好 1 次", count)
	}
}

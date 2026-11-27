package provider

import (
	"bytes"
	"go/ast"
	"go/format"
	"go/parser"
	"go/token"
	"strings"
	"testing"
)

// renderGuardExpr 把 AST 节点还原成源码文本，避免用行号/偏移做脆弱断言。
func renderGuardExpr(fset *token.FileSet, n ast.Node) string {
	var b bytes.Buffer
	if err := format.Node(&b, fset, n); err != nil {
		return ""
	}
	return b.String()
}

// TestGeneratePaymentRejectsNonPositiveAmount 锁住「付款单金额必须为正」这道闸门。
//
// 背景：数量侧已在 CreateOrder 堵死负数数量，但订单金额仍可能从别的路径变负
// （历史脏数据、后台改单等）。余额支付里 `user.Balance < payment.Amount` 对负数
// 恒 false，`balance - (-N)` 就把扣款变成给攻击者加钱。
//
// 断言（AST，不依赖数据库，本机无 MySQL 也能跑）：
//   - GeneratePayment 函数体第一条语句就是 `amount <= 0` 的拦截 if；
//   - 该 if 必须 return error；
//   - 拦截必须排在「复用已有 payment」和「落库 Save」之前。
//
// 反向对照：把校验条件改成 `order.Amount > 0`（或删掉、或挪到 Save 之后），本测试立刻红。
func TestGeneratePaymentRejectsNonPositiveAmount(t *testing.T) {
	fset := token.NewFileSet()
	file, err := parser.ParseFile(fset, "order.go", nil, 0)
	if err != nil {
		t.Fatalf("解析 order.go 失败: %v", err)
	}

	var fn *ast.FuncDecl
	for _, d := range file.Decls {
		if f, ok := d.(*ast.FuncDecl); ok && f.Name.Name == "GeneratePayment" && f.Recv != nil {
			fn = f
			break
		}
	}
	if fn == nil {
		t.Fatal("护栏空转：没找到 (*Website).GeneratePayment")
	}

	// guard 必须是函数体第一条语句：先于 GetPaymentInfoByOrderId 复用与 Save 落库。
	if len(fn.Body.List) == 0 {
		t.Fatal("GeneratePayment 函数体为空")
	}
	guard, ok := fn.Body.List[0].(*ast.IfStmt)
	if !ok {
		t.Fatal("GeneratePayment 第一条语句必须是金额非正的拦截 if（先于复用已有付款单）")
	}
	cond := renderGuardExpr(fset, guard.Cond)
	if !strings.Contains(cond, "Amount") || !strings.Contains(cond, "<=") || !strings.Contains(cond, "0") {
		t.Fatalf("金额拦截条件必须是 `amount <= 0`，实际: %s", cond)
	}

	// guard 内必须 return error（不能只 log 或 continue）。
	hasReturn := false
	ast.Inspect(guard.Body, func(n ast.Node) bool {
		if _, ok := n.(*ast.ReturnStmt); ok {
			hasReturn = true
		}
		return true
	})
	if !hasReturn {
		t.Fatal("金额非法时必须 return error，不能继续生成 payment")
	}

	// 落库 Save(payment) 必须出现在 guard 之后。
	savePos := token.NoPos
	ast.Inspect(fn.Body, func(n ast.Node) bool {
		if call, ok := n.(*ast.CallExpr); ok {
			if sel, ok := call.Fun.(*ast.SelectorExpr); ok && sel.Sel.Name == "Save" {
				if savePos == token.NoPos {
					savePos = call.Pos()
				}
			}
		}
		return true
	})
	if savePos == token.NoPos {
		t.Fatal("护栏空转：GeneratePayment 里没找到落库 Save 调用")
	}
	if guard.Pos() >= savePos {
		t.Fatal("金额拦截必须排在落库 Save 之前，否则负金额仍会写进 payments 表")
	}
}

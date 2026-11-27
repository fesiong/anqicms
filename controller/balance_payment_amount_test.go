package controller

import (
	"bytes"
	"go/ast"
	"go/format"
	"go/parser"
	"go/token"
	"strings"
	"testing"
)

func renderBalanceGuard(fset *token.FileSet, n ast.Node) string {
	var b bytes.Buffer
	if err := format.Node(&b, fset, n); err != nil {
		return ""
	}
	return b.String()
}

// TestCreateBalancePaymentRejectsNonPositiveAmount 锁住余额支付的金额闸门。
//
// 历史漏洞：payment.Amount 为负时，`user.Balance < payment.Amount` 恒 false，
// 绕过余额检查后 `balance - (-N)` 给攻击者反向加钱。数量侧已在 CreateOrder 堵死
// 负数量，这里是最后一道防线。
//
// 断言（AST，纯文本，不依赖数据库）：
//   - createBalancePayment 内存在 `payment.Amount <= 0` 的拦截 if；
//   - 该 if 必须排在余额比较、Begin 事务、扣款 UpdateColumn 之前；
//   - 该 if 内必须 return（fail closed）。
//
// 反向对照：删掉该校验、把它挪到扣款之后、或改成 `> 0`，本测试立刻红。
func TestCreateBalancePaymentRejectsNonPositiveAmount(t *testing.T) {
	fset := token.NewFileSet()
	file, err := parser.ParseFile(fset, "apiOrder.go", nil, 0)
	if err != nil {
		t.Fatalf("解析 apiOrder.go 失败: %v", err)
	}

	var fn *ast.FuncDecl
	for _, d := range file.Decls {
		if f, ok := d.(*ast.FuncDecl); ok && f.Name.Name == "createBalancePayment" && f.Recv == nil {
			fn = f
			break
		}
	}
	if fn == nil {
		t.Fatal("护栏空转：没找到 createBalancePayment")
	}

	// 找到金额拦截 if（条件里含 Amount 与 <=）。
	var guard *ast.IfStmt
	ast.Inspect(fn.Body, func(n ast.Node) bool {
		if i, ok := n.(*ast.IfStmt); ok {
			c := renderBalanceGuard(fset, i.Cond)
			if strings.Contains(c, "Amount") && strings.Contains(c, "<=") {
				guard = i
			}
		}
		return true
	})
	if guard == nil {
		t.Fatal("createBalancePayment 缺少 `payment.Amount <= 0` 的金额拦截")
	}

	// guard 必须 return，不能只报错后继续走扣款。
	hasReturn := false
	ast.Inspect(guard.Body, func(n ast.Node) bool {
		if _, ok := n.(*ast.ReturnStmt); ok {
			hasReturn = true
		}
		return true
	})
	if !hasReturn {
		t.Fatal("金额非正的拦截必须 return，不能继续执行余额扣款")
	}

	// 关键动作的源码位置：余额比较 / Begin 事务 / 扣款 UpdateColumn。
	// 三者都必须排在 guard 之后。
	var balanceCmpPos, beginPos, deductPos token.Pos
	ast.Inspect(fn.Body, func(n ast.Node) bool {
		switch node := n.(type) {
		case *ast.IfStmt:
			c := renderBalanceGuard(fset, node.Cond)
			if strings.Contains(c, "user.Balance") && strings.Contains(c, "payment.Amount") &&
				!strings.Contains(c, "<=") && balanceCmpPos == token.NoPos {
				balanceCmpPos = node.Pos()
			}
		case *ast.CallExpr:
			src := renderBalanceGuard(fset, node)
			if strings.Contains(src, ".Begin(") && beginPos == token.NoPos {
				beginPos = node.Pos()
			}
			if sel, ok := node.Fun.(*ast.SelectorExpr); ok && sel.Sel.Name == "UpdateColumn" {
				if strings.Contains(src, "balance") && deductPos == token.NoPos {
					deductPos = node.Pos()
				}
			}
		}
		return true
	})
	if balanceCmpPos == token.NoPos {
		t.Fatal("护栏空转：createBalancePayment 里没找到余额比较")
	}
	if beginPos == token.NoPos {
		t.Fatal("护栏空转：createBalancePayment 里没找到 Begin 事务")
	}
	if deductPos == token.NoPos {
		t.Fatal("护栏空转：createBalancePayment 里没找到余额扣款 UpdateColumn")
	}
	if guard.Pos() >= balanceCmpPos {
		t.Fatal("金额拦截必须排在余额比较之前")
	}
	if guard.Pos() >= beginPos {
		t.Fatal("金额拦截必须排在 Begin 事务之前")
	}
	if guard.Pos() >= deductPos {
		t.Fatal("金额拦截必须排在扣款 UpdateColumn 之前")
	}
}

package provider

import (
	"go/ast"
	"go/parser"
	"go/token"
	"testing"
)

// TestCreateOrderChecksStockUpdateErrors 下单流程里每条库存/销量写入都必须消费 Error。
//
// 客户实测：quantity=-1 时数据库报 Out of range value for column 'sold_count'，
// 但接口仍返回成功。根因就是 `tx.Model(...).UpdateColumns(...)` 被当成语句直接调用，
// 返回值（含 Error）被丢掉，事务照样 Commit。
//
// 这条用 AST 断言：CreateOrder 函数体内不允许出现「裸调用」的 UpdateColumn(s)。
// 反向对照：把任意一处改回 `tx.Model(...).UpdateColumns(...)`（不接 Error），立刻红。
func TestCreateOrderChecksStockUpdateErrors(t *testing.T) {
	fset := token.NewFileSet()
	file, err := parser.ParseFile(fset, "order.go", nil, 0)
	if err != nil {
		t.Fatalf("解析 order.go 失败: %v", err)
	}
	var fn *ast.FuncDecl
	for _, d := range file.Decls {
		if f, ok := d.(*ast.FuncDecl); ok && f.Name.Name == "CreateOrder" && f.Recv != nil {
			fn = f
			break
		}
	}
	if fn == nil {
		t.Fatal("没找到 (*Website).CreateOrder")
	}

	// isUpdateColumnCall 判断表达式里是否出现了 .UpdateColumn / .UpdateColumns 调用
	var isUpdateColumnCall func(expr ast.Expr) bool
	isUpdateColumnCall = func(expr ast.Expr) bool {
		switch e := expr.(type) {
		case *ast.CallExpr:
			if sel, ok := e.Fun.(*ast.SelectorExpr); ok {
				if sel.Sel.Name == "UpdateColumn" || sel.Sel.Name == "UpdateColumns" {
					return true
				}
				// 链式调用：tx.Model(...).Where(...).UpdateColumns(...) 的内层
				return isUpdateColumnCall(sel.X)
			}
			return isUpdateColumnCall(e.Fun)
		case *ast.SelectorExpr:
			return isUpdateColumnCall(e.X)
		}
		return false
	}

	bad, checked := 0, 0
	countAssign := func(as *ast.AssignStmt) {
		for _, r := range as.Rhs {
			if isUpdateColumnCall(r) {
				checked++
			}
		}
	}
	ast.Inspect(fn.Body, func(n ast.Node) bool {
		switch st := n.(type) {
		case *ast.ExprStmt:
			if isUpdateColumnCall(st.X) {
				bad++
				t.Errorf("%s: 库存/销量更新没有检查 Error，数据库报错后事务仍会提交",
					fset.Position(st.Pos()))
			}
		case *ast.AssignStmt:
			countAssign(st)
		case *ast.IfStmt:
			// if uErr := tx....UpdateColumns(...).Error; uErr != nil { ... }
			if as, ok := st.Init.(*ast.AssignStmt); ok {
				countAssign(as)
			}
		}
		return true
	})
	// 防空转：函数里必须真的有「接住 Error」的 UpdateColumn 调用，
	// 否则函数被重构成别的写法后，上面的检查会永远通过。
	if checked == 0 {
		t.Fatal("护栏空转：CreateOrder 里没有检测到任何被检查 Error 的 UpdateColumn 调用")
	}
}

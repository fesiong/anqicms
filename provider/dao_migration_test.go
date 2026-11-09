package provider

import (
	"go/ast"
	"go/parser"
	"go/token"
	"testing"
)

// autoMigrateModels 解析 dao.go 中 AutoMigrateDB 的 models 清单，
// 返回其中注册的 model.X 类型名集合。只取 `var models = []interface{}{...}`
// 这个字面量本身，不把函数体里其它 &model.X{} 引用算进来。
func autoMigrateModels(t *testing.T) map[string]bool {
	t.Helper()
	fset := token.NewFileSet()
	f, err := parser.ParseFile(fset, "dao.go", nil, 0)
	if err != nil {
		t.Fatalf("解析 dao.go 失败：%v", err)
	}
	var fn *ast.FuncDecl
	for _, decl := range f.Decls {
		if fd, ok := decl.(*ast.FuncDecl); ok && fd.Name.Name == "AutoMigrateDB" {
			fn = fd
			break
		}
	}
	if fn == nil || fn.Body == nil {
		t.Fatal("dao.go 中找不到 AutoMigrateDB 函数体")
	}

	var list *ast.CompositeLit
	ast.Inspect(fn.Body, func(n ast.Node) bool {
		vs, ok := n.(*ast.ValueSpec)
		if !ok {
			return true
		}
		for _, name := range vs.Names {
			if name.Name != "models" {
				continue
			}
			for _, v := range vs.Values {
				if cl, ok := v.(*ast.CompositeLit); ok {
					list = cl
				}
			}
		}
		return true
	})
	if list == nil {
		t.Fatal("找不到 AutoMigrateDB 的 models 清单字面量（提取逻辑失效，护栏会空转）")
	}

	models := map[string]bool{}
	for _, elt := range list.Elts {
		unary, ok := elt.(*ast.UnaryExpr)
		if !ok || unary.Op != token.AND {
			continue
		}
		lit, ok := unary.X.(*ast.CompositeLit)
		if !ok {
			continue
		}
		sel, ok := lit.Type.(*ast.SelectorExpr)
		if !ok {
			continue
		}
		pkg, ok := sel.X.(*ast.Ident)
		if !ok || pkg.Name != "model" {
			continue
		}
		models[sel.Sel.Name] = true
	}
	if len(models) == 0 {
		t.Fatal("models 清单里没解析出任何 model.X{}（提取逻辑失效，护栏会空转）")
	}
	return models
}

// TestSubscribedUserRegisteredForMigration 锁住 SubscribedUser 进入自动迁移清单。
//
// 订阅消息事件（controller/apiNotify.go 的 SubscribeMsgPopup / SubscribeMsgChange）
// 真实对 subscribed_users 表做 FirstOrCreate / Delete，且不检查 .Error。
// 漏注册会让老站库缺表，订阅写入静默失败。把这一行从清单删掉，本测试必须变红。
func TestSubscribedUserRegisteredForMigration(t *testing.T) {
	models := autoMigrateModels(t)
	// 反向对照：连一直都在的 Admin 都解析不到，说明是提取逻辑坏了，而不是清单真缺项。
	if !models["Admin"] {
		t.Fatal("反向对照失败：Admin 未在解析结果中，清单提取逻辑失效")
	}
	if !models["SubscribedUser"] {
		t.Error("SubscribedUser 未加入 AutoMigrateDB 的 models 清单 —— subscribed_users 表不会被自动创建")
	}
}

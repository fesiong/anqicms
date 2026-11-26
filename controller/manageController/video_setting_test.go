package manageController

import (
	"go/ast"
	"go/token"
	"testing"
)

func TestSettingContentFormPersistsNormalizedVideoConfig(t *testing.T) {
	fn := mustFindFunc(t, "setting.go", "SettingContentForm")
	if findSelector(fn, "SaveSettingValue") == "" {
		t.Fatal("SettingContentForm 应保存内容设置，测试前提不成立")
	}

	var normalizePos, assignPos, savePos token.Pos
	ast.Inspect(fn, func(n ast.Node) bool {
		switch node := n.(type) {
		case *ast.CallExpr:
			if exprString(node.Fun) == "currentSite.SaveSettingValue" {
				savePos = node.Pos()
			}
			selector, ok := node.Fun.(*ast.SelectorExpr)
			if !ok || selector.Sel.Name != "Normalize" {
				break
			}
			if exprString(selector.X) == "req.Video" {
				normalizePos = node.Pos()
			}
			if exprString(selector.X) == "currentSite.SaveSettingValue" && normalizePos == 0 {
				t.Error("视频配置必须在保存前归一化")
			}
		case *ast.AssignStmt:
			for _, lhs := range node.Lhs {
				if exprString(lhs) != "currentSite.Content.Video" {
					continue
				}
				for _, rhs := range node.Rhs {
					if exprString(rhs) == "req.Video" {
						assignPos = node.Pos()
					}
				}
			}
		}
		return true
	})

	if normalizePos == 0 {
		t.Error("SettingContentForm 必须调用 req.Video.Normalize()")
	}
	if assignPos == 0 {
		t.Error("SettingContentForm 必须显式写入 currentSite.Content.Video 保存白名单")
	}
	if normalizePos > savePos || assignPos > savePos {
		t.Error("视频配置归一化和白名单赋值必须发生在 SaveSettingValue 之前")
	}
}

func exprString(expr ast.Expr) string {
	switch node := expr.(type) {
	case *ast.Ident:
		return node.Name
	case *ast.SelectorExpr:
		return exprString(node.X) + "." + node.Sel.Name
	default:
		return ""
	}
}

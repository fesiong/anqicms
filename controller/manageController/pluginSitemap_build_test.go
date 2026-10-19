package manageController

import (
	"go/ast"
	"go/parser"
	"go/token"
	"testing"
)

// TestSitemapBuildMustNotPersistConfig 锁住「生成端点不得写配置」这一不变式。
//
// 2026-10-17 实测：PluginSitemapBuild 里曾有一段「先保存一次」，把请求体当作
// 全量配置覆盖已存配置。而前端「手动操作」tab 发出的是设置表单的当前值，
// 一旦该表单取不到值（getFieldsValue 返回空/残缺），点一次「生成」就会把真实
// 配置清成默认值：type 回落 txt、auto_build 归零、排除项全部丢失。
// 更糟的是接口返回成功，没有任何报错线索，用户只会发现"配置怎么失效了"。
//
// 职责边界：保存只归 PluginSitemapForm，生成一律以已保存配置为准。
// 用 go/ast 断言而不是读函数返回值 —— 后者要起库起站点，CI 里跑不动。
func TestSitemapBuildMustNotPersistConfig(t *testing.T) {
	const srcFile = "pluginSitemap.go"

	buildFn := mustFindFunc(t, srcFile, "PluginSitemapBuild")
	for _, forbidden := range []string{"SaveSettingValue", "ReadJSON"} {
		if findSelector(buildFn, forbidden) != "" {
			t.Errorf("PluginSitemapBuild 不应调用 %s：生成端点必须只用已保存的配置，"+
				"否则前端传来的残缺表单值会覆盖真实配置", forbidden)
		}
	}

	// 反向对照：证明上面的断言不是空转 —— 保存端点本来就必须真的在保存。
	// 若哪天两个函数都不再调用 SaveSettingValue，这里会先失败，
	// 而不是让上面两条"恰好通过"。
	formFn := mustFindFunc(t, srcFile, "PluginSitemapForm")
	if findSelector(formFn, "SaveSettingValue") == "" {
		t.Fatal("PluginSitemapForm 应有 SaveSettingValue —— 测试前提不成立，断言已失去意义")
	}
}

// mustFindFunc 从指定源文件中取出顶层函数声明。
func mustFindFunc(t *testing.T, file, name string) *ast.FuncDecl {
	t.Helper()
	fset := token.NewFileSet()
	parsed, err := parser.ParseFile(fset, file, nil, 0)
	if err != nil {
		t.Fatalf("解析 %s 失败：%v", file, err)
	}
	for _, decl := range parsed.Decls {
		if fn, ok := decl.(*ast.FuncDecl); ok && fn.Name.Name == name {
			return fn
		}
	}
	t.Fatalf("%s 中找不到函数 %s", file, name)
	return nil
}

// findSelector 返回函数体内出现的指定方法/函数名，没有则返回空串。
func findSelector(fn *ast.FuncDecl, name string) string {
	var found string
	ast.Inspect(fn, func(n ast.Node) bool {
		if se, ok := n.(*ast.SelectorExpr); ok && se.Sel.Name == name {
			found = name
			return false
		}
		return true
	})
	return found
}

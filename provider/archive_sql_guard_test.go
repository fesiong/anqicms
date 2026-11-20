package provider

import (
	"go/ast"
	"go/parser"
	"go/token"
	"testing"
)

// TestNoToSQLInUserFacingListQueries 锁住 issue #93 的根治：
// 凡是把外部字符串（search q / module q / archives.q / archive.filename）汇入的
// 查询函数，体内都不得再调用 ToSQL —— 它内部 Dialector.Explain 会把 Vars 内联成
// 字面量，反斜杠 + 单引号可击穿字符串导致注入。必须走 dryRunSQL（保留 ? + Vars）
// 或直接参数化链式查询。
//
// 用 go/ast 而非黑盒：CI 无站点库，AST 断言稳定；反向对照由 dryRunSQL 的正向
// 存在性承担 —— 若某天 dryRunSQL 被删且这些函数也不再含 ToSQL，反向锚点会先红。
func TestNoToSQLInUserFacingListQueries(t *testing.T) {
	const srcFile = "archive.go"

	// filename 经 GetArchiveByFunc → selectArchiveByFunc 取整行。
	// 这条路径用参数化链式 Take（ops(...).Take），天然无 ToSQL 内联，正向锚点是
	// selectArchiveByFunc 的参数化 Take；两个函数体内都不得出现 ToSQL。
	for _, name := range []string{"GetArchiveByFunc", "selectArchiveByFunc"} {
		fn := mustFindProviderFunc(t, srcFile, name)
		if callName(fn, "ToSQL") != "" {
			t.Errorf("%s 不得调用 ToSQL（issue #93）：filename 等外部输入会经此查询", name)
		}
	}
	selectFn := mustFindProviderFunc(t, srcFile, "selectArchiveByFunc")
	if callName(selectFn, "Take") == "" {
		t.Error("selectArchiveByFunc 应以参数化 Take 取整行，否则反向对照失效")
	}

	listFn := mustFindProviderFunc(t, srcFile, "GetArchiveList")
	if callName(listFn, "ToSQL") != "" {
		t.Error("GetArchiveList 不得调用 ToSQL（issue #93）：search/module/archives 的 q 经此查询")
	}
	if callName(listFn, "getExplainCountParams") == "" {
		t.Error("GetArchiveList 应使用参数化 getExplainCountParams 做 EXPLAIN 估算")
	}
}

// TestTagListUsesParameterizedExplain 锁住 tag.go 的同类调用：
// tag 列表的 title 是外部输入，count 估算不得用内联 SQL。
func TestTagListUsesParameterizedExplain(t *testing.T) {
	fn := mustFindProviderFunc(t, "tag.go", "GetTagList")
	if callName(fn, "ToSQL") != "" {
		t.Error("GetTagList 不得调用 ToSQL（issue #93）：title 是外部输入")
	}
	if callName(fn, "GetExplainCountParams") == "" {
		t.Error("GetTagList 应使用 GetExplainCountParams 做参数化 EXPLAIN")
	}
}

// TestExplainExecUsesVars 断言 EXPLAIN 执行把 vars 传给 Raw，而不是字符串拼接
// —— 这是整个根治方案能成立的关键一行。oem 分支中真正执行的是 getExplainCountParams。
func TestExplainExecUsesVars(t *testing.T) {
	fn := mustFindProviderFunc(t, "archive.go", "getExplainCountParams")
	rawOK := false
	ast.Inspect(fn, func(n ast.Node) bool {
		call, ok := n.(*ast.CallExpr)
		if !ok {
			return true
		}
		sel, ok := call.Fun.(*ast.SelectorExpr)
		if !ok || sel.Sel.Name != "Raw" {
			return true
		}
		// Raw("EXPLAIN "+sql, vars...)：至少一个参数，且 Ellipsis 非 NoPos 表示用了 vars...
		if len(call.Args) >= 2 && call.Ellipsis.IsValid() {
			rawOK = true
		}
		return true
	})
	if !rawOK {
		t.Error("getExplainCountParams 必须以 Raw(sql, vars...) 参数化执行 EXPLAIN")
	}
}

func mustFindProviderFunc(t *testing.T, file, name string) *ast.FuncDecl {
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

// callName 返回函数体内是否调用了指定名字的方法/函数（SelectorExpr 的 Sel 或 Ident）。
func callName(fn *ast.FuncDecl, name string) string {
	found := ""
	ast.Inspect(fn, func(n ast.Node) bool {
		switch x := n.(type) {
		case *ast.SelectorExpr:
			if x.Sel.Name == name {
				found = name
			}
		case *ast.Ident:
			if x.Name == name {
				found = name
			}
		}
		return found == ""
	})
	return found
}

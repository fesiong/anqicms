package controller

import (
	"go/ast"
	"go/parser"
	"go/token"
	"os"
	"testing"
)

// tags 表在真实站点上会到十几万行，一次 SELECT count(*) 就是一次全索引扫描。
// 2026-10-28 的线上 CPU 打满就是这条语句被按 ?page=N 反复触发造成的。
// 以下断言锁住三件事，缺任何一条都会让那条慢查询回来：
//  1. 前台列表接口限制最大页码（后台不受限）；
//  2. list 模式把 needTotal 传成 false，跳过 count；
//  3. 后台分页仍然要精确总数。
func TestApiTagListEnforcesMaxPage(t *testing.T) {
	src := readFuncSource(t, "apiTag.go", "ApiTagList")

	mustContain(t, src, `currentPage\s*>\s*currentSite\.Content\.MaxPage`,
		"前台标签列表必须限制最大页码，否则扒站方按 ?page=N 能一路翻到十几万页")
}

// list 模式不需要精确总数，传 true 就等于每个请求都扫一遍 tags 全表。
func TestApiTagListListModeSkipsTotal(t *testing.T) {
	arg := getTagListNeedTotalArg(t, "apiTag.go", "ApiTagList")

	cmp, ok := arg.(*ast.BinaryExpr)
	if !ok || cmp.Op != token.EQL {
		t.Fatalf(`needTotal 必须由 listType == "page" 决定，实际传入：%s`, exprString(arg))
	}
	x, ok := cmp.X.(*ast.Ident)
	if !ok || x.Name != "listType" {
		t.Fatalf(`needTotal 的左值应是 listType，实际：%s`, exprString(cmp.X))
	}
	y, ok := cmp.Y.(*ast.BasicLit)
	if !ok || y.Value != `"page"` {
		t.Fatalf(`needTotal 只应在 "page" 模式下为真，实际右值：%s`, exprString(cmp.Y))
	}
}

// 后台标签管理要分页，必须保留精确总数；但不受前台的 MaxPage 限制。
func TestPluginTagListKeepsTotalAndSkipsMaxPageGate(t *testing.T) {
	src := readFuncSource(t, "../controller/manageController/pluginTag.go", "PluginTagList")
	mustNotContain(t, src, `MaxPage`,
		"后台标签管理不应套用前台的最大页码限制（否则超限页看不到数据）")

	arg := getTagListNeedTotalArg(t, "../controller/manageController/pluginTag.go", "PluginTagList")
	if id, ok := arg.(*ast.Ident); !ok || id.Name != "true" {
		t.Fatalf("后台分页必须传 needTotal=true，否则分页条算不出来，实际：%s", exprString(arg))
	}
}

// getTagListNeedTotalArg 取 funcName 里调用 GetTagList 时传入的第 7 个实参（needTotal）。
//
// 用 ast 取实参而不是正则匹配源码文本：`listType == "page" || true` 这类等价写法
// 正则挡不住，断言就会变成空转（实测漏过一次）。
func getTagListNeedTotalArg(t *testing.T, file, funcName string) ast.Expr {
	t.Helper()

	src, err := os.ReadFile(file)
	if err != nil {
		t.Fatalf("读取 %s 失败：%v", file, err)
	}
	fset := token.NewFileSet()
	f, err := parser.ParseFile(fset, file, src, parser.ParseComments)
	if err != nil {
		t.Fatalf("解析 %s 失败：%v", file, err)
	}

	var fn *ast.FuncDecl
	for _, decl := range f.Decls {
		d, ok := decl.(*ast.FuncDecl)
		if ok && d.Name.Name == funcName {
			fn = d
			break
		}
	}
	if fn == nil {
		t.Fatalf("%s 中找不到函数 %s", file, funcName)
	}

	var arg ast.Expr
	ast.Inspect(fn, func(n ast.Node) bool {
		call, ok := n.(*ast.CallExpr)
		if !ok {
			return true
		}
		sel, ok := call.Fun.(*ast.SelectorExpr)
		if !ok || sel.Sel.Name != "GetTagList" {
			return true
		}
		// itemId, title, categoryIds, firstLetter, offset, pageSize, needTotal, order
		if len(call.Args) != 8 {
			t.Fatalf("%s 中 GetTagList 实参应为 8 个，实际 %d 个", funcName, len(call.Args))
		}
		arg = call.Args[6]
		return false
	})
	if arg == nil {
		t.Fatalf("%s 中没有找到 GetTagList 调用", funcName)
	}
	return arg
}

func exprString(e ast.Expr) string {
	switch v := e.(type) {
	case *ast.Ident:
		return v.Name
	case *ast.BasicLit:
		return v.Value
	case *ast.BinaryExpr:
		return exprString(v.X) + " " + v.Op.String() + " " + exprString(v.Y)
	}
	return "（无法渲染的表达式）"
}

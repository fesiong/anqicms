package provider

import (
	"go/ast"
	"go/parser"
	"go/token"
	"strings"
	"testing"
)

// TestPreviousNextArchiveSingleQuery 护栏：上一篇 / 下一篇必须一条 SQL 直接取整行。
//
// 这两个函数原先都是「ToSQL 拼 SELECT id ... LIMIT 1 → Raw 取 id → 按 id 回表」，
// 详情页每篇要各付一次，等于每页白烧两条往返。正确形状是把过滤条件直接交给
// GetArchiveByFunc：ORDER BY id DESC/ASC + LIMIT 1 由 Take 补，一次取回整行。
//
// 为什么用 AST 而不是假驱动跑：这两个函数的返回值要补缩略图、伪静态链接、
// 地区前缀，依赖一套真实站点配置（System / MultiLanguage / PluginPlace /
// rewrite 快照），假驱动很容易测成「另一个函数」。形状由这里守，端到端效果由
// 服务器抓包核对（详情页 SQL 条数）。
func TestPreviousNextArchiveSingleQuery(t *testing.T) {
	fset := token.NewFileSet()
	file, err := parser.ParseFile(fset, "archive.go", nil, 0)
	if err != nil {
		t.Fatalf("解析 archive.go 失败：%v", err)
	}

	cases := []struct {
		name  string
		order string
	}{
		{name: "GetPreviousArchive", order: "DESC"},
		{name: "GetNextArchive", order: "ASC"},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			fn := findMethodDecl(file, tc.name)
			if fn == nil {
				t.Fatalf("archive.go 里找不到 %s", tc.name)
			}

			var (
				lookupCalls int
				orderFound  string
			)
			ast.Inspect(fn.Body, func(n ast.Node) bool {
				call, ok := n.(*ast.CallExpr)
				if !ok {
					return true
				}
				sel, ok := call.Fun.(*ast.SelectorExpr)
				if !ok {
					return true
				}
				switch sel.Sel.Name {
				case "ToSQL", "Raw":
					t.Errorf("%s 里出现了 %s：不要再走「先取 id 再回表」那套", tc.name, sel.Sel.Name)
				case "GetArchiveByFunc":
					lookupCalls++
				case "Order":
					// 只认条件闭包里那一次 Order，用来确认取的是相邻而不是任意一行。
					if len(call.Args) > 0 {
						if lit, ok := call.Args[0].(*ast.BasicLit); ok {
							orderFound = lit.Value
						}
					}
				}
				return true
			})

			if lookupCalls != 1 {
				t.Errorf("%s 调用了 %d 次 GetArchiveByFunc，期望 1 次", tc.name, lookupCalls)
			}
			if !strings.Contains(strings.ToUpper(orderFound), tc.order) {
				t.Errorf("%s 的排序条件应为 %s，实际 %q —— 去掉排序会取到任意一行", tc.name, tc.order, orderFound)
			}
			// 旧实现的探针变量，出现就说明两步查询回潮了。
			ast.Inspect(fn.Body, func(n ast.Node) bool {
				if id, ok := n.(*ast.Ident); ok && id.Name == "archiveId" {
					t.Errorf("%s 里重新出现了 archiveId 探针", tc.name)
				}
				return true
			})
		})
	}
}

// findMethodDecl 在已解析文件里找指定名字的函数声明。
func findMethodDecl(file *ast.File, name string) *ast.FuncDecl {
	for _, decl := range file.Decls {
		fn, ok := decl.(*ast.FuncDecl)
		if ok && fn.Name.Name == name && fn.Body != nil {
			return fn
		}
	}
	return nil
}

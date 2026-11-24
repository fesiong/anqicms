package provider

import (
	"bytes"
	"fmt"
	"go/ast"
	"go/parser"
	"go/printer"
	"go/token"
	"strings"
	"testing"
)

// 上一篇 / 下一篇的调用链护栏（SQL 形状由 archive_prevnext_sql_test.go 用 DryRun 守）。
//
// 历史坑（2026-11-23 线上事故）：`WHERE category_id = ? AND id < ? ORDER BY id DESC LIMIT 1`
// 在 MySQL 5.7 上退化成扫完整个分类区间（17 万行 / 0.4~25 秒），把磁盘打满导致实例反复复位。
//
// 第一版修复改成「先 MAX(id) 再按主键回表」两步，但依赖的 (category_id, id) 索引
// 只在 model tag 里登记、库里并不存在，实测退化成扫 299,960 行 / 235ms。
//
// 现在的形状：一条 SQL 的标量子查询
//
//	WHERE id = (SELECT MAX(`id`) FROM archives WHERE category_id = ? AND id < ?)
//
// 本文件守调用链（谁调谁、方向实参），防止有人改回两步或改回 ORDER BY。
// 为什么用 AST 而不是假驱动跑：这两个函数的返回值要补缩略图、伪静态链接、
// 地区前缀，依赖一套真实站点配置，假驱动很容易测成「另一个函数」。
type prevNextCase struct {
	name    string // 函数名
	prevArg string // getNeighborArchive 第三个实参的字面量
}

var prevNextCases = []prevNextCase{
	{name: "GetPreviousArchive", prevArg: "true"},
	{name: "GetNextArchive", prevArg: "false"},
}

// checkPrevNext 返回形状问题清单，空切片代表通过。
func checkPrevNext(file *ast.File) []string {
	var problems []string

	for _, tc := range prevNextCases {
		fn := findMethodDecl(file, tc.name)
		if fn == nil {
			problems = append(problems, fmt.Sprintf("%s 找不到", tc.name))
			continue
		}

		var (
			neighborCalls int
			orderCalls    int
			rawCalls      int
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
				rawCalls++
			case "Order":
				orderCalls++
			case "getNeighborArchive":
				neighborCalls++
				if len(call.Args) != 3 {
					problems = append(problems, fmt.Sprintf(
						"%s 调 getNeighborArchive 给了 %d 个实参，期望 3 个（categoryId, id, 方向）",
						tc.name, len(call.Args)))
					return true
				}
				// 方向实参要能静态读出来：true/false 在 Go AST 里是 Ident，
				// 字符串字面量才是 BasicLit，两种都收。
				var dir string
				switch arg := call.Args[2].(type) {
				case *ast.Ident:
					dir = arg.Name
				case *ast.BasicLit:
					dir = strings.Trim(arg.Value, "`\"")
				default:
					problems = append(problems, fmt.Sprintf(
						"%s 调 getNeighborArchive 的方向实参必须是字面量，当前是表达式", tc.name))
					return true
				}
				if dir != "" && dir != tc.prevArg {
					problems = append(problems, fmt.Sprintf(
						"%s 的相邻方向实参是 %s，期望 %s", tc.name, dir, tc.prevArg))
				}
			}
			return true
		})

		if neighborCalls != 1 {
			problems = append(problems, fmt.Sprintf(
				"%s 调了 %d 次 getNeighborArchive，期望恰好 1 次（一步查完，不要拆成定位 + 回表两次往返）",
				tc.name, neighborCalls))
		}
		if orderCalls != 0 {
			problems = append(problems, fmt.Sprintf(
				"%s 里出现了 %d 次 Order：MySQL 5.7 无反向索引扫描，ORDER BY id DESC/ASC "+
					"会把整个 category 区间读完再过滤，必须走 MAX(id)/MIN(id) 子查询定位",
				tc.name, orderCalls))
		}
		if rawCalls != 0 {
			problems = append(problems, fmt.Sprintf(
				"%s 里出现了 %d 次 Raw/ToSQL：不要绕开参数化链式查询", tc.name, rawCalls))
		}
	}

	// 中间层：getNeighborArchive 必须用 neighborArchiveScope 造查询，并用 takeArchive 取行。
	mid := findMethodDecl(file, "getNeighborArchive")
	if mid == nil {
		problems = append(problems, "getNeighborArchive 找不到：上一篇/下一篇必须共用一条查询构造")
		return problems
	}
	if n := countCalls(mid, "neighborArchiveScope"); n != 1 {
		problems = append(problems, fmt.Sprintf(
			"getNeighborArchive 调了 %d 次 neighborArchiveScope，期望 1 次", n))
	}
	if n := countCalls(mid, "takeArchive"); n != 1 {
		problems = append(problems, fmt.Sprintf(
			"getNeighborArchive 调了 %d 次 takeArchive，期望 1 次（一条 SQL 取整行）", n))
	}

	// 最底层：聚合必须真的是裸 MAX/MIN。
	scope := findMethodDecl(file, "neighborArchiveScope")
	if scope == nil {
		problems = append(problems, "neighborArchiveScope 找不到：相邻文章必须走 MAX/MIN 索引定位")
		return problems
	}
	var buf bytes.Buffer
	if err := printer.Fprint(&buf, token.NewFileSet(), scope); err != nil {
		problems = append(problems, fmt.Sprintf("打印 neighborArchiveScope 源码失败：%v", err))
		return problems
	}
	src := buf.String()
	if !strings.Contains(src, "MAX(`id`)") {
		problems = append(problems, "neighborArchiveScope 里没有 MAX(`id`)：上一篇的定位方式不对")
	}
	if !strings.Contains(src, "MIN(`id`)") {
		problems = append(problems, "neighborArchiveScope 里没有 MIN(`id`)：下一篇的定位方式不对")
	}
	return problems
}

// countCalls 统计函数体内调用指定函数/方法的次数。
// neighborArchiveScope 是包级函数（Ident），takeArchive 是方法（SelectorExpr），两种都收。
func countCalls(fn *ast.FuncDecl, name string) int {
	n := 0
	ast.Inspect(fn.Body, func(node ast.Node) bool {
		call, ok := node.(*ast.CallExpr)
		if !ok {
			return true
		}
		switch fun := call.Fun.(type) {
		case *ast.SelectorExpr:
			if fun.Sel.Name == name {
				n++
			}
		case *ast.Ident:
			if fun.Name == name {
				n++
			}
		}
		return true
	})
	return n
}

func parseArchiveGo(t *testing.T, src, name string) *ast.File {
	t.Helper()
	fset := token.NewFileSet()
	file, err := parser.ParseFile(fset, name, src, 0)
	if err != nil {
		t.Fatalf("解析 %s 失败：%v", name, err)
	}
	return file
}

// TestPrevNextArchiveUsesMaxMinIndexLookup 护栏：上一篇 / 下一篇必须一步走 MAX/MIN 索引定位。
func TestPrevNextArchiveUsesMaxMinIndexLookup(t *testing.T) {
	fset := token.NewFileSet()
	file, err := parser.ParseFile(fset, "archive.go", nil, 0)
	if err != nil {
		t.Fatalf("解析 archive.go 失败：%v", err)
	}

	if problems := checkPrevNext(file); len(problems) > 0 {
		for _, p := range problems {
			t.Errorf("上一篇/下一篇形状不合格：%s", p)
		}
	}
}

// TestPrevNextGuardCatchesOldImplementation 反向对照：把旧实现（ORDER BY id DESC + LIMIT 1）
// 喂给同一个检查器，必须被抓出来 —— 否则上面的护栏就是空转的。
func TestPrevNextGuardCatchesOldImplementation(t *testing.T) {
	oldImpl := `
package provider

import "gorm.io/gorm"

func (w *Website) GetPreviousArchive(categoryId, id int64) (*model.Archive, error) {
	archive, err := w.GetArchiveByFunc(func(tx *gorm.DB) *gorm.DB {
		if categoryId > 0 {
			tx = tx.Where("` + "`category_id`" + ` = ?", categoryId)
		}
		return tx.Where("` + "`id`" + ` < ?", id).Order("` + "`id`" + ` DESC")
	})
	return archive, err
}

func (w *Website) GetNextArchive(categoryId, id int64) (*model.Archive, error) {
	archive, err := w.GetArchiveByFunc(func(tx *gorm.DB) *gorm.DB {
		if categoryId > 0 {
			tx = tx.Where("` + "`category_id`" + ` = ?", categoryId)
		}
		return tx.Where("` + "`id`" + ` > ?", id).Order("` + "`id`" + ` ASC")
	})
	return archive, err
}
`
	problems := checkPrevNext(parseArchiveGo(t, oldImpl, "old_impl.go"))
	if len(problems) == 0 {
		t.Fatal("反向对照失败：旧实现（ORDER BY id DESC/ASC）没有被护栏抓到，护栏形同虚设")
	}
	t.Logf("反向对照如期报错 %d 条：", len(problems))
	for _, p := range problems {
		t.Logf("  - %s", p)
	}
}

// TestPrevNextGuardCatchesTwoStepImplementation 反向对照之二：
// 「先 MAX(id) 取 id、再按主键回表」的两步实现也必须被抓到（多一次往返，
// 而且当时真正的问题是索引不在、聚合退化成了整区间扫描）。
func TestPrevNextGuardCatchesTwoStepImplementation(t *testing.T) {
	twoStepImpl := `
package provider

import "gorm.io/gorm"

func (w *Website) GetPreviousArchive(categoryId, id int64) (*model.Archive, error) {
	prevId, err := w.findNeighborArchiveId(categoryId, id, true)
	if err != nil {
		return nil, err
	}
	archive, err := w.GetArchiveByFunc(func(tx *gorm.DB) *gorm.DB {
		return tx.Where("` + "`id`" + ` = ?", prevId)
	})
	return archive, err
}

func (w *Website) GetNextArchive(categoryId, id int64) (*model.Archive, error) {
	nextId, err := w.findNeighborArchiveId(categoryId, id, false)
	if err != nil {
		return nil, err
	}
	archive, err := w.GetArchiveByFunc(func(tx *gorm.DB) *gorm.DB {
		return tx.Where("` + "`id`" + ` = ?", nextId)
	})
	return archive, err
}
`
	problems := checkPrevNext(parseArchiveGo(t, twoStepImpl, "two_step_impl.go"))
	if len(problems) == 0 {
		t.Fatal("反向对照失败：两步实现没有被护栏抓到")
	}
	t.Logf("反向对照如期报错 %d 条：", len(problems))
	for _, p := range problems {
		t.Logf("  - %s", p)
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

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

// 上一篇 / 下一篇的取法护栏。
//
// 历史坑（2026-11-23 线上事故）：这两个函数原先是
//
//	WHERE category_id = ? AND id < ? ORDER BY id DESC LIMIT 1
//
// 一条 SQL 直接取整行。形状看着对，性能上是灾难：MySQL 5.7 没有反向索引扫描，
// 这条 SQL 会退化成「把整个 category 区间扫完再过滤」。线上 archives 21 万行、
// category 6 占 18.6 万行，慢日志实测一条扫 17 万行、0.4~25 秒（Rows_sent=1）。
// 详情页每次访问打两次（上一篇 + 下一篇），几十个并发就能把 2 核 / 1.6G 的机器
// 磁盘打满（100MB/s 读、load 30、systemd-logind 建会话超时），实例被反复硬复位。
//
// 现在的形状：先用 MAX(id) / MIN(id) 走 MySQL 的 MIN/MAX 优化
// （EXPLAIN = "Select tables optimized away"，只读 1 行、0.1ms），再按主键回表。
//
// 为什么用 AST 而不是假驱动跑：这两个函数的返回值要补缩略图、伪静态链接、
// 地区前缀，依赖一套真实站点配置，假驱动很容易测成「另一个函数」。
// 形状由这里守，端到端效果由服务器 EXPLAIN 核对。
type prevNextCase struct {
	name    string // 函数名
	prevArg string // findNeighborArchiveId 第三个实参的字面量
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
			neighborCalls  int
			getArchiveCall int
			orderCalls     int
			rawCalls       int
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
			case "GetArchiveByFunc":
				getArchiveCall++
			case "findNeighborArchiveId":
				neighborCalls++
				if len(call.Args) != 3 {
					problems = append(problems, fmt.Sprintf(
						"%s 调 findNeighborArchiveId 只给了 %d 个实参，期望 3 个（categoryId, id, 方向）",
						tc.name, len(call.Args)))
					break
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
						"%s 调 findNeighborArchiveId 的方向实参必须是字面量，当前是表达式", tc.name))
					break
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
				"%s 调了 %d 次 findNeighborArchiveId，期望恰好 1 次（上一篇/下一篇各定位一次）",
				tc.name, neighborCalls))
		}
		if getArchiveCall != 1 {
			problems = append(problems, fmt.Sprintf(
				"%s 调了 %d 次 GetArchiveByFunc，期望 1 次（按主键回表取整行）",
				tc.name, getArchiveCall))
		}
		if orderCalls != 0 {
			problems = append(problems, fmt.Sprintf(
				"%s 里出现了 %d 次 Order：MySQL 5.7 无反向索引扫描，ORDER BY id DESC/ASC "+
					"会把整个 category 区间读完再过滤，必须换成 MAX(id)/MIN(id) 索引定位",
				tc.name, orderCalls))
		}
		if rawCalls != 0 {
			problems = append(problems, fmt.Sprintf(
				"%s 里出现了 %d 次 Raw/ToSQL：不要绕开参数化链式查询", tc.name, rawCalls))
		}
	}

	// 定位辅助函数必须真的用了 MIN/MAX 聚合，否则「不写 Order」也可能是别的坏形状。
	helper := findMethodDecl(file, "findNeighborArchiveId")
	if helper == nil {
		problems = append(problems, "findNeighborArchiveId 找不到：相邻文章必须走 MAX/MIN 索引定位")
		return problems
	}
	var buf bytes.Buffer
	if err := printer.Fprint(&buf, token.NewFileSet(), helper); err != nil {
		problems = append(problems, fmt.Sprintf("打印 findNeighborArchiveId 源码失败：%v", err))
		return problems
	}
	src := buf.String()
	if !strings.Contains(src, "MAX(`id`)") {
		problems = append(problems, "findNeighborArchiveId 里没有 MAX(`id`)：上一篇的定位方式不对")
	}
	if !strings.Contains(src, "MIN(`id`)") {
		problems = append(problems, "findNeighborArchiveId 里没有 MIN(`id`)：下一篇的定位方式不对")
	}
	return problems
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

// TestPrevNextArchiveUsesMaxMinIndexLookup 护栏：上一篇 / 下一篇必须先走 MAX/MIN 索引定位。
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

// TestPrevNextGuardCatchesMissingAggregate 反向对照之二：定位函数在、但里面没用 MIN/MAX 聚合，
// 也必须被抓出来（否则「去掉 Order」可能只是换了个别的坏形状）。
func TestPrevNextGuardCatchesMissingAggregate(t *testing.T) {
	badImpl := `
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

func (w *Website) findNeighborArchiveId(categoryId, id int64, prev bool) (int64, error) {
	return 0, nil
}
`
	problems := checkPrevNext(parseArchiveGo(t, badImpl, "bad_impl.go"))
	if len(problems) == 0 {
		t.Fatal("反向对照失败：没有 MIN/MAX 聚合的实现没有被护栏抓到")
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

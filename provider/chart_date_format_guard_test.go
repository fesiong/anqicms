package provider

import (
	"go/ast"
	"go/parser"
	"go/token"
	"io/fs"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"testing"
)

// chartDateForbidden 是图表日期列的写法禁忌。
//
// 2026-11-06 实测：DSN 带 parseTime=True（provider/dao.go），MySQL 的 DATE() 结果会被
// 驱动解析成 time.Time，GORM 再赋给 response.ChartData.Date（string）就成了
// "2026-11-05T00:00:00+08:00"，dashboard 订单统计的 x 轴直接显示这一长串。
// 同一份数据在 Go 侧分支里是 time.Unix(..).Format("2006-01-02")，两种格式混在同一个
// 接口里，前端按 label 合并系列时也对不上。
//
// 约定：按天分组必须走 ChartDateSQL（DATE_FORMAT 出字符串），不要退回裸 DATE()。
const chartDateForbidden = "DATE(FROM_UNIXTIME("

// TestChartDateSQLContract 钉住助手产出的格式。
func TestChartDateSQLContract(t *testing.T) {
	got := ChartDateSQL("created_time")
	want := "DATE_FORMAT(FROM_UNIXTIME(`created_time`), '%Y-%m-%d') AS `date`"
	if got != want {
		t.Errorf("ChartDateSQL 产出变了\n got: %s\nwant: %s", got, want)
	}
	if strings.Contains(got, chartDateForbidden) {
		t.Errorf("ChartDateSQL 自身不能含 %s", chartDateForbidden)
	}
}

// TestChartDateColumnsUseHelper 扫描图表数据所在的源码目录，
// 断言没有任何一处 SQL 用裸 DATE(FROM_UNIXTIME(..)) 产出日期列。
func TestChartDateColumnsUseHelper(t *testing.T) {
	violations := scanChartDateViolations(t, ".", ".."+string(filepath.Separator)+"controller")
	if len(violations) > 0 {
		for _, v := range violations {
			t.Errorf("图表日期列不得使用裸 DATE(FROM_UNIXTIME(..))，请改用 ChartDateSQL：%s", v)
		}
	}
}

// TestChartDateGuardIsNotVacuous 反向对照：把坏写法写进临时文件，
// 扫描器必须报出来。否则上面那条断言可能只是因为扫不到文件而"通过"。
func TestChartDateGuardIsNotVacuous(t *testing.T) {
	dir := t.TempDir()
	bad := filepath.Join(dir, "badchart.go")
	src := "package badchart\n\n" +
		"func q() string {\n" +
		"\treturn \"DATE(FROM_UNIXTIME(created_time)) AS `date`\"\n" +
		"}\n"
	if err := os.WriteFile(bad, []byte(src), 0o644); err != nil {
		t.Fatalf("准备反向对照文件失败：%v", err)
	}
	got := scanChartDateViolations(t, dir)
	if len(got) != 1 {
		t.Fatalf("反向对照不成立：坏写法没有被扫出（得到 %d 条），说明扫描逻辑失效", len(got))
	}
	if !strings.Contains(got[0], "badchart.go") {
		t.Fatalf("反向对照报错了位置：%s", got[0])
	}

	// 同时确认注释里的这句说明不会被误判成 SQL 违规。
	okFile := filepath.Join(dir, "commentonly.go")
	okSrc := "package commentonly\n\n" +
		"// 不要写成 DATE(FROM_UNIXTIME(x))，要用 ChartDateSQL\n" +
		"func k() int { return 1 }\n"
	if err := os.WriteFile(okFile, []byte(okSrc), 0o644); err != nil {
		t.Fatalf("准备注释对照文件失败：%v", err)
	}
	if got := scanChartDateViolations(t, dir); len(got) != 1 {
		t.Errorf("注释里的说明不应计入违规，实际：%v", got)
	}
}

// scanChartDateViolations 遍历 dirs 下的非测试 .go 文件，
// 只检查字符串字面量（注释不计），返回违规位置描述。
func scanChartDateViolations(t *testing.T, dirs ...string) []string {
	t.Helper()
	var violations []string
	fset := token.NewFileSet()

	for _, dir := range dirs {
		err := filepath.WalkDir(dir, func(path string, d fs.DirEntry, err error) error {
			if err != nil {
				// 目录不存在/无权限时跳过，扫描器不应因为环境问题报错
				return nil
			}
			if d.IsDir() {
				name := d.Name()
				if path != dir && (name == "testdata" || name == "node_modules") {
					return filepath.SkipDir
				}
				return nil
			}
			if !strings.HasSuffix(path, ".go") || strings.HasSuffix(path, "_test.go") {
				return nil
			}
			parsed, perr := parser.ParseFile(fset, path, nil, 0)
			if perr != nil {
				t.Errorf("解析 %s 失败：%v", path, perr)
				return nil
			}
			ast.Inspect(parsed, func(n ast.Node) bool {
				lit, ok := n.(*ast.BasicLit)
				if !ok || lit.Kind != token.STRING {
					return true
				}
				value, uerr := unquoteAny(lit.Value)
				if uerr != nil {
					return true
				}
				if strings.Contains(value, chartDateForbidden) {
					violations = append(violations,
						path+":"+strconv.Itoa(fset.Position(lit.Pos()).Line))
				}
				return true
			})
			return nil
		})
		if err != nil {
			t.Errorf("扫描 %s 失败：%v", dir, err)
		}
	}
	return violations
}

func unquoteAny(raw string) (string, error) {
	if strings.HasPrefix(raw, "`") {
		return strings.TrimSuffix(strings.TrimPrefix(raw, "`"), "`"), nil
	}
	return strconv.Unquote(raw)
}

package provider

import (
	"context"
	"strings"
	"testing"

	"gorm.io/driver/mysql"
	"gorm.io/gorm"
	"kandaoni.com/anqicms/model"
)

// 上一篇 / 下一篇的 SQL 形状护栏。
//
// 历史（2026-11-23 线上事故）：原先是
//
//	WHERE category_id = ? AND id < ? ORDER BY id DESC LIMIT 1
//
// MySQL 5.7 没有反向索引扫描，这条会退化成「把整个 category 区间扫完再过滤」，
// 21 万行的库实测扫 17 万行 / 0.4~25 秒，详情页每访问一次打两条，磁盘被打满后
// 实例被反复硬复位。
//
// 后来改成「先 MAX(id) 再按主键回表」两步，方向对但踩了第二个坑：MAX(id) 的
// 索引定位优化依赖 (category_id, id) 复合索引**真实存在**，而它当时只写在 model
// tag 里、库里没有。实测退化成 covering skip scan 扫 299,960 行 / 235ms。
//
// 现在的形状是一条 SQL 的子查询：
//
//	WHERE id = (SELECT MAX(`id`) FROM archives WHERE category_id = ? AND id < ?)
//
// 标量子查询不引用外层列，MySQL 先算一次（8.0.45 实测 "Rows fetched before
// execution"、0.2ms），再按主键点查。索引存在时 5.7 / 8.0 都是 O(1)，
// 且详情页只发 2 条 SQL（上一篇、下一篇各一条）。
//
// 反向对照见 TestNeighborArchiveScopeCatchesDegradedShapes。
func newDryRunArchiveDB(t *testing.T) *gorm.DB {
	t.Helper()
	db, err := gorm.Open(mysql.New(mysql.Config{
		DriverName:                "mysql",
		DSN:                       "root:@tcp(127.0.0.1:3306)/anqidev",
		SkipInitializeWithVersion: true,
	}), &gorm.Config{DryRun: true, DisableAutomaticPing: true})
	if err != nil {
		t.Fatalf("打开 DryRun 连接失败：%v", err)
	}
	return db
}

// buildNeighborSQL 渲染 neighborArchiveScope 真正会发出的 SQL（不连库）。
func buildNeighborSQL(t *testing.T, db *gorm.DB, categoryId, id int64, prev bool) string {
	t.Helper()
	var archive model.Archive
	tx := neighborArchiveScope(db.WithContext(context.Background()), categoryId, id, prev)
	stmt := tx.Session(&gorm.Session{DryRun: true}).Take(&archive).Statement
	return stmt.SQL.String()
}

// checkNeighborSQL 返回 SQL 形状问题清单，空切片代表通过。
func checkNeighborSQL(sqlText string, prev bool) []string {
	var problems []string
	upper := strings.ToUpper(sqlText)
	agg := "MIN"
	if prev {
		agg = "MAX"
	}

	// 1) 定位必须靠裸聚合子查询：`id` = (SELECT MAX(`id`) ...)
	if !strings.Contains(sqlText, "`id` = (SELECT "+agg+"(`id`)") {
		problems = append(problems, "定位不是 `id` = (SELECT "+agg+"(`id`) ...) 子查询形态")
	}
	// 2) 子查询必须带分类等值条件（否则会跨分类取相邻，语义错）
	if !strings.Contains(sqlText, "`category_id`") {
		problems = append(problems, "子查询丢了 category_id 等值条件")
	}
	// 3) id 方向条件必须保留
	if prev && !strings.Contains(sqlText, "`id` <") {
		problems = append(problems, "上一篇少了 `id` < 条件")
	}
	if !prev && !strings.Contains(sqlText, "`id` >") {
		problems = append(problems, "下一篇少了 `id` > 条件")
	}
	// 4) 不能有 ORDER BY：5.7 无反向索引扫描，ORDER BY id DESC/ASC + LIMIT 1
	//    会退化成扫完整个分类区间（线上崩库的形状）。
	if strings.Contains(upper, "ORDER BY") {
		problems = append(problems, "SQL 带了 ORDER BY，会退化成整区间扫描")
	}
	// 5) 外层只允许一个 SELECT *（取整行），子查询必须是聚合，不能也 SELECT *
	if n := strings.Count(upper, "SELECT *"); n != 1 {
		problems = append(problems, "SELECT * 出现次数应为 1（外层取整行、子查询取聚合）")
	}
	// 6) 不能回退成「先取 id 再回表」的实现产物：那会多一条独立 SQL 且没有子查询。
	if strings.Count(upper, "SELECT") != 2 {
		problems = append(problems, "SQL 里 SELECT 出现次数应为 2（外层 + 子查询）")
	}
	return problems
}

func TestNeighborArchiveSQLShape(t *testing.T) {
	db := newDryRunArchiveDB(t)
	cases := []struct {
		name string
		prev bool
	}{
		{name: "上一篇", prev: true},
		{name: "下一篇", prev: false},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			sqlText := buildNeighborSQL(t, db, 13, 300017, tc.prev)
			t.Logf("生成的 SQL：%s", sqlText)
			if problems := checkNeighborSQL(sqlText, tc.prev); len(problems) > 0 {
				for _, p := range problems {
					t.Errorf("SQL 形状不合格：%s", p)
				}
			}
		})
	}
}

// TestNeighborArchiveSQLShapeWithoutCategory 不限分类时（categoryId=0）
// 子查询只剩主键条件，仍然必须是 MAX/MIN 子查询，不能退化成 ORDER BY。
func TestNeighborArchiveSQLShapeWithoutCategory(t *testing.T) {
	db := newDryRunArchiveDB(t)
	sqlText := buildNeighborSQL(t, db, 0, 300017, true)
	t.Logf("生成的 SQL：%s", sqlText)
	if !strings.Contains(sqlText, "`id` = (SELECT MAX(`id`)") {
		t.Errorf("不限分类时定位方式不对：%s", sqlText)
	}
	if strings.Contains(strings.ToUpper(sqlText), "ORDER BY") {
		t.Errorf("不限分类时仍在用 ORDER BY：%s", sqlText)
	}
	if strings.Contains(sqlText, "`category_id`") {
		t.Errorf("categoryId=0 时不应带分类条件：%s", sqlText)
	}
}

// TestNeighborArchiveScopeCatchesDegradedShapes 反向对照：
// 两种历史坏形状喂给同一个检查器都必须被抓到，否则护栏是空转的。
func TestNeighborArchiveScopeCatchesDegradedShapes(t *testing.T) {
	cases := []struct {
		name    string
		sqlText string
		prev    bool
	}{
		{
			name:    "事故形状 ORDER BY id DESC LIMIT 1",
			sqlText: "SELECT * FROM `archives` WHERE `category_id` = ? AND `id` < ? ORDER BY `id` DESC LIMIT 1",
			prev:    true,
		},
		{
			name:    "两步实现里的聚合 SQL（没有子查询定位）",
			sqlText: "SELECT MAX(`id`) AS aid FROM `archives` WHERE `category_id` = ? AND `id` < ?",
			prev:    true,
		},
		{
			name:    "子查询取整行（丢了聚合）",
			sqlText: "SELECT * FROM `archives` WHERE `id` = (SELECT * FROM `archives` WHERE `category_id` = ? AND `id` < ?) LIMIT 1",
			prev:    true,
		},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			if problems := checkNeighborSQL(tc.sqlText, tc.prev); len(problems) == 0 {
				t.Fatalf("反向对照失败：坏形状没被抓到 —— %s", tc.sqlText)
			} else {
				t.Logf("如期抓到 %d 条问题", len(problems))
				for _, p := range problems {
					t.Logf("  - %s", p)
				}
			}
		})
	}
}

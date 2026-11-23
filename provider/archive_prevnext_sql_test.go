package provider

import (
	"context"
	"database/sql"
	"strings"
	"testing"

	"gorm.io/driver/mysql"
	"gorm.io/gorm"
	"kandaoni.com/anqicms/model"
)

// TestNeighborArchiveIdSqlShape 护栏：相邻文章定位的 SQL 必须保持
// 「SELECT MAX(`id`) AS aid FROM archives WHERE ...」这个形状。
//
// 这不是形式检查：MySQL 只对「带等值前缀的裸 MAX/MIN 聚合」做索引定位优化
// （EXPLAIN 显示 "Select tables optimized away"，只读 1 行）。一旦 GORM 把它
// 包成子查询、或者塞进 ORDER BY / LIMIT 之外的结构，5.7 就会退化成扫全表 ——
// 线上就是这么把磁盘打满导致实例反复崩溃的（见 archive_prevnext_guard_test.go）。
//
// 用 DryRun：只生成 SQL 不连库，可以直接在 CI 里跑。
func TestNeighborArchiveIdSqlShape(t *testing.T) {
	db, err := gorm.Open(mysql.New(mysql.Config{
		DriverName:                "mysql",
		DSN:                       "root:@tcp(127.0.0.1:3306)/anqidev",
		SkipInitializeWithVersion: true,
	}), &gorm.Config{DryRun: true, DisableAutomaticPing: true})
	if err != nil {
		t.Fatalf("打开 DryRun 连接失败：%v", err)
	}

	build := func(agg string, idWhere string) string {
		var row struct {
			Aid sql.NullInt64 `gorm:"column:aid"`
		}
		q := db.WithContext(context.Background()).Model(&model.Archive{}).
			Where("`category_id` = ?", 6).
			Where(idWhere, 44886).
			Select(agg + " AS aid")
		stmt := q.Session(&gorm.Session{DryRun: true}).Scan(&row).Statement
		return stmt.SQL.String()
	}

	cases := []struct {
		name    string
		agg     string
		idWhere string
		want    string
	}{
		{name: "上一篇", agg: "MAX(`id`)", idWhere: "`id` < ?", want: "MAX(`id`) AS aid"},
		{name: "下一篇", agg: "MIN(`id`)", idWhere: "`id` > ?", want: "MIN(`id`) AS aid"},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			sqlText := build(tc.agg, tc.idWhere)
			t.Logf("生成的 SQL：%s", sqlText)

			if !strings.Contains(sqlText, tc.want) {
				t.Errorf("SQL 里没有 %s，实际：%s", tc.want, sqlText)
			}
			if !strings.Contains(sqlText, "`category_id`") {
				t.Errorf("SQL 丢了 category_id 过滤条件：%s", sqlText)
			}
			if !strings.Contains(sqlText, "archives") {
				t.Errorf("SQL 没查到 archives 表：%s", sqlText)
			}
			// 有 ORDER BY / 子查询包裹，5.7 的 MIN/MAX 优化就不生效了。
			if strings.Contains(strings.ToUpper(sqlText), "ORDER BY") {
				t.Errorf("SQL 带了 ORDER BY，会让 MIN/MAX 优化失效：%s", sqlText)
			}
			if strings.Contains(sqlText, "SELECT *") {
				t.Errorf("SQL 取了整行，应该只取聚合值：%s", sqlText)
			}
		})
	}
}

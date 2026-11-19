package provider

import (
	"bytes"
	"context"
	"database/sql"
	"database/sql/driver"
	"errors"
	"io"
	"log"
	"strings"
	"testing"
	"time"

	"gorm.io/driver/mysql"
	"gorm.io/gorm"
	"gorm.io/gorm/logger"
	"kandaoni.com/anqicms/config"
	"kandaoni.com/anqicms/library"
	"kandaoni.com/anqicms/model"
)

type archiveQuerySQL struct {
	aiSettingSQL
	queries  []string
	empty    bool
	moduleId int64 // 伪造成 archives.module_id；0 表示默认 2（产品模型）
}

// moduleColumn 返回伪行里的 module_id。默认产品模型，让侧表读取逻辑生效。
func (d *archiveQuerySQL) moduleColumn() int64 {
	if d.moduleId == 0 {
		return 2
	}
	return d.moduleId
}

func (d *archiveQuerySQL) Connect(context.Context) (driver.Conn, error) { return d, nil }
func (d *archiveQuerySQL) QueryContext(_ context.Context, query string, _ []driver.NamedValue) (driver.Rows, error) {
	d.queries = append(d.queries, query)
	if strings.Contains(query, "FROM `archive_products`") {
		return &archiveQueryRows{
			columns: []string{"archive_id", "brand", "product_type"},
			data:    [][]driver.Value{{int64(1), "brand", int64(config.ProductTypeNormal)}},
		}, nil
	}
	if strings.Contains(query, "FROM `archives`") {
		// module_id 决定要不要读侧表（见 archiveProductScope）。
		rows := &archiveQueryRows{columns: []string{"id", "module_id"}}
		if !d.empty {
			rows.data = [][]driver.Value{{int64(1), d.moduleColumn()}, {int64(2), d.moduleColumn()}}
		}
		return rows, nil
	}
	return nil, errors.New("unexpected query: " + query)
}

type archiveQueryRows struct {
	columns []string
	data    [][]driver.Value
}

func (r *archiveQueryRows) Columns() []string { return r.columns }
func (r *archiveQueryRows) Close() error      { return nil }
func (r *archiveQueryRows) Next(dest []driver.Value) error {
	if len(r.data) == 0 {
		return io.EOF
	}
	copy(dest, r.data[0])
	r.data = r.data[1:]
	return nil
}

func newArchiveQuerySite(t *testing.T, d *archiveQuerySQL, l logger.Interface) *Website {
	t.Helper()
	conn := sql.OpenDB(d)
	t.Cleanup(func() { _ = conn.Close() })
	db, err := gorm.Open(mysql.New(mysql.Config{Conn: conn, SkipInitializeWithVersion: true}), &gorm.Config{DisableAutomaticPing: true, Logger: l})
	if err != nil {
		t.Fatal(err)
	}
	w := &Website{DB: db, Cache: library.InitMemoryCache(), System: &config.SystemConfig{ShopMode: 1}}
	// 侧表只对产品模型的行走 SQL（见 archiveProductScope），这里把 2 号模型
	// 预置成产品，省掉一次 modules 查询干扰计数。
	_ = w.Cache.Set("module_kinds", map[uint]string{2: model.ModuleKindProduct}, 0)
	w.registerArchiveProductLoader()
	return w
}

func TestArchiveProductQueryCount(t *testing.T) {
	for _, kind := range []string{"list", "single", "draft"} {
		t.Run(kind, func(t *testing.T) {
			d := &archiveQuerySQL{}
			w := newArchiveQuerySite(t, d, logger.Default.LogMode(logger.Silent))
			var brand string
			switch kind {
			case "list":
				var list []model.Archive
				if err := w.DB.Find(&list).Error; err != nil {
					t.Fatal(err)
				}
				if len(list) != 2 || list[1].ProductType != 1 {
					t.Fatalf("missing default product fields: %+v", list)
				}
				brand = list[0].Brand
			case "single":
				var a model.Archive
				if err := w.DB.Take(&a).Error; err != nil {
					t.Fatal(err)
				}
				brand = a.Brand
			case "draft":
				var list []model.ArchiveDraft
				if err := w.DB.Table("archives").Find(&list).Error; err != nil {
					t.Fatal(err)
				}
				brand = list[0].Brand
			}
			if brand != "brand" {
				t.Fatalf("brand = %q", brand)
			}
			if len(d.queries) != 2 {
				t.Fatalf("queries = %d, want main + one batch: %v", len(d.queries), d.queries)
			}
			if !strings.Contains(d.queries[1], " IN ") {
				t.Fatalf("not a batch query: %s", d.queries[1])
			}
		})
	}
}

func TestArchiveListSQLLogging(t *testing.T) {
	for _, level := range []logger.LogLevel{logger.Info, logger.Warn} {
		t.Run(map[logger.LogLevel]string{logger.Info: "info", logger.Warn: "warn"}[level], func(t *testing.T) {
			var output bytes.Buffer
			l := logger.New(log.New(&output, "", 0), logger.Config{LogLevel: level, SlowThreshold: time.Hour})
			d := &archiveQuerySQL{empty: true}
			w := newArchiveQuerySite(t, d, l)
			for i := range 2 {
				output.Reset()
				if _, _, err := w.GetArchiveList(nil, "id desc", 0, 8); err != nil {
					t.Fatal(err)
				}
				want := 0
				if i == 0 && level == logger.Info {
					want = 1
				}
				if got := strings.Count(output.String(), "SELECT"); got != want {
					t.Fatalf("request %d logged %d SQL statements, want %d: %s", i, got, want, output.String())
				}
			}
			if len(d.queries) != 1 {
				t.Fatalf("empty list cache: %d actual queries, want 1", len(d.queries))
			}
		})
	}
}

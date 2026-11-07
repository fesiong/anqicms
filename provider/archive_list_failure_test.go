package provider

import (
	"context"
	"database/sql"
	"database/sql/driver"
	"errors"
	"fmt"
	"io"
	"strings"
	"sync"
	"testing"
	"time"

	"gorm.io/driver/mysql"
	"gorm.io/gorm"
	"gorm.io/gorm/callbacks"
	"kandaoni.com/anqicms/config"
	"kandaoni.com/anqicms/library"
	"kandaoni.com/anqicms/model"
)

type randomListSQL struct {
	stage    string
	err      error
	queries  []string
	estimate string
	respond  func(string) (driver.Rows, error)
}

func (d *randomListSQL) Connect(context.Context) (driver.Conn, error) { return d, nil }
func (d *randomListSQL) Driver() driver.Driver                        { return d }
func (d *randomListSQL) Open(string) (driver.Conn, error)             { return d, nil }
func (d *randomListSQL) Prepare(string) (driver.Stmt, error) {
	return nil, errors.New("unexpected prepare")
}
func (d *randomListSQL) Close() error              { return nil }
func (d *randomListSQL) Begin() (driver.Tx, error) { return nil, errors.New("unexpected transaction") }
func (d *randomListSQL) QueryContext(_ context.Context, query string, _ []driver.NamedValue) (driver.Rows, error) {
	d.queries = append(d.queries, query)
	if d.respond != nil {
		return d.respond(query)
	}
	if strings.HasPrefix(query, "EXPLAIN") {
		if d.stage == "explain" {
			return nil, d.err
		}
		if d.stage == "empty explain" {
			return &randomListRows{columns: []string{"rows"}}, nil
		}
		estimate := d.estimate
		if estimate == "" {
			estimate = "100001"
		}
		return &randomListRows{columns: []string{"rows"}, data: [][]driver.Value{{estimate}}}, nil
	}
	if strings.Contains(query, "MIN(archives.id)") {
		if d.stage == "bounds" {
			return nil, d.err
		}
		return &randomListRows{columns: []string{"min_id", "max_id"}, data: [][]driver.Value{{int64(42), int64(42)}}}, nil
	}
	if strings.Contains(query, "archives.id >=") {
		if d.stage == "sample" {
			return nil, d.err
		}
		return &randomListRows{columns: []string{"id"}, data: [][]driver.Value{{int64(42)}}}, nil
	}
	return &randomListRows{columns: []string{"id"}}, nil
}

type randomListRows struct {
	columns []string
	data    [][]driver.Value
}

func (r *randomListRows) Columns() []string { return r.columns }
func (r *randomListRows) Close() error      { return nil }
func (r *randomListRows) Next(dest []driver.Value) error {
	if len(r.data) == 0 {
		return io.EOF
	}
	copy(dest, r.data[0])
	r.data = r.data[1:]
	return nil
}

func TestExplainCountSkippedQuery(t *testing.T) {
	conn := sql.OpenDB(&randomListSQL{})
	t.Cleanup(func() { _ = conn.Close() })
	db, err := gorm.Open(mysql.New(mysql.Config{Conn: conn, SkipInitializeWithVersion: true}), &gorm.Config{DisableAutomaticPing: true})
	if err != nil {
		t.Fatal(err)
	}
	w := &Website{DB: db}
	count, err := w.getExplainCount("SELECT id FROM archives WHERE title = 'system'")
	if err != nil || count != 0 {
		t.Fatalf("skipped estimate = %d, %v; want existing count fallback", count, err)
	}
}

func TestExplainCountValues(t *testing.T) {
	for _, tt := range []struct {
		name    string
		input   string
		want    int64
		wantErr bool
	}{
		{"zero", "0", 0, false}, {"large", "100001", 100001, false}, {"partition suffix", "3 (0%)", 3, false}, {"negative", "-1", 0, true},
	} {
		t.Run(tt.name, func(t *testing.T) {
			conn := sql.OpenDB(&randomListSQL{estimate: tt.input})
			t.Cleanup(func() { _ = conn.Close() })
			db, err := gorm.Open(mysql.New(mysql.Config{Conn: conn, SkipInitializeWithVersion: true}), &gorm.Config{DisableAutomaticPing: true})
			if err != nil {
				t.Fatal(err)
			}
			w := &Website{DB: db}
			got, err := w.getExplainCount("SELECT id FROM archives")
			if got != tt.want || (err != nil) != tt.wantErr {
				t.Fatalf("got %d, %v; want %d, error=%v", got, err, tt.want, tt.wantErr)
			}
		})
	}
}

func TestArchiveListEstimateFailureStopsQueries(t *testing.T) {
	for _, stage := range []string{"explain", "empty explain", "invalid estimate"} {
		for _, page := range []int{0, 1} {
			t.Run(fmt.Sprintf("%s/page%d", stage, page), func(t *testing.T) {
				wantErr := errors.New("explain failed")
				fake := &randomListSQL{stage: stage, err: wantErr}
				if stage == "invalid estimate" {
					fake.estimate = "unknown"
				}
				sqlDB := sql.OpenDB(fake)
				t.Cleanup(func() { _ = sqlDB.Close() })
				db, err := gorm.Open(mysql.New(mysql.Config{Conn: sqlDB, SkipInitializeWithVersion: true}), &gorm.Config{DisableAutomaticPing: true})
				if err != nil {
					t.Fatal(err)
				}
				w := &Website{DB: db, Cache: library.InitMemoryCache()}
				for range 2 {
					_, _, err := w.GetArchiveList(nil, "rand()", page, 2)
					if err == nil {
						t.Fatal("estimate failure reported as successful list")
					}
					if stage == "explain" && !errors.Is(err, wantErr) {
						t.Fatalf("lost explain error: %v", err)
					}
				}
				for _, query := range fake.queries {
					if !strings.HasPrefix(query, "EXPLAIN") {
						t.Fatalf("estimate failure triggered data query: %s", query)
					}
				}
			})
		}
	}
}

func TestArchiveListRandomFailure(t *testing.T) {
	for _, stage := range []string{"bounds", "sample", "single candidate"} {
		t.Run(stage, func(t *testing.T) {
			wantErr := errors.New("random list unavailable")
			fake := &randomListSQL{stage: stage, err: wantErr}
			sqlDB := sql.OpenDB(fake)
			t.Cleanup(func() { _ = sqlDB.Close() })
			db, err := gorm.Open(mysql.New(mysql.Config{Conn: sqlDB, SkipInitializeWithVersion: true}), &gorm.Config{DisableAutomaticPing: true})
			if err != nil {
				t.Fatal(err)
			}
			w := &Website{DB: db, Cache: library.InitMemoryCache()}
			for range 2 {
				_, _, err := w.GetArchiveList(nil, "rand()", 0, 2)
				if stage == "single candidate" {
					if err != nil {
						t.Fatal(err)
					}
					last := fake.queries[len(fake.queries)-1]
					if !strings.Contains(last, "archives.id IN") {
						t.Fatalf("single candidate missing from final selection: %s", last)
					}
					break
				}
				if !errors.Is(err, wantErr) {
					t.Fatalf("got %v, want query error", err)
				}
			}
		})
	}
}

func TestArchiveListPagedFailureRecovery(t *testing.T) {
	for _, stage := range []string{"count", "ids"} {
		t.Run(stage, func(t *testing.T) {
			conn := sql.OpenDB(&randomListSQL{estimate: "0"})
			db, err := gorm.Open(mysql.New(mysql.Config{Conn: conn, SkipInitializeWithVersion: true}), &gorm.Config{DisableAutomaticPing: true})
			if err != nil {
				t.Fatal(err)
			}
			sqlDB, err := db.DB()
			if err != nil {
				t.Fatal(err)
			}
			t.Cleanup(func() { _ = sqlDB.Close() })
			w := &Website{DB: db, Cache: library.InitMemoryCache()}
			sql := db.ToSQL(func(tx *gorm.DB) *gorm.DB { return tx.Order("id desc").Find(&[]*model.Archive{}) })
			key := "archive-list-count" + library.Md5(sql)[8:24]
			if stage == "ids" {
				if err := w.Cache.Set(key, int64(7), 300); err != nil {
					t.Fatal(err)
				}
			}
			wantErr := errors.New("paged query failed")
			fail := true
			idQueries := 0
			if err := db.Callback().Query().Replace("gorm:query", func(tx *gorm.DB) {
				callbacks.BuildQuerySQL(tx)
				if tx.DryRun {
					return
				}
				if count, ok := tx.Statement.Dest.(*int64); ok {
					if fail && stage == "count" {
						tx.AddError(wantErr)
					} else {
						*count = 7
						tx.RowsAffected = 1
					}
					return
				}
				idQueries++
				if fail && stage == "ids" {
					tx.AddError(wantErr)
				}
			}); err != nil {
				t.Fatal(err)
			}
			_, _, err = w.GetArchiveList(nil, "id desc", 1, 10)
			if !errors.Is(err, wantErr) {
				t.Errorf("got %v, want paged query error", err)
			}
			if stage == "count" {
				var count int64
				if err := w.Cache.Get(key, &count); err == nil {
					t.Errorf("failed count cached: %d", count)
				}
			}
			fail = false
			before := idQueries
			got, total, err := w.GetArchiveList(nil, "id desc", 1, 10)
			if idQueries != before+1 {
				t.Fatalf("recovery did not query IDs: before=%d, after=%d", before, idQueries)
			}
			if err != nil || len(got) != 0 || total != 7 {
				t.Fatalf("recovery = %v, %d, %v", got, total, err)
			}
		})
	}
}

func TestArchiveListNonemptyRecovery(t *testing.T) {
	for _, tt := range []struct {
		name  string
		page  int
		draft int
	}{
		{"list", 0, 0}, {"paged", 2, 0}, {"draft list", 0, 1}, {"draft paged", 2, 1},
	} {
		t.Run(tt.name, func(t *testing.T) {
			wantErr := errors.New("detail temporarily unavailable")
			fail := true
			counts, ids, details := 0, 0, 0
			fake := &randomListSQL{}
			fake.respond = func(query string) (driver.Rows, error) {
				if strings.Contains(query, "FROM `archive_products`") {
					return &randomListRows{columns: []string{"archive_id"}}, nil
				}
				if strings.HasPrefix(query, "EXPLAIN") {
					return &randomListRows{columns: []string{"rows"}, data: [][]driver.Value{{"7"}}}, nil
				}
				if strings.Contains(query, "count(*)") {
					counts++
					return &randomListRows{columns: []string{"count(*)"}, data: [][]driver.Value{{int64(7)}}}, nil
				}
				if tt.draft == 1 && !strings.Contains(query, "archive_drafts") {
					t.Errorf("draft data read from wrong table: %s", query)
				}
				if strings.Contains(query, " IN (") {
					details++
					if fail {
						return nil, wantErr
					}
					return &randomListRows{
						columns: []string{"id", "title", "fixed_link"},
						data:    [][]driver.Value{{int64(42), "newer", "/article-42"}, {int64(41), "older", "/article-41"}},
					}, nil
				}
				ids++
				if !strings.Contains(query, "ORDER BY id desc") || !strings.Contains(query, "LIMIT 2") || (tt.page > 0 && !strings.Contains(query, "OFFSET 2")) {
					t.Errorf("selection lost ordering or pagination: %s", query)
				}
				return &randomListRows{columns: []string{"id"}, data: [][]driver.Value{{int64(42)}, {int64(41)}}}, nil
			}
			conn := sql.OpenDB(fake)
			t.Cleanup(func() { _ = conn.Close() })
			db, err := gorm.Open(mysql.New(mysql.Config{Conn: conn, SkipInitializeWithVersion: true}), &gorm.Config{DisableAutomaticPing: true})
			if err != nil {
				t.Fatal(err)
			}
			w := &Website{
				DB: db, Cache: library.InitMemoryCache(),
				System:        &config.SystemConfig{BaseUrl: "https://example.test"},
				Content:       &config.ContentConfig{DefaultThumbs: []string{"/default.jpg"}},
				PluginStorage: &config.PluginStorageConfig{StorageUrl: "https://example.test"},
				MultiLanguage: &config.PluginMultiLangConfig{},
				parsedPattern: &RewritePattern{Parsed: true, Patterns: RewriteMode{}},
			}
			if got, _, err := w.GetArchiveList(nil, "id desc", tt.page, 2, 0, tt.draft); !errors.Is(err, wantErr) || len(got) != 0 {
				t.Fatalf("failure = %v, %v", got, err)
			}
			fail = false
			for attempt := range 2 {
				got, total, err := w.GetArchiveList(nil, "id desc", tt.page, 2, 0, tt.draft)
				if err != nil || len(got) != 2 {
					t.Fatalf("recovery = %v, %v", got, err)
				}
				for i, want := range []struct {
					id    int64
					title string
				}{{42, "newer"}, {41, "older"}} {
					if got[i].Id != want.id || got[i].Title != want.title || got[i].Link != fmt.Sprintf("https://example.test/article-%d", want.id) || got[i].Thumb != "https://example.test/thumb_default.jpg" {
						t.Errorf("article %d lost data or enrichment: %+v", i, got[i])
					}
				}
				wantTotal := int64(0)
				if tt.page > 0 {
					wantTotal = 7
				} else if attempt == 1 && tt.draft == 0 {
					wantTotal = 2
				}
				if total != wantTotal {
					t.Errorf("total = %d, want %d", total, wantTotal)
				}
			}
			wantReads := 3
			if tt.page == 0 && tt.draft == 0 {
				wantReads = 2
			}
			if ids != wantReads || details != wantReads {
				t.Errorf("ID/detail reads = %d/%d, want %d each", ids, details, wantReads)
			}
			wantCounts := 0
			if tt.page > 0 {
				wantCounts = 1
			}
			if counts != wantCounts {
				t.Errorf("count reads = %d, want %d", counts, wantCounts)
			}
		})
	}
}

func TestArchiveListDetailFailure(t *testing.T) {
	for _, tt := range []struct {
		name  string
		page  int
		draft int
	}{
		{"list", 0, 0}, {"paged", 1, 0}, {"draft list", 0, 1}, {"draft paged", 1, 1},
	} {
		t.Run(tt.name, func(t *testing.T) {
			db, err := gorm.Open(mysql.New(mysql.Config{DSN: "unused:unused@tcp(127.0.0.1:1)/unused", SkipInitializeWithVersion: true}), &gorm.Config{DisableAutomaticPing: true})
			if err != nil {
				t.Fatal(err)
			}
			sqlDB, err := db.DB()
			if err != nil {
				t.Fatal(err)
			}
			t.Cleanup(func() { _ = sqlDB.Close() })
			w := &Website{DB: db, Cache: library.InitMemoryCache()}
			countSQL := db.ToSQL(func(tx *gorm.DB) *gorm.DB { return tx.Order("id desc").Find(&[]*model.Archive{}) })
			if err := w.Cache.Set("archive-list-count"+library.Md5(countSQL)[8:24], int64(1), 300); err != nil {
				t.Fatal(err)
			}
			wantErr := errors.New("detail query failed")
			details := 0
			if err := db.Callback().Query().Replace("gorm:query", func(tx *gorm.DB) {
				callbacks.BuildQuerySQL(tx)
				if tx.DryRun {
					return
				}
				if ids, ok := tx.Statement.Dest.(*[]int64); ok {
					*ids = []int64{42}
					return
				}
				details++
				tx.AddError(wantErr)
			}); err != nil {
				t.Fatal(err)
			}
			for range 2 {
				got, _, err := w.GetArchiveList(nil, "id desc", tt.page, 10, 0, tt.draft)
				if !errors.Is(err, wantErr) {
					t.Errorf("got %v, want detail error", err)
				}
				if len(got) != 0 {
					t.Errorf("returned incomplete data: %v", got)
				}
			}
			if details != 2 {
				t.Errorf("detail attempts = %d, want 2 (failure must not be cached)", details)
			}
		})
	}
}

func TestArchiveListQueryFailureDoesNotCache(t *testing.T) {
	db, err := gorm.Open(mysql.New(mysql.Config{DSN: "unused:unused@tcp(127.0.0.1:1)/unused", SkipInitializeWithVersion: true}), &gorm.Config{DisableAutomaticPing: true})
	if err != nil {
		t.Fatal(err)
	}
	sqlDB, err := db.DB()
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = sqlDB.Close() })
	wantErr := errors.New("list query unavailable")
	fail := true
	calls := 0
	w := &Website{DB: db, Cache: library.InitMemoryCache()}
	sql := db.ToSQL(func(tx *gorm.DB) *gorm.DB { return tx.Order("id desc").Limit(10).Find(&[]*model.Archive{}) })
	key := "archive-list-" + library.Md5(sql)[8:24]
	var pending *sync.WaitGroup
	if err := db.Callback().Query().Replace("gorm:query", func(tx *gorm.DB) {
		callbacks.BuildQuerySQL(tx)
		if tx.DryRun {
			return
		}
		calls++
		if fail {
			pending, _ = w.Cache.Pending(key)
			tx.AddError(wantErr)
		}
	}); err != nil {
		t.Fatal(err)
	}
	_, _, err = w.GetArchiveList(nil, "id desc", 0, 10)
	if !errors.Is(err, wantErr) {
		t.Errorf("got %v, want query error", err)
	}
	if _, ok := w.Cache.Pending(key); ok {
		t.Fatal("failed query retained pending marker")
	}
	if pending == nil {
		t.Fatal("query did not register pending marker")
	}
	done := make(chan struct{})
	go func() { pending.Wait(); close(done) }()
	select {
	case <-done:
	case <-time.After(3 * time.Second):
		pending.Done()
		<-done
		t.Fatal("query failure did not wake cache waiters")
	}
	var cached []*model.Archive
	if err := w.Cache.Get(key, &cached); err == nil {
		t.Error("failed query cached an empty list")
	}
	fail = false
	before := calls
	_, _, err = w.GetArchiveList(nil, "id desc", 0, 10)
	if err != nil {
		t.Fatal(err)
	}
	if calls == before {
		t.Fatal("retry read failed query result from cache")
	}
}

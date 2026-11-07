package provider

import (
	"errors"
	"reflect"
	"strings"
	"sync"

	"gorm.io/gorm/callbacks"
	"sync/atomic"
	"testing"
	"time"

	"gorm.io/driver/mysql"
	"gorm.io/gorm"
	"kandaoni.com/anqicms/config"
)

func TestTimeReleaseRandomBoundsQuery(t *testing.T) {
	for _, name := range []string{"empty", "single id", "multiple ids", "query failure"} {
		t.Run(name, func(t *testing.T) {
			db, err := gorm.Open(mysql.New(mysql.Config{DSN: "unused:unused@tcp(127.0.0.1:1)/unused", SkipInitializeWithVersion: true}), &gorm.Config{DisableAutomaticPing: true})
			if err != nil {
				t.Fatal(err)
			}
			sqlDB, err := db.DB()
			if err != nil {
				t.Fatal(err)
			}
			t.Cleanup(func() { _ = sqlDB.Close() })
			var queries []string
			var values [][]interface{}
			err = db.Callback().Query().Replace("gorm:query", func(tx *gorm.DB) {
				callbacks.BuildQuerySQL(tx)
				queries = append(queries, tx.Statement.SQL.String())
				values = append(values, append([]interface{}(nil), tx.Statement.Vars...))
				if name == "query failure" {
					tx.AddError(errors.New("bounds unavailable"))
					return
				}
				if (name == "single id" || name == "multiple ids") && strings.Contains(tx.Statement.SQL.String(), "max(id)") {
					v := reflect.ValueOf(tx.Statement.Dest).Elem()
					v.FieldByName("MaxId").SetInt(42)
					if name == "multiple ids" {
						v.FieldByName("MaxId").SetInt(50)
					}
					if min := v.FieldByName("MinId"); min.IsValid() {
						min.SetInt(42)
					}
				}
			})
			if err != nil {
				t.Fatal(err)
			}
			w := &Website{DB: db}
			w.TimeReleaseArchives(&config.PluginTimeFactor{ReleaseOpen: true, Random: true, DailyLimit: 100, ModuleIds: []int64{1, 2}, CategoryIds: []int64{3}})
			want := 1
			if name == "single id" || name == "multiple ids" {
				want = 2
			}
			if len(queries) != want {
				t.Fatalf("queries = %v, want %d round trips", queries, want)
			}
			for _, fragment := range []string{"max(id) as max_id", "min(id) as min_id", "`status` = 0", "module_id IN", "category_id NOT IN"} {
				if !strings.Contains(queries[0], fragment) {
					t.Errorf("bounds query missing %q: %s", fragment, queries[0])
				}
			}
			if !reflect.DeepEqual(values[0][:3], []interface{}{int64(1), int64(2), int64(3)}) {
				t.Fatalf("changed bounds filters: %v", values[0])
			}
			if want == 2 {
				for _, fragment := range []string{"SELECT *", "`status` = 0", "module_id IN", "category_id NOT IN", "id >= ?", "ORDER BY id asc"} {
					if !strings.Contains(queries[1], fragment) {
						t.Errorf("draft query missing %q: %s", fragment, queries[1])
					}
				}
				if strings.Contains(queries[1], "max(id)") || strings.Contains(queries[1], "min(id)") {
					t.Fatalf("aggregate leaked to draft query: %s", queries[1])
				}
				if !reflect.DeepEqual(values[1][:3], values[0][:3]) {
					t.Fatalf("changed draft filters: %v", values[1])
				}
				lower := values[1][3].(int64)
				if lower < 42 || (name == "single id" && lower != 42) || (name == "multiple ids" && lower >= 50) {
					t.Fatalf("random bound out of range: %d", lower)
				}
			}
		})
	}
}

func TestArchiveReleaseDoesNotOverlap(t *testing.T) {
	for _, kind := range []string{"planned", "timefactor"} {
		t.Run(kind, func(t *testing.T) {
			db, err := gorm.Open(mysql.New(mysql.Config{
				DSN:                       "unused:unused@tcp(127.0.0.1:1)/unused",
				SkipInitializeWithVersion: true,
			}), &gorm.Config{DisableAutomaticPing: true})
			if err != nil {
				t.Fatal(err)
			}
			sqlDB, err := db.DB()
			if err != nil {
				t.Fatal(err)
			}
			t.Cleanup(func() { _ = sqlDB.Close() })

			entered := make(chan struct{})
			release := make(chan struct{})
			var releaseOnce sync.Once
			unblock := func() { releaseOnce.Do(func() { close(release) }) }
			t.Cleanup(unblock)
			wait := func(ch <-chan struct{}) {
				t.Helper()
				select {
				case <-ch:
				case <-time.After(3 * time.Second):
					t.Fatal("timed out waiting for release task")
				}
			}
			var calls atomic.Int32
			err = db.Callback().Query().Replace("gorm:query", func(tx *gorm.DB) {
				if calls.Add(1) == 1 {
					close(entered)
					<-release
				}
			})
			if err != nil {
				t.Fatal(err)
			}
			w := &Website{DB: db}
			run := w.PublishPlanArchives
			if kind == "timefactor" {
				w.PluginTimeFactor = &config.PluginTimeFactor{
					ReleaseOpen: true,
					Random:      true,
					DailyLimit:  100,
					ModuleIds:   []int64{1},
				}
				run = w.TryToRunTimeFactor
			}
			firstDone := make(chan struct{})
			go func() { defer close(firstDone); run() }()
			wait(entered)
			secondDone := make(chan struct{})
			go func() { defer close(secondDone); run() }()
			select {
			case <-secondDone:
			case <-time.After(3 * time.Second):
				unblock()
				wait(firstDone)
				t.Fatal("overlapping tick must skip rather than wait")
			}
			overlapped := calls.Load() != 1
			unblock()
			wait(firstDone)
			if overlapped {
				t.Fatal("overlapping tick executed another database query")
			}
			before := calls.Load()
			run()
			if calls.Load() <= before {
				t.Fatal("next tick did not resume after the previous run finished")
			}
		})
	}
}

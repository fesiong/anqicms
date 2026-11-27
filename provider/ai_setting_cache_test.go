package provider

import (
	"context"
	"database/sql"
	"database/sql/driver"
	"errors"
	"io"
	"reflect"
	"runtime"
	"strings"
	"testing"
	"time"

	"gorm.io/driver/mysql"
	"gorm.io/gorm"
	"gorm.io/gorm/logger"
	"kandaoni.com/anqicms/library"
	"kandaoni.com/anqicms/pkg/ai/eino"
)

// Keep GORM and the real memory cache; replace only the database connection.
type aiSettingSQL struct {
	value       string
	row         bool
	queryErr    error
	saveErr     error
	queries     int
	writes      int
	readStarted chan struct{}
	releaseRead chan struct{}
}

func (d *aiSettingSQL) Connect(context.Context) (driver.Conn, error) { return d, nil }
func (d *aiSettingSQL) Driver() driver.Driver                        { return d }
func (d *aiSettingSQL) Open(string) (driver.Conn, error)             { return d, nil }
func (d *aiSettingSQL) Close() error                                 { return nil }
func (d *aiSettingSQL) Prepare(string) (driver.Stmt, error) {
	return nil, errors.New("unexpected prepare")
}
func (d *aiSettingSQL) Begin() (driver.Tx, error) {
	return nil, errors.New("unexpected transaction")
}
func (d *aiSettingSQL) QueryContext(_ context.Context, query string, args []driver.NamedValue) (driver.Rows, error) {
	if !strings.HasPrefix(query, "SELECT `value` FROM `settings`") || len(args) != 1 || args[0].Value != AiSettingKey {
		return nil, errors.New("unexpected settings query: " + query)
	}
	d.queries++
	if d.queryErr != nil {
		return nil, d.queryErr
	}
	rows := &aiSettingRows{value: d.value, row: d.row}
	if d.readStarted != nil && d.queries == 1 {
		close(d.readStarted)
		<-d.releaseRead
	}
	return rows, nil
}
func (d *aiSettingSQL) ExecContext(_ context.Context, query string, args []driver.NamedValue) (driver.Result, error) {
	if !strings.HasPrefix(query, "UPDATE `settings` SET `value`=? WHERE `key` = ?") || len(args) != 2 {
		return nil, errors.New("unexpected settings write: " + query)
	}
	d.writes++
	if d.saveErr != nil {
		return nil, d.saveErr
	}
	if args[1].Value == AiSettingKey {
		d.value = args[0].Value.(string)
		d.row = true
	}
	return driver.RowsAffected(1), nil
}

type aiSettingRows struct {
	value string
	row   bool
}

func (r *aiSettingRows) Columns() []string { return []string{"value"} }
func (r *aiSettingRows) Close() error      { return nil }
func (r *aiSettingRows) Next(dest []driver.Value) error {
	if !r.row {
		return io.EOF
	}
	dest[0] = r.value
	r.row = false
	return nil
}

func newAiSettingCacheSite(t *testing.T, d *aiSettingSQL) *Website {
	t.Helper()
	conn := sql.OpenDB(d)
	t.Cleanup(func() { _ = conn.Close() })
	db, err := gorm.Open(mysql.New(mysql.Config{Conn: conn, SkipInitializeWithVersion: true}), &gorm.Config{
		DisableAutomaticPing:   true,
		SkipDefaultTransaction: true,
		Logger:                 logger.Default.LogMode(logger.Silent),
	})
	if err != nil {
		t.Fatal(err)
	}
	return &Website{Id: 1, DB: db, Cache: library.InitMemoryCache()}
}

func TestAiSettingCacheEmpty(t *testing.T) {
	for _, row := range []bool{false, true} {
		t.Run(map[bool]string{false: "missing row", true: "empty value"}[row], func(t *testing.T) {
			d := &aiSettingSQL{row: row}
			w := newAiSettingCacheSite(t, d)
			before := eino.GlobalConfig()
			for range 2 {
				if got := w.LoadAiSetting(""); !reflect.DeepEqual(*got, eino.Configs{}) {
					t.Fatalf("empty settings = %+v; want zero value", got)
				}
			}
			if d.queries != 1 {
				t.Fatalf("empty settings queried %d times; want 1", d.queries)
			}
			var cached eino.Configs
			if err := w.Cache.Get("ai_setting", &cached); err != nil || !reflect.DeepEqual(cached, eino.Configs{}) {
				t.Fatalf("negative cache = %+v, %v; want valid zero value", cached, err)
			}
			if eino.GlobalConfig() != before {
				t.Fatal("empty settings initialized the AI client")
			}
		})
	}
}

func TestAiSettingCacheRetries(t *testing.T) {
	for _, scenario := range []string{"query error", "invalid JSON"} {
		t.Run(scenario, func(t *testing.T) {
			d := &aiSettingSQL{row: true, value: "invalid"}
			if scenario == "query error" {
				d.queryErr = errors.New("database unavailable")
			}
			w := newAiSettingCacheSite(t, d)
			for range 2 {
				w.LoadAiSetting("")
				var cached eino.Configs
				if err := w.Cache.Get("ai_setting", &cached); err == nil {
					t.Fatal("failed load was cached")
				}
			}
			if d.queries != 2 {
				t.Fatalf("failed loads queried %d times; want 2", d.queries)
			}
			// Recovery must be observable, not hidden behind a cached failure.
			d.queryErr, d.value = nil, ""
			w.LoadAiSetting("")
			w.LoadAiSetting("")
			if d.queries != 3 {
				t.Fatalf("recovery queried %d times; want 3 total", d.queries)
			}
		})
	}
}

func TestAiSettingCacheSave(t *testing.T) {
	for _, raw := range []bool{false, true} {
		for _, scenario := range []string{"success", "failure", "other key", "marshal failure", "nil DB", "nil cache"} {
			if raw && scenario == "marshal failure" {
				continue
			}
			t.Run(map[bool]string{false: "JSON", true: "raw"}[raw]+"/"+scenario, func(t *testing.T) {
				d := &aiSettingSQL{}
				w := newAiSettingCacheSite(t, d)
				// Seed a previously loaded negative cache; writes must invalidate it.
				if err := w.Cache.Set("ai_setting", eino.Configs{}, 86400); err != nil {
					t.Fatal(err)
				}
				if err := w.Cache.Set("unrelated", "keep", 86400); err != nil {
					t.Fatal(err)
				}
				key := AiSettingKey
				var value interface{} = eino.Configs{LastModel: "saved"}
				if raw {
					value = `{"last_model":"saved"}`
				}
				wantErr := scenario == "failure" || scenario == "marshal failure"
				switch scenario {
				case "failure":
					d.saveErr = errors.New("save failed")
				case "other key":
					key = SystemSettingKey
				case "marshal failure":
					value = make(chan int)
				case "nil DB":
					w.DB = nil
				case "nil cache":
					w.Cache = nil
				}
				var err error
				if raw {
					err = w.SaveSettingValueRaw(key, value)
				} else {
					err = w.SaveSettingValue(key, value)
				}
				if (err != nil) != wantErr {
					t.Fatalf("save error = %v; want error %v", err, wantErr)
				}
				if scenario == "nil cache" {
					if got := w.LoadAiSetting(""); got.LastModel != "saved" {
						t.Fatalf("uncached saved settings = %+v", got)
					}
					return
				}
				var unrelated string
				if err := w.Cache.Get("unrelated", &unrelated); err != nil || unrelated != "keep" {
					t.Fatal("save removed unrelated cache")
				}
				got := w.LoadAiSetting("")
				if scenario == "success" {
					if got.LastModel != "saved" || d.queries != 1 || d.writes != 1 {
						t.Fatalf("successful save left stale settings: %+v, queries=%d, writes=%d", got, d.queries, d.writes)
					}
				} else if got.LastModel != "" || d.queries != 0 {
					t.Fatalf("unsuccessful/unrelated save invalidated cache: %+v, queries=%d", got, d.queries)
				}
			})
		}
	}
}

// Catch a save invalidation being overtaken by an older empty database read.
func TestAiSettingCacheConcurrentSave(t *testing.T) {
	for _, raw := range []bool{false, true} {
		t.Run("same instance", func(t *testing.T) {
			testAiSettingCacheConcurrentSave(t, raw, func(w *Website) *Website { return w })
		})
		t.Run("clone", func(t *testing.T) {
			testAiSettingCacheConcurrentSave(t, raw, func(w *Website) *Website { return cloneWithContext(w, nil) })
		})
		t.Run("same ID", func(t *testing.T) {
			testAiSettingCacheConcurrentSave(t, raw, func(w *Website) *Website {
				return &Website{Id: w.Id, DB: w.DB, Cache: w.Cache}
			})
		})
	}
}

func testAiSettingCacheConcurrentSave(t *testing.T, raw bool, clone func(*Website) *Website) {
	t.Helper()
	t.Run(map[bool]string{false: "JSON", true: "raw"}[raw], func(t *testing.T) {
		d := &aiSettingSQL{readStarted: make(chan struct{}), releaseRead: make(chan struct{})}
		w := newAiSettingCacheSite(t, d)
		writer := clone(w)
		loaded := make(chan *eino.Configs, 1)
		go func() { loaded <- w.LoadAiSetting("") }()
		<-d.readStarted // The read has captured an empty result, but cannot fill the cache yet.
		released := false
		defer func() {
			if !released {
				close(d.releaseRead)
				<-loaded
			}
		}()

		saved := make(chan error, 1)
		go func() {
			if raw {
				saved <- writer.SaveSettingValueRaw(AiSettingKey, `{"last_model":"saved"}`)
			} else {
				saved <- writer.SaveSettingValue(AiSettingKey, eino.Configs{LastModel: "saved"})
			}
		}()

		// Wait for an observable scheduling state, not an elapsed sleep:
		// broken code finishes saving; fixed code blocks on the read's mutex.
		// synctest.Wait cannot be used because mutex waits are not durably blocked.
		deadline := time.NewTimer(5 * time.Second)
		defer deadline.Stop()
		saveFinished := false
	waitSave:
		for {
			select {
			case err := <-saved:
				if err != nil {
					t.Fatal(err)
				}
				saveFinished = true
				break waitSave
			case <-deadline.C:
				t.Fatal("save neither completed nor blocked on the AI settings mutex")
			default:
			}
			buf := make([]byte, 1<<20)
			n := runtime.Stack(buf, true)
			for _, stack := range strings.Split(string(buf[:n]), "\n\n") {
				if strings.Contains(stack, "[sync.Mutex.Lock]") &&
					strings.Contains(stack, "(*Website).SaveSettingValue") {
					break waitSave
				}
			}
			runtime.Gosched()
		}
		close(d.releaseRead)
		released = true
		if got := <-loaded; !reflect.DeepEqual(*got, eino.Configs{}) {
			t.Fatalf("initial read = %+v; want captured empty settings", got)
		}
		if !saveFinished {
			if err := <-saved; err != nil {
				t.Fatal(err)
			}
		}
		for range 2 {
			if got := w.LoadAiSetting(""); got.LastModel != "saved" {
				t.Fatalf("save was overwritten by stale empty cache: %+v", got)
			}
		}
	})
}

func TestAiSettingLockSiteIsolation(t *testing.T) {
	w := &Website{Id: 101}
	lock := w.aiSettingLock()
	lock.Lock()
	defer lock.Unlock()

	// Same-ID instances and real request clones must see the held lock.
	for _, site := range []*Website{w, cloneWithContext(w, nil), {Id: 101}} {
		if other := site.aiSettingLock(); other.TryLock() {
			other.Unlock()
			t.Fatal("same site ID did not share the held AI settings lock")
		}
	}
	otherSite := &Website{Id: 102}
	other := otherSite.aiSettingLock()
	if !other.TryLock() {
		t.Fatal("different site IDs share a global AI settings lock")
	}
	other.Unlock()
}

func TestAiSettingCacheNil(t *testing.T) {
	for _, value := range []string{"", `{"last_model":"uncached"}`} {
		t.Run(value, func(t *testing.T) {
			d := &aiSettingSQL{row: true, value: value}
			w := newAiSettingCacheSite(t, d)
			w.Cache = nil
			for range 2 {
				got := w.LoadAiSetting("")
				if value != "" && got.LastModel != "uncached" {
					t.Fatalf("uncached settings = %+v", got)
				}
			}
			if d.queries != 2 {
				t.Fatalf("nil cache queried %d times; want 2", d.queries)
			}
		})
	}
}

package provider

import (
	"context"
	"database/sql"
	"database/sql/driver"
	"errors"
	"go/ast"
	"io"
	"strings"
	"testing"
	"time"

	"gorm.io/driver/mysql"
	"gorm.io/gorm"
	"gorm.io/gorm/logger"
)

func TestVideoTaskIsRegisteredForSchemaMigration(t *testing.T) {
	fn := mustFindFunc(t, "dao.go", "AutoMigrateDB")
	found := false
	ast.Inspect(fn, func(node ast.Node) bool {
		selector, ok := node.(*ast.SelectorExpr)
		if ok && selector.Sel.Name == "VideoTask" {
			found = true
			return false
		}
		return true
	})
	if !found {
		t.Fatal("AutoMigrateDB must register model.VideoTask")
	}
}

func TestVideoTaskSchemaCompleteChecksEveryPersistedColumn(t *testing.T) {
	columns := []string{
		"id", "created_time", "updated_time", "attachment_id", "status", "progress",
		"attempts", "error_msg", "config", "playlist", "artifacts", "next_retry_at",
		"started_at", "finished_at", "lease_until", "lease_owner",
	}
	for _, missing := range columns {
		t.Run(missing, func(t *testing.T) {
			d := &videoSchemaSQL{missingColumn: missing}
			db := newVideoSchemaSQLSite(t, d)
			if videoTaskSchemaComplete(db) {
				t.Fatalf("schema without %s must be incomplete", missing)
			}
			if !d.columnChecked {
				t.Fatalf("schema check did not inspect %s", missing)
			}
		})
	}
}

func TestVideoTaskSchemaCompleteChecksEveryIndex(t *testing.T) {
	for _, missing := range []string{"idx_attachment_status", "idx_status_retry", "idx_created_time"} {
		t.Run(missing, func(t *testing.T) {
			d := &videoSchemaSQL{missingIndex: missing}
			db := newVideoSchemaSQLSite(t, d)
			if videoTaskSchemaComplete(db) {
				t.Fatalf("schema without %s must be incomplete", missing)
			}
			if !d.indexChecked {
				t.Fatalf("schema check did not inspect %s", missing)
			}
		})
	}
}

func TestVideoTaskSchemaCompleteRejectsLegacyTimeColumns(t *testing.T) {
	for _, column := range []string{"created_time", "updated_time"} {
		t.Run(column, func(t *testing.T) {
			d := &videoSchemaSQL{timeColumns: map[string]string{column: "datetime"}}
			db := newVideoSchemaSQLSite(t, d)
			if videoTaskSchemaComplete(db) {
				t.Fatalf("schema with datetime %s must be incomplete", column)
			}
		})
	}
}

func TestEnsureVideoTaskTableConvertsLegacyTimeColumns(t *testing.T) {
	d := &videoSchemaSQL{timeColumnType: "datetime"}
	db := newVideoSchemaSQLSite(t, d)
	if err := ensureVideoTaskTable(db); err != nil {
		t.Fatal(err)
	}
	if len(d.execs) == 0 {
		t.Fatal("legacy datetime columns must be converted before AutoMigrate")
	}
	joined := strings.Join(d.execs, "\n")
	for _, column := range []string{
		"created_time", "updated_time", "next_retry_at", "started_at", "finished_at", "lease_until",
	} {
		if !strings.Contains(joined, "`"+column+"_unix`") || !strings.Contains(joined, "CHANGE COLUMN") {
			t.Fatalf("migration must preserve %s through a Unix-seconds staging column: %s", column, joined)
		}
	}
	if strings.Contains(joined, "UNIX_TIMESTAMP") {
		t.Fatalf("migration must not depend on the MySQL session timezone: %s", joined)
	}
	if !videoTaskSchemaComplete(db) {
		t.Fatalf("migration must leave all task timestamps as bigint: %s", joined)
	}
}

func TestMigrateVideoTaskTimeColumnsUsesGoLocalTime(t *testing.T) {
	legacy := time.Date(2026, time.November, 25, 14, 59, 33, 0, time.Local)
	d := &videoSchemaSQL{
		timeColumnType: "datetime",
		legacyRows: map[string][][]driver.Value{
			"created_time":  {{int64(6), legacy.Add(-time.Hour)}},
			"updated_time":  {{int64(7), legacy.Add(time.Hour)}},
			"next_retry_at": {{int64(1), nil}, {int64(2), time.Time{}}},
			"started_at":    {{int64(3), legacy}},
			"finished_at":   {{int64(4), legacy.Add(29 * time.Second)}},
			"lease_until":   {{int64(5), legacy.Add(2 * time.Minute)}},
		},
	}
	db := newVideoSchemaSQLSite(t, d)
	if err := migrateVideoTaskTimeColumns(db); err != nil {
		t.Fatal(err)
	}
	want := map[string]map[int64]int64{
		"created_time":  {6: legacy.Add(-time.Hour).Unix()},
		"updated_time":  {7: legacy.Add(time.Hour).Unix()},
		"next_retry_at": {1: 0, 2: 0},
		"started_at":    {3: legacy.Unix()},
		"finished_at":   {4: legacy.Add(29 * time.Second).Unix()},
		"lease_until":   {5: legacy.Add(2 * time.Minute).Unix()},
	}
	for column, rows := range want {
		for id, timestamp := range rows {
			found := false
			for _, update := range d.updates {
				if strings.Contains(update.query, "`"+column+"_unix`") && len(update.args) == 2 {
					if update.args[0].Value == timestamp && update.args[1].Value == id {
						found = true
						break
					}
				}
			}
			if !found {
				t.Fatalf("%s id=%d must use Go-local Unix timestamp %d; updates=%+v", column, id, timestamp, d.updates)
			}
		}
	}
}

type videoSchemaUpdate struct {
	query string
	args  []driver.NamedValue
}

type videoSchemaSQL struct {
	missingColumn  string
	missingIndex   string
	timeColumnType string
	columnChecked  bool
	indexChecked   bool
	execs          []string
	updates        []videoSchemaUpdate
	legacyRows     map[string][][]driver.Value
	timeColumns    map[string]string
}

func (d *videoSchemaSQL) Connect(context.Context) (driver.Conn, error) { return d, nil }
func (d *videoSchemaSQL) Driver() driver.Driver                        { return d }
func (d *videoSchemaSQL) Open(string) (driver.Conn, error)             { return d, nil }
func (d *videoSchemaSQL) Close() error                                 { return nil }
func (d *videoSchemaSQL) Prepare(string) (driver.Stmt, error) {
	return nil, errors.New("unexpected prepare")
}
func (d *videoSchemaSQL) Begin() (driver.Tx, error) {
	return nil, errors.New("unexpected transaction")
}
func (d *videoSchemaSQL) QueryContext(_ context.Context, query string, args []driver.NamedValue) (driver.Rows, error) {
	lower := strings.ToLower(query)
	switch {
	case strings.HasPrefix(query, "SELECT DATABASE()"):
		return &videoSchemaRows{columns: []string{"DATABASE()"}, data: [][]driver.Value{{"anqicms"}}}, nil
	case strings.Contains(lower, "information_schema.tables"):
		return &videoSchemaRows{columns: []string{"count"}, data: [][]driver.Value{{int64(1)}}}, nil
	case strings.Contains(lower, "information_schema.columns") && strings.Contains(lower, "data_type"):
		timeType := d.timeColumnType
		if timeType == "" {
			timeType = "bigint"
		}
		columnType := func(column string) driver.Value {
			if value, ok := d.timeColumns[column]; ok {
				return value
			}
			return timeType
		}
		return &videoSchemaRows{
			columns: []string{"column_name", "data_type"},
			data: [][]driver.Value{
				{"created_time", columnType("created_time")},
				{"updated_time", columnType("updated_time")},
				{"next_retry_at", columnType("next_retry_at")},
				{"started_at", columnType("started_at")},
				{"finished_at", columnType("finished_at")},
				{"lease_until", columnType("lease_until")},
			},
		}, nil
	case strings.Contains(lower, "information_schema.columns"):
		if len(args) != 3 {
			return nil, errors.New("unexpected column query args")
		}
		column := args[2].Value.(string)
		d.columnChecked = column == d.missingColumn
		count := int64(1)
		if column == d.missingColumn {
			count = 0
		}
		return &videoSchemaRows{columns: []string{"count"}, data: [][]driver.Value{{count}}}, nil
	case strings.Contains(lower, "information_schema.statistics"):
		if len(args) != 3 {
			return nil, errors.New("unexpected index query args")
		}
		index := args[2].Value.(string)
		d.indexChecked = index == d.missingIndex
		count := int64(1)
		if index == d.missingIndex {
			count = 0
		}
		return &videoSchemaRows{columns: []string{"count"}, data: [][]driver.Value{{count}}}, nil
	case strings.Contains(lower, "from `video_tasks`"):
		for _, column := range videoTaskTimeColumns {
			if strings.Contains(query, "`"+column+"` AS value") {
				return &videoSchemaRows{columns: []string{"id", "value"}, data: d.legacyRows[column]}, nil
			}
		}
		return nil, errors.New("unexpected legacy timestamp query: " + query)
	default:
		return nil, errors.New("unexpected schema query: " + query)
	}
}

func (d *videoSchemaSQL) ExecContext(_ context.Context, query string, args []driver.NamedValue) (driver.Result, error) {
	d.execs = append(d.execs, query)
	if strings.HasPrefix(strings.ToUpper(strings.TrimSpace(query)), "UPDATE `VIDEO_TASKS`") {
		d.updates = append(d.updates, videoSchemaUpdate{query: query, args: append([]driver.NamedValue{}, args...)})
	}
	upper := strings.ToUpper(query)
	if strings.Contains(upper, "ALTER TABLE") && strings.Contains(upper, "CHANGE COLUMN") {
		if d.timeColumns == nil {
			d.timeColumns = make(map[string]string)
		}
		for _, column := range []string{
			"created_time", "updated_time", "next_retry_at", "started_at", "finished_at", "lease_until",
		} {
			if strings.Contains(query, "`"+column+"_unix`") {
				d.timeColumns[column] = "bigint"
			}
		}
	}
	return driver.RowsAffected(1), nil
}

type videoSchemaRows struct {
	columns []string
	data    [][]driver.Value
}

func (r *videoSchemaRows) Columns() []string { return r.columns }
func (r *videoSchemaRows) Close() error      { return nil }
func (r *videoSchemaRows) Next(dest []driver.Value) error {
	if len(r.data) == 0 {
		return io.EOF
	}
	copy(dest, r.data[0])
	r.data = r.data[1:]
	return nil
}

func newVideoSchemaSQLSite(t *testing.T, d *videoSchemaSQL) *gorm.DB {
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
	return db
}

func TestSameVersionMigrationEnsuresVideoTaskTable(t *testing.T) {
	fn := mustFindFunc(t, "siteMigration.go", "MigrateSiteSchema")
	var branch ast.Stmt
	ast.Inspect(fn, func(node ast.Node) bool {
		statement, ok := node.(*ast.IfStmt)
		if !ok {
			return true
		}
		unary, ok := statement.Cond.(*ast.UnaryExpr)
		if !ok || unary.Op.String() != "!" {
			return true
		}
		identifier, ok := unary.X.(*ast.Ident)
		if ok && identifier.Name == "pending" {
			branch = statement.Body
			return false
		}
		return true
	})
	if branch == nil {
		t.Fatal("MigrateSiteSchema must keep its same-version branch")
	}
	callPos, returnPos := -1, -1
	ast.Inspect(branch, func(node ast.Node) bool {
		if _, ok := node.(*ast.ReturnStmt); ok && returnPos < 0 {
			returnPos = int(node.Pos())
			return true
		}
		call, ok := node.(*ast.CallExpr)
		if !ok {
			return true
		}
		identifier, ok := call.Fun.(*ast.Ident)
		if ok && identifier.Name == "ensureVideoTaskTable" {
			callPos = int(call.Pos())
			return false
		}
		return true
	})
	if callPos < 0 || returnPos < 0 || callPos > returnPos {
		t.Fatalf("same-version sites must ensure video_tasks before returning: call=%d return=%d", callPos, returnPos)
	}
	helper := mustFindFunc(t, "siteMigration.go", "ensureVideoTaskTable")
	slotFound, migrateFound := false, false
	ast.Inspect(helper, func(node ast.Node) bool {
		call, ok := node.(*ast.CallExpr)
		if !ok {
			return true
		}
		identifier, ok := call.Fun.(*ast.Ident)
		if ok && identifier.Name == "migrateSlot" {
			slotFound = true
		}
		selector, ok := call.Fun.(*ast.SelectorExpr)
		if ok && selector.Sel.Name == "AutoMigrate" {
			migrateFound = true
		}
		return true
	})
	if !slotFound || !migrateFound {
		t.Fatalf("video_tasks backfill must use the migration gate and AutoMigrate: slot=%v migrate=%v", slotFound, migrateFound)
	}
}

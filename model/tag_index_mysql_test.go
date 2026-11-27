package model

import (
	"context"
	"database/sql"
	"fmt"
	"os"
	"reflect"
	"strings"
	"sync"
	"testing"
	"time"

	mysql "github.com/go-sql-driver/mysql"
	"gorm.io/gorm/schema"
)

// This test never reads site configuration. Use a disposable MySQL database whose
// name starts with anqicms_tag_index_test. All fixture tables are session-local.
func TestTagIndexesMySQL(t *testing.T) {
	dsn := os.Getenv("ANQICMS_TAG_INDEX_TEST_DSN")
	if dsn == "" {
		t.Skip("set ANQICMS_TAG_INDEX_TEST_DSN to an isolated MySQL database")
	}
	cfg, err := mysql.ParseDSN(dsn)
	if err != nil {
		t.Fatal(err)
	}
	if !strings.HasPrefix(cfg.DBName, "anqicms_tag_index_test") {
		t.Fatal("refusing non-test database: name must start with anqicms_tag_index_test")
	}
	db, err := sql.Open("mysql", dsn)
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()
	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Minute)
	defer cancel()
	conn, err := db.Conn(ctx)
	if err != nil {
		t.Fatal(err)
	}
	defer conn.Close()
	exec := func(query string) {
		t.Helper()
		if _, err := conn.ExecContext(ctx, query); err != nil {
			t.Fatal(err)
		}
	}
	exec("CREATE TEMPORARY TABLE tag_index_articles (id BIGINT PRIMARY KEY, created_time BIGINT NOT NULL, INDEX(created_time)) ENGINE=InnoDB")
	exec("CREATE TEMPORARY TABLE tag_index_relations (id INT UNSIGNED AUTO_INCREMENT PRIMARY KEY, tag_id INT NOT NULL, item_id BIGINT NOT NULL, INDEX(tag_id), INDEX(item_id)) ENGINE=InnoDB")
	for base := 1; base <= 20000; base += 500 {
		var articles, relations []string
		for id := base; id < base+500; id++ {
			articles = append(articles, fmt.Sprintf("(%d,%d)", id, (id*7919)%20000))
			// Tag 1 has 18000 articles; tag 2 has only 20.
			if id <= 18000 {
				relations = append(relations, fmt.Sprintf("(1,%d)", id))
			}
			if id <= 20 {
				relations = append(relations, fmt.Sprintf("(2,%d)", id))
			}
			relations = append(relations, fmt.Sprintf("(%d,%d)", 100+id%1000, id))
		}
		exec("INSERT INTO tag_index_articles VALUES " + strings.Join(articles, ","))
		exec("INSERT INTO tag_index_relations(tag_id,item_id) VALUES " + strings.Join(relations, ","))
	}
	queries := []string{
		"SELECT item_id FROM tag_index_relations WHERE tag_id=2 ORDER BY item_id LIMIT 20",
		"SELECT tag_id FROM tag_index_relations WHERE item_id=10 ORDER BY tag_id",
		"SELECT a.id FROM tag_index_articles a JOIN tag_index_relations t ON a.id=t.item_id WHERE t.tag_id=1 ORDER BY a.created_time DESC,a.id DESC LIMIT 20",
		"SELECT a.id FROM tag_index_articles a JOIN tag_index_relations t ON a.id=t.item_id WHERE t.tag_id=1 ORDER BY a.created_time DESC,a.id DESC LIMIT 20 OFFSET 10000",
	}
	read := func(query string) []int64 {
		t.Helper()
		rows, err := conn.QueryContext(ctx, query)
		if err != nil {
			t.Fatal(err)
		}
		defer rows.Close()
		var ids []int64
		for rows.Next() {
			var id int64
			if err := rows.Scan(&id); err != nil {
				t.Fatal(err)
			}
			ids = append(ids, id)
		}
		if err := rows.Err(); err != nil {
			t.Fatal(err)
		}
		return ids
	}
	plan := func(label, query string) string {
		t.Helper()
		var explain string
		if err := conn.QueryRowContext(ctx, "EXPLAIN FORMAT=JSON "+query).Scan(&explain); err != nil {
			t.Fatal(err)
		}
		t.Logf("%s: %s", label, explain)
		return explain
	}
	before := make([][]int64, len(queries))
	for i, q := range queries {
		before[i] = read(q)
		plan("before", q)
	}
	// Existing duplicates must not prevent adding the new indexes.
	exec("INSERT INTO tag_index_relations(tag_id,item_id) VALUES(1,20001),(1,20001)")
	s, err := schema.Parse(&TagData{}, &sync.Map{}, schema.NamingStrategy{})
	if err != nil {
		t.Fatal(err)
	}
	for _, index := range s.ParseIndexes() {
		if len(index.Fields) != 2 {
			continue
		}
		var columns []string
		for _, field := range index.Fields {
			columns = append(columns, "`"+field.DBName+"`")
		}
		class := ""
		if index.Class == "UNIQUE" {
			class = "UNIQUE "
		}
		exec("CREATE " + class + "INDEX `" + index.Name + "` ON tag_index_relations (" + strings.Join(columns, ",") + ")")
	}
	for i, q := range queries {
		if after := read(q); !reflect.DeepEqual(before[i], after) {
			t.Fatalf("query %d results changed", i)
		}
		p := plan("after", q)
		if i < 2 && !strings.Contains(p, `"using_index": true`) {
			t.Errorf("query %d is not covered by an index", i)
		}
	}
	// A unique index would break compatibility with existing duplicate relations.
	exec("INSERT INTO tag_index_relations(tag_id,item_id) VALUES(1,1),(1,1)")
}

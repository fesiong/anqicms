package provider

import (
	"database/sql"
	"database/sql/driver"
	"errors"
	"fmt"
	"reflect"
	"strings"
	"testing"

	"gorm.io/driver/mysql"
	"gorm.io/gorm"
	"kandaoni.com/anqicms/config"
	"kandaoni.com/anqicms/library"
	"kandaoni.com/anqicms/model"
	"kandaoni.com/anqicms/provider/fulltext"
	"kandaoni.com/anqicms/request"
)

type relatedSearchService struct {
	fakeFulltextService
	docs      []fulltext.TinyArchive
	err       error
	failPage  int
	pageTotal bool
}

func (s *relatedSearchService) Search(key string, module uint, page, size int) ([]fulltext.TinyArchive, int64, error) {
	if key != "CLW5061X" || module != 7 {
		return nil, 0, errors.New("incorrect search parameters")
	}
	if s.failPage == page {
		return nil, 0, errors.New("later page failed")
	}
	start := (page - 1) * size
	if start >= len(s.docs) {
		return nil, int64(len(s.docs)), s.err
	}
	end := min(start+size, len(s.docs))
	total := int64(len(s.docs))
	if s.pageTotal {
		total = int64(end - start)
	}
	return s.docs[start:end], total, s.err
}

// These tests catch bypassing the index, falling back after an index failure,
// and applying the large-site threshold to a model rather than the whole site.
func TestRelatedKeywordsPaging(t *testing.T) {
	for _, tc := range []struct {
		name string
		fail bool
	}{{name: "page-local total"}, {name: "later page error", fail: true}} {
		t.Run(tc.name, func(t *testing.T) {
			d := &randomListSQL{}
			d.respond = func(query string) (driver.Rows, error) {
				if strings.Contains(query, "FROM `archive_products`") {
					return &randomListRows{columns: []string{"archive_id"}}, nil
				}
				if strings.HasPrefix(query, "SELECT archives.id") {
					return &randomListRows{columns: []string{"id"}, data: [][]driver.Value{{int64(42)}, {int64(41)}}}, nil
				}
				if strings.Contains(query, "FROM `archives`") {
					return &randomListRows{columns: []string{"id", "fixed_link"}, data: [][]driver.Value{{int64(42), "/42"}, {int64(41), "/41"}}}, nil
				}
				return nil, fmt.Errorf("unexpected SQL: %s", query)
			}
			conn := sql.OpenDB(d)
			t.Cleanup(func() { _ = conn.Close() })
			db, err := gorm.Open(mysql.New(mysql.Config{Conn: conn, SkipInitializeWithVersion: true}), &gorm.Config{DisableAutomaticPing: true})
			if err != nil {
				t.Fatal(err)
			}
			w := &Website{DB: db, Cache: library.InitMemoryCache(), Content: &config.ContentConfig{}, System: &config.SystemConfig{}, MultiLanguage: &config.PluginMultiLangConfig{}, PluginStorage: &config.PluginStorageConfig{}, PluginFulltext: &config.PluginFulltextConfig{Open: true, Modules: []uint{7}}, parsedPattern: &RewritePattern{Parsed: true, Patterns: RewriteMode{}}}
			s := &relatedSearchService{pageTotal: true}
			for range 100 {
				s.docs = append(s.docs, fulltext.TinyArchive{Id: 9, Type: fulltext.CategoryType})
			}
			s.docs = append(s.docs, fulltext.TinyArchive{Id: 41, Type: fulltext.ArchiveType}, fulltext.TinyArchive{Id: 42, Type: fulltext.ArchiveType})
			if tc.fail {
				s.failPage = 3
				s.pageTotal = false
				for range 100 {
					s.docs = append(s.docs, fulltext.TinyArchive{Id: 9, Type: fulltext.CategoryType})
				}
			}
			w.searcher = s
			limit := 1
			if tc.fail {
				limit = 3
			}
			got := w.getRelatedKeywordArchives(&request.ApiArchiveListRequest{ModuleId: 7, Limit: limit, Offset: 1}, 0, "CLW5061X")
			ids := []int64{}
			for _, a := range got {
				ids = append(ids, a.Id)
			}
			want := []int64{42}
			if tc.fail {
				want = []int64{}
			}
			if !reflect.DeepEqual(ids, want) {
				t.Fatalf("got IDs %v, want %v", ids, want)
			}
		})
	}
}

func TestRelatedKeywordsSearchPolicy(t *testing.T) {
	for _, tt := range []struct {
		name          string
		open          bool
		module        int64
		modules       []uint
		estimate      string
		searchErr     error
		uninitialized bool
		want          string
		noSQL         bool
	}{
		{name: "indexed model", open: true, module: 7, modules: []uint{7}, want: "archives.`id` IN (42)"},
		{name: "indexed empty", open: true, module: 7, modules: []uint{7}, noSQL: true},
		{name: "indexed error", open: true, module: 7, modules: []uint{7}, searchErr: errors.New("search unavailable"), noSQL: true},
		{name: "indexed uninitialized", open: true, module: 7, modules: []uint{7}, uninitialized: true, noSQL: true},
		{name: "no module large site", open: true, modules: []uint{7}, estimate: "100001", want: "`keywords` like 'CLW5061X%'"},
		{name: "unindexed model large site", open: true, module: 7, modules: []uint{8}, estimate: "100001", want: "`keywords` like 'CLW5061X%'"},
		{name: "exact threshold", open: true, module: 7, modules: []uint{8}, estimate: "100000", want: "`keywords` like '%CLW5061X%'"},
		{name: "disabled large site", module: 7, modules: []uint{7}, estimate: "100001", want: "`keywords` like 'CLW5061X%'"},
	} {
		t.Run(tt.name, func(t *testing.T) {
			d := &randomListSQL{estimate: tt.estimate}
			conn := sql.OpenDB(d)
			t.Cleanup(func() { _ = conn.Close() })
			db, err := gorm.Open(mysql.New(mysql.Config{Conn: conn, SkipInitializeWithVersion: true}), &gorm.Config{DisableAutomaticPing: true})
			if err != nil {
				t.Fatal(err)
			}
			w := &Website{DB: db, Cache: library.InitMemoryCache(), Content: &config.ContentConfig{}, PluginFulltext: &config.PluginFulltextConfig{Open: tt.open, Modules: tt.modules}}
			_ = w.Cache.Set("modules", []model.Module{{Id: 7}}, 0)
			s := &relatedSearchService{err: tt.searchErr}
			if tt.name == "indexed model" {
				s.docs = []fulltext.TinyArchive{{Id: 42, Type: fulltext.ArchiveType}, {Id: 9, Type: fulltext.CategoryType}}
			}
			if !tt.uninitialized {
				w.searcher = s
			}
			w.ApiGetArchives(&request.ApiArchiveListRequest{Type: "related", Like: "keywords", Keywords: "CLW5061X", ModuleId: tt.module, CategoryIds: []int{3}, ExcludeCategoryIds: []int{4}, Limit: 12})
			queries := strings.Join(d.queries, "\n")
			if tt.noSQL {
				if queries != "" {
					t.Fatalf("indexed failure/empty result issued SQL: %s", queries)
				}
				return
			}
			if !strings.Contains(queries, tt.want) {
				t.Fatalf("missing %q in queries:\n%s", tt.want, queries)
			}
			for _, filter := range []string{"`category_id` = 3", "`category_id` NOT IN (4)", "archives.`id` != 0"} {
				if !strings.Contains(queries, filter) {
					t.Fatalf("missing filter %q: %s", filter, queries)
				}
			}
			if tt.name == "indexed model" && strings.Contains(strings.ToLower(queries), "like") {
				t.Fatalf("index bypass: %s", queries)
			}
			if tt.estimate != "" && !strings.Contains(queries, "EXPLAIN SELECT id FROM archives") {
				t.Fatalf("missing whole-site estimate: %s", queries)
			}
		})
	}
}

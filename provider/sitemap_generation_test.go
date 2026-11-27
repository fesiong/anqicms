package provider

import (
	"errors"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"sync/atomic"
	"testing"

	"gorm.io/driver/mysql"
	"gorm.io/gorm"
	"kandaoni.com/anqicms/config"
)

func TestBuildSitemapErrors(t *testing.T) {
	for _, name := range []string{"categories", "archives", "tags", "archive file", "tag file", "category file", "index file", "timestamp", "success"} {
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
			queryErr := errors.New("sitemap query failed")
			if err := db.Callback().Query().Replace("gorm:query", func(tx *gorm.DB) {
				if tx.Statement.Table == name {
					tx.AddError(queryErr)
				}
			}); err != nil {
				t.Fatal(err)
			}
			dir := t.TempDir() + string(os.PathSeparator)
			w := &Website{DB: db, System: &config.SystemConfig{}, PublicPath: dir, CachePath: dir, PluginSitemap: &config.PluginSitemapConfig{Type: "txt", PageSize: 100}}
			files := map[string]string{"archive file": "sitemap-archive-1.txt", "tag file": "sitemap-tag-1.txt", "category file": "sitemap-category.txt", "index file": "sitemap.txt", "timestamp": "sitemap-time.log"}
			if file := files[name]; file != "" {
				if err := os.Mkdir(filepath.Join(dir, file), 0700); err != nil {
					t.Fatal(err)
				}
			}
			err = w.BuildSitemap()
			if name == "success" {
				if err != nil {
					t.Fatal(err)
				}
				data, err := os.ReadFile(filepath.Join(dir, "sitemap.txt"))
				if err != nil || !strings.Contains(string(data), "/sitemap-category.txt") || !strings.Contains(string(data), "/sitemap-archive-1.txt") || !strings.Contains(string(data), "/sitemap-tag-1.txt") {
					t.Fatalf("invalid sitemap index %q: %v", data, err)
				}
				return
			}
			if err == nil {
				t.Fatal("generation failure reported as success")
			}
			if name == "categories" || name == "archives" || name == "tags" {
				if !errors.Is(err, queryErr) {
					t.Fatalf("lost query error: %v", err)
				}
				if _, err := os.Stat(filepath.Join(dir, "sitemap.txt")); !os.IsNotExist(err) {
					t.Fatalf("failed build published index: %v", err)
				}
			}
		})
	}
}

func TestEnsureSitemapFileCoalescesConcurrentRequests(t *testing.T) {
	path := filepath.Join(t.TempDir(), "sitemap.xml")
	var calls atomic.Int32
	build := func() error {
		calls.Add(1)
		return os.WriteFile(path, []byte("sitemap"), 0600)
	}
	var wg sync.WaitGroup
	errs := make(chan error, 20)
	for range 20 {
		wg.Add(1)
		go func() {
			defer wg.Done()
			errs <- ensureSitemapFile(path, build)
		}()
	}
	wg.Wait()
	close(errs)
	for err := range errs {
		if err != nil {
			t.Fatal(err)
		}
	}
	if calls.Load() != 1 {
		t.Fatalf("generated %d times, want one", calls.Load())
	}
	if err := ensureSitemapFile(path, build); err != nil {
		t.Fatal(err)
	}
	if calls.Load() != 1 {
		t.Fatal("existing sitemap was rebuilt")
	}
}

func TestEnsureSitemapFileRequiresOutput(t *testing.T) {
	path := filepath.Join(t.TempDir(), "sitemap.xml")
	if err := ensureSitemapFile(path, func() error { return nil }); err == nil {
		t.Fatal("missing output reported as successful generation")
	}
}

func TestEnsureSitemapFileRetriesAfterFailure(t *testing.T) {
	path := filepath.Join(t.TempDir(), "sitemap.xml")
	wantErr := errors.New("build failed")
	if err := ensureSitemapFile(path, func() error { return wantErr }); !errors.Is(err, wantErr) {
		t.Fatalf("got %v, want build failure", err)
	}
	if err := ensureSitemapFile(path, func() error {
		return os.WriteFile(path, []byte("retried"), 0600)
	}); err != nil {
		t.Fatal(err)
	}
	data, err := os.ReadFile(path)
	if err != nil || string(data) != "retried" {
		t.Fatalf("retry did not create sitemap: %q, %v", data, err)
	}
}

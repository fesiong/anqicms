package controller

import (
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"testing"

	"github.com/kataras/iris/v12"
	"kandaoni.com/anqicms/config"
	"kandaoni.com/anqicms/provider"
)

func TestFileServeSitemap(t *testing.T) {
	for _, tt := range []struct {
		name     string
		baseURI  string
		kind     string
		path     string
		existing bool
		want     int
	}{
		{"existing xml", "", "xml", "/sitemap.xml", true, http.StatusOK},
		{"existing txt under base", "/cms/", "txt", "/cms/sitemap.txt", true, http.StatusOK},
		{"missing root xml", "", "xml", "/sitemap.xml", false, http.StatusServiceUnavailable},
		{"missing root txt under base", "/cms/", "txt", "/cms/sitemap.txt", false, http.StatusServiceUnavailable},
		{"nested xml", "", "xml", "/other/sitemap.xml", false, http.StatusNotFound},
		{"nested txt under base", "/cms/", "txt", "/cms/other/sitemap.txt", false, http.StatusNotFound},
		{"suffix without separator", "", "xml", "/othersitemap.xml", false, http.StatusNotFound},
		{"wrong format", "", "xml", "/sitemap.txt", false, http.StatusNotFound},
		{"configured language", "", "xml", "/en/sitemap.xml", false, http.StatusServiceUnavailable},
		{"nested configured language", "", "xml", "/en/other/sitemap.xml", false, http.StatusNotFound},
		{"unknown language", "", "xml", "/fr/sitemap.xml", false, http.StatusNotFound},
	} {
		t.Run(tt.name, func(t *testing.T) {
			dir := t.TempDir()
			public := filepath.Join(dir, "public")
			if tt.existing {
				if err := os.Mkdir(public, 0700); err != nil {
					t.Fatal(err)
				}
				if err := os.WriteFile(filepath.Join(public, "sitemap."+tt.kind), []byte("existing sitemap"), 0600); err != nil {
					t.Fatal(err)
				}
			} else {
				if err := os.WriteFile(public, []byte("not a directory"), 0600); err != nil {
					t.Fatal(err)
				}
			}
			site := &provider.Website{
				RootPath:      dir + string(os.PathSeparator),
				PublicPath:    public + string(os.PathSeparator),
				BaseURI:       tt.baseURI,
				PluginSitemap: &config.PluginSitemapConfig{Type: tt.kind},
				MultiLanguage: &config.PluginMultiLangConfig{Open: true, Type: config.MultiLangTypeDirectory, SubSites: []config.MultiLangSite{{Language: "en"}}},
			}
			app := iris.New()
			app.Get("/{path:path}", func(ctx iris.Context) {
				if !fileServe(ctx, site) {
					ctx.StatusCode(http.StatusNotFound)
				}
			})
			if err := app.Build(); err != nil {
				t.Fatal(err)
			}
			rec := httptest.NewRecorder()
			app.ServeHTTP(rec, httptest.NewRequest(http.MethodGet, tt.path, nil))
			if rec.Code != tt.want {
				t.Fatalf("status = %d, want %d", rec.Code, tt.want)
			}
			if tt.existing && rec.Body.String() != "existing sitemap" {
				t.Fatalf("existing file changed: %q", rec.Body.String())
			}
		})
	}
}

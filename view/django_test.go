package view

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/flosch/pongo2/v6"
	"kandaoni.com/anqicms/config"
	"kandaoni.com/anqicms/provider"
)

func newTestEngine(t *testing.T, dir string, reload bool) (*DjangoEngine, *provider.Website) {
	t.Helper()
	s := Django(".html")
	if reload {
		s.Reload(true)
	}
	var siteId uint = 9527
	s.Set[siteId] = pongo2.NewSet("test-set", &tDjangoAssetLoader{
		fs:      os.DirFS(dir),
		rootDir: ".",
	})
	site := &provider.Website{}
	site.Id = siteId
	return s, site
}

func execTemplate(t *testing.T, tmpl *pongo2.Template) string {
	t.Helper()
	if tmpl == nil {
		t.Fatal("template is nil (page would render as 404 without any hint)")
	}
	out, err := tmpl.ExecuteBytes(nil)
	if err != nil {
		t.Fatalf("template execute failed: %v", err)
	}
	return string(out)
}

// 语法错误的模板：页面必须能渲染出具体错误信息，而不是 404 无提示。
func TestBrokenTemplateShowsParseError(t *testing.T) {
	dir := t.TempDir()
	broken := "<div>{% if x %}</div>"
	if err := os.WriteFile(filepath.Join(dir, "broken.html"), []byte(broken), 0644); err != nil {
		t.Fatal(err)
	}
	s, site := newTestEngine(t, dir, false)

	// 预解析报错，但错误必须被缓存为伪模板
	err := s.ParseTemplate(site, "broken.html", []byte(broken))
	if err == nil {
		t.Fatal("expected parse error for broken template")
	}

	tmpl := s.fromCache(site.Id, "broken.html")
	out := execTemplate(t, tmpl)
	if !strings.Contains(out, "on file broken.html") {
		t.Fatalf("error page should mention the file, got: %s", out)
	}
	if strings.Contains(out, "{% if") {
		t.Fatalf("error text must be escaped from pongo2 syntax, got: %s", out)
	}
}

// 模板修好后，即使不触发重载，访问也应恢复正常渲染（FromCache 优先于错误伪模板）。
func TestFixedTemplateRendersNormally(t *testing.T) {
	dir := t.TempDir()
	file := filepath.Join(dir, "page.html")
	broken := "<div>{% if x %}</div>"
	if err := os.WriteFile(file, []byte(broken), 0644); err != nil {
		t.Fatal(err)
	}
	s, site := newTestEngine(t, dir, false)

	_ = s.ParseTemplate(site, "page.html", []byte(broken))
	if s.fromCache(site.Id, "page.html") == nil {
		t.Fatal("expected error pseudo template for broken file")
	}

	// 修复文件（模拟后台保存后触发 template_reload 的 ParseTemplate 清缓存）
	fixed := "<div>ok</div>"
	if err := os.WriteFile(file, []byte(fixed), 0644); err != nil {
		t.Fatal(err)
	}
	if err := s.ParseTemplate(site, "page.html", []byte(fixed)); err != nil {
		t.Fatalf("unexpected parse error after fix: %v", err)
	}
	tmpl := s.fromCache(site.Id, "page.html")
	out := execTemplate(t, tmpl)
	if out != fixed {
		t.Fatalf("expected fixed render %q, got %q", fixed, out)
	}
}

// reload（development）模式下，坏模板同样要展示错误而不是 404。
func TestReloadModeShowsParseError(t *testing.T) {
	dir := t.TempDir()
	broken := "<div>{{ unclosed"
	if err := os.WriteFile(filepath.Join(dir, "broken2.html"), []byte(broken), 0644); err != nil {
		t.Fatal(err)
	}
	s, site := newTestEngine(t, dir, true)

	tmpl := s.fromCache(site.Id, "broken2.html")
	out := execTemplate(t, tmpl)
	if !strings.Contains(out, "on file broken2.html") {
		t.Fatalf("reload mode should also show error page, got: %s", out)
	}
}

// 后台切换模板（TemplateName 变化）后重载，渲染必须切换到新模板目录，
// 而不是继续读旧目录的文件（重载不彻底导致页面显示旧模板）。
func TestSwitchTemplateRendersNewTemplate(t *testing.T) {
	root := t.TempDir()
	dirA := filepath.Join(root, "template", "default")
	dirB := filepath.Join(root, "template", "other")
	if err := os.MkdirAll(dirA, 0755); err != nil {
		t.Fatal(err)
	}
	if err := os.MkdirAll(dirB, 0755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dirA, "index.html"), []byte("TEMPLATE-A"), 0644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dirB, "index.html"), []byte("TEMPLATE-B"), 0644); err != nil {
		t.Fatal(err)
	}

	s := Django(".html")
	var siteId uint = 9527
	site := &provider.Website{}
	site.Id = siteId
	site.RootPath = root + string(filepath.Separator)
	site.System = &config.SystemConfig{TemplateName: "default"}

	if err := s.LoadTemplates(siteId, site); err != nil {
		t.Fatalf("load template A failed: %v", err)
	}
	if out := execTemplate(t, s.fromCache(siteId, "index.html")); out != "TEMPLATE-A" {
		t.Fatalf("expected TEMPLATE-A, got %q", out)
	}

	// 模拟后台切换模板：System.TemplateName 变化后触发 template_reload
	site.System.TemplateName = "other"
	if err := s.LoadTemplates(siteId, site); err != nil {
		t.Fatalf("load template B failed: %v", err)
	}
	if out := execTemplate(t, s.fromCache(siteId, "index.html")); out != "TEMPLATE-B" {
		t.Fatalf("after switch, expected TEMPLATE-B, got %q (stale template dir)", out)
	}
}

// LoadTemplates：单个模板语法错误不得中断其余模板的加载。
func TestLoadTemplatesContinuesAfterError(t *testing.T) {
	root := t.TempDir()
	tplDir := filepath.Join(root, "template", "default")
	if err := os.MkdirAll(filepath.Join(tplDir, "sub"), 0755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(tplDir, "bad.html"), []byte("<div>{% if x %}</div>"), 0644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(tplDir, "sub", "good.html"), []byte("<div>good</div>"), 0644); err != nil {
		t.Fatal(err)
	}
	s := Django(".html")
	var siteId uint = 9527
	site := &provider.Website{}
	site.Id = siteId
	site.RootPath = root + string(filepath.Separator)
	site.System = &config.SystemConfig{TemplateName: "default"}

	err := s.LoadTemplates(site.Id, site)
	if err == nil {
		t.Fatal("expected error report from LoadTemplates")
	}
	// bad.html 之后遍历到的 sub/good.html 必须已正常编译，证明 walk 没有中断
	if _, err := s.Set[siteId].FromCache("sub/good.html"); err != nil {
		t.Fatalf("walk must not stop at the broken template: %v", err)
	}
	// 坏模板仍可访问并显示错误
	tmpl := s.fromCache(siteId, "bad.html")
	out := execTemplate(t, tmpl)
	if !strings.Contains(out, "on file bad.html") {
		t.Fatalf("broken template should render error page, got: %s", out)
	}
}

package provider

// 「按站点放行」的护栏（provider/siteAccess.go）。
//
// 这条边界很容易被顺手改坏：只要有人把判据从"站点在注册表里"换成
// "站点在 websites 表里"或者"phase == ready"，106 个站点的升级就会退回
// "全站停机到最后一个站点迁完"。这里把三种误放行的形态都钉住。

import (
	"go/ast"
	"go/parser"
	"go/token"
	"net/http"
	"net/http/httptest"
	"os"
	"testing"

	"github.com/kataras/iris/v12"
	"kandaoni.com/anqicms/config"
)

func withTempSites(t *testing.T, sites ...*Website) {
	t.Helper()
	for _, s := range sites {
		websites.Set(s)
		// 注册表 + Initialed 还不够放行，必须打上"可服务"标记（InitWebsite 末尾才打）。
		if s.Initialed && s.System != nil {
			markSiteServable(s.Id)
		}
	}
	t.Cleanup(func() {
		for _, s := range sites {
			websites.Delete(s.Id)
			unmarkSiteServable(s.Id)
		}
	})
}

func TestSiteReadyForHostOnlyAllowsMigratedSites(t *testing.T) {
	withTempSites(t,
		// 101：已迁移完成，根域名
		&Website{Id: 101, Initialed: true, System: &config.SystemConfig{BaseUrl: "http://a.example.com"}},
		// 102：已迁移完成，挂在二级目录
		&Website{Id: 102, Initialed: true, System: &config.SystemConfig{BaseUrl: "http://b.example.com/sub/"}},
		// 103：迁移失败 / 连库失败 → Initialed=false，必须 503
		&Website{Id: 103, Initialed: false, System: &config.SystemConfig{BaseUrl: "http://c.example.com"}},
		// 104：半初始化站点（System == nil）不能 panic，也不能放行
		&Website{Id: 104, Initialed: true},
	)

	cases := []struct {
		name string
		host string
		path string
		want bool
	}{
		{"已迁移站点首页", "a.example.com", "/", true},
		{"已迁移站点内页", "a.example.com", "/article/1.html", true},
		{"二级目录站点在其前缀内", "b.example.com", "/sub/x.html", true},
		{"二级目录站点前缀外不能放行", "b.example.com", "/other", false},
		{"迁移失败的站点", "c.example.com", "/", false},
		{"普通站点前缀不误伤", "b.example.com", "/", false},
		{"未注册的域名（还没轮到的站点）", "pending.example.com", "/", false},
		{"空 host", "", "/", false},
	}
	for _, c := range cases {
		if got := SiteReadyForHost(c.host, c.path); got != c.want {
			t.Fatalf("%s：SiteReadyForHost(%q, %q) = %v，期望 %v", c.name, c.host, c.path, got, c.want)
		}
	}
}

func TestSiteReadyRequiresRegisteredAndSystemLoaded(t *testing.T) {
	withTempSites(t,
		&Website{Id: 201, Initialed: true, System: &config.SystemConfig{BaseUrl: "http://ok.example.com"}},
		&Website{Id: 202, Initialed: false, System: &config.SystemConfig{BaseUrl: "http://bad.example.com"}},
		&Website{Id: 203, Initialed: true}, // System 未加载
	)

	if !SiteReady(201) {
		t.Fatal("已注册且 Initialed 的站点应判定为可服务")
	}
	if SiteReady(202) {
		t.Fatal("Initialed=false 的站点不能放行")
	}
	if SiteReady(203) {
		t.Fatal("System 为 nil 的半初始化站点不能放行，否则 handler 读 w.System.X 立刻 panic")
	}
	if SiteReady(0) || SiteReady(9999) {
		t.Fatal("未注册的站点不能放行")
	}

	// 半初始化窗口：已注册、Initialed、System 都在，但 limiter/cache 还没建完
	// （InitWebsite 末尾才打可服务标记）。这一段必须挡住。
	websites.Set(&Website{Id: 204, Initialed: true, System: &config.SystemConfig{BaseUrl: "http://mid.example.com"}})
	t.Cleanup(func() { websites.Delete(204); unmarkSiteServable(204) })
	if SiteReady(204) {
		t.Fatal("初始化尚未结束（未打可服务标记）的站点不能放行")
	}
	markSiteServable(204)
	if !SiteReady(204) {
		t.Fatal("打完可服务标记后应放行")
	}
}

// TestRequestServableDuringStartup 是闸门的判定矩阵：阶段、域名、显式站点三者
// 的组合顺序错一步，就会出现"放行了未迁移的站点"或者"已迁移的站点还要陪着等"。
func TestRequestServableDuringStartup(t *testing.T) {
	resetStartupStateForTest()
	defer resetStartupStateForTest()
	withTempSites(t,
		&Website{Id: 301, Initialed: true, System: &config.SystemConfig{BaseUrl: "http://ready.example.com"}},
		&Website{Id: 302, Initialed: true, System: &config.SystemConfig{BaseUrl: "http://ready2.example.com"}},
	)
	BeginStartup()

	type probe struct {
		name   string
		host   string
		header map[string]string
		want   bool
	}
	probes := []probe{
		{"已迁移完成的站点", "ready.example.com", nil, true},
		{"还在排队/正在迁移的站点", "pending.example.com", nil, false},
		{"后台显式声明已就绪站点", "pending.example.com", map[string]string{"Site-Id": "302"}, true},
		{"后台显式声明未就绪站点（域名不能救它）", "ready.example.com", map[string]string{"Site-Id": "9999"}, false},
		{"X-Site-Id 同口径", "pending.example.com", map[string]string{"X-Site-Id": "301"}, true},
		{"非法 Site-Id 值退回域名判定", "ready.example.com", map[string]string{"Site-Id": "abc"}, true},
	}

	got := make([]bool, 0, len(probes))
	app := iris.New()
	app.Get("/probe", func(ctx iris.Context) {
		got = append(got, RequestServable(ctx))
	})
	if err := app.Build(); err != nil {
		t.Fatalf("build: %v", err)
	}
	for _, p := range probes {
		req := httptest.NewRequest(http.MethodGet, "/probe", nil)
		req.Host = p.host
		for k, v := range p.header {
			req.Header.Set(k, v)
		}
		app.ServeHTTP(httptest.NewRecorder(), req)
	}
	for i, p := range probes {
		if got[i] != p.want {
			t.Fatalf("%s：RequestServable = %v，期望 %v", p.name, got[i], p.want)
		}
	}

	// panic 之后一律不放行：注册表状态未知，宁可全站等重启。
	FinishStartup(true)
	got = got[:0]
	req := httptest.NewRequest(http.MethodGet, "/probe", nil)
	req.Host = "ready.example.com"
	app.ServeHTTP(httptest.NewRecorder(), req)
	if got[0] {
		t.Fatal("failed 阶段不能按站点放行")
	}
}

// TestInitWebsiteMarksServableLast 锁住 InitWebsite 里的顺序：
//
//	迁移 → 清可服务标记 → 注册（websites.Set）→ ...初始化... → 打可服务标记
//
// 每一条都有代价：
//   - Set 必须在 GetRewritePattern 之前（它要回写注册表里同一个对象）；
//   - Set 必须在 InitFulltext 之前（fulltext.go 里 websites.MustGet(w.Id)）；
//   - 打标记必须在最后（limiter/cache 建好之前不能放行）。
//
// 混在一起的后果不是编译错误，是"Cache 还是 nil 就被放给线上流量"。
func TestInitWebsiteMarksServableLast(t *testing.T) {
	src, err := os.ReadFile("website.go")
	if err != nil {
		t.Fatalf("读取 website.go：%v", err)
	}
	fset := token.NewFileSet()
	file, err := parser.ParseFile(fset, "website.go", src, 0)
	if err != nil {
		t.Fatalf("解析 website.go：%v", err)
	}
	var body *ast.BlockStmt
	for _, d := range file.Decls {
		if fn, ok := d.(*ast.FuncDecl); ok && fn.Name.Name == "InitWebsite" {
			body = fn.Body
		}
	}
	if body == nil {
		t.Fatal("没找到 InitWebsite")
	}

	var unmark, migrate, set, fulltext, mark token.Pos
	ast.Inspect(body, func(n ast.Node) bool {
		call, ok := n.(*ast.CallExpr)
		if !ok {
			return true
		}
		switch fn := call.Fun.(type) {
		case *ast.SelectorExpr:
			if ident, ok := fn.X.(*ast.Ident); ok && ident.Name == "websites" && fn.Sel.Name == "Set" {
				set = call.Pos()
			}
			if ident, ok := fn.X.(*ast.Ident); ok && ident.Name == "w" && fn.Sel.Name == "InitFulltext" {
				fulltext = call.Pos()
			}
		case *ast.Ident:
			switch fn.Name {
			case "unmarkSiteServable":
				unmark = call.Pos()
			case "markSiteServable":
				mark = call.Pos()
			case "MigrateSiteSchema":
				migrate = call.Pos()
			}
		}
		return true
	})

	if unmark == token.NoPos || migrate == token.NoPos || set == token.NoPos || fulltext == token.NoPos || mark == token.NoPos {
		t.Fatalf("InitWebsite 里缺少关键调用：unmark=%v migrate=%v set=%v fulltext=%v mark=%v",
			unmark != token.NoPos, migrate != token.NoPos, set != token.NoPos, fulltext != token.NoPos, mark != token.NoPos)
	}
	if !(migrate < unmark && unmark < set && set < fulltext && fulltext < mark) {
		t.Fatal("InitWebsite 的顺序必须是：迁移 → 清标记 → 注册 → InitFulltext → 打可服务标记")
	}
}

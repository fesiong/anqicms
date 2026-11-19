package route

import (
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/kataras/iris/v12"
	"github.com/kataras/iris/v12/context"
	"github.com/kataras/iris/v12/core/router"
)

// 本文件锁住「前台页面接受 HEAD 请求」这件事。
//
// 为什么必须有护栏：iris 的 app.Get 只注册 GET，且 FireMethodNotAllowed 默认 false
// → 没注册的方法直接 404。把 base.go 里的 "GET HEAD" 改回 app.Get，仓库里没有任何
// 既有测试会变红，线上表现就是 HEAD 一律 404。
//
// 反向对照：把任意一处 "GET HEAD" 改回 iris.MethodGet，TestFrontendPagesAcceptHead
// 与 TestFrontendHeadReusesGetChain 都会立即报错。

// frontendPageRoutes 是需要支持 HEAD 的前台页面路由，写法与 route/base.go 源码一致。
var frontendPageRoutes = []string{
	"/{path:path}",                  // 首页/文档/分类/单页/标签/搜索/城市分站等全部前台页面
	"/guestbook.html",               // 留言页
	"/{base:string}/guestbook.html", // 多站点子目录下的留言页
	"/login",                        // 登录页
	"/login/{platform:string}",      // 第三方登录跳转页
	"/register",                     // 注册页
	"/register/{platform:string}",   // 第三方注册跳转页
	"/account/password/reset",       // 找回密码页
	"/account/{route:path}",         // 用户中心
	"/order/{route:path}",           // 订单中心
}

// routesBySource 用「方法 + 源码路径」建索引。
//
// 不用 Route.Path：iris 会把 {path:path} 改写成 /*path、{platform:string} 改写成
// :platform，拿改写后的字符串断言就得跟着框架版本漂移，而且和 base.go 对不上号。
// Route.Tmpl().Src 保留原始写法，是唯一能和源码一一对应的字段。
func routesBySource(t *testing.T) map[string]*router.Route {
	t.Helper()
	app := iris.New()
	Register(app, routeTestSystemFiles)
	app.Build()

	table := map[string]*router.Route{}
	for _, item := range app.GetRoutes() {
		if item.Method == "" || item.Tmpl().Src == "" {
			continue
		}
		table[item.Method+" "+item.Tmpl().Src] = item
	}
	return table
}

func handlerNames(r *router.Route) []string {
	names := make([]string, 0, len(r.Handlers))
	for _, h := range r.Handlers {
		names = append(names, context.HandlerName(h))
	}
	return names
}

// TestFrontendPagesAcceptHead 断言每条前台页面路由都注册了 HEAD。
func TestFrontendPagesAcceptHead(t *testing.T) {
	table := routesBySource(t)

	for _, path := range frontendPageRoutes {
		if _, ok := table["GET "+path]; !ok {
			t.Errorf("路由表里没有 GET %s：断言基线变了，先确认这条路由还在", path)
			continue
		}
		if _, ok := table["HEAD "+path]; !ok {
			t.Errorf("路由表里没有 HEAD %s（HEAD 请求会 404）", path)
		}
	}
}

// TestFrontendHeadReusesGetChain 断言 HEAD 与 GET 的处理链逐函数一致。
//
// 只注册 HEAD 却漏掉 Check301/ParseUserToken/HandlerTimeout 不会报错，
// 后果是 HEAD 拿到错误的状态码或未登录视图（例如该 301 的给了 200）。
func TestFrontendHeadReusesGetChain(t *testing.T) {
	table := routesBySource(t)

	for _, path := range frontendPageRoutes {
		getRoute, hasGet := table["GET "+path]
		headRoute, hasHead := table["HEAD "+path]
		if !hasGet || !hasHead {
			t.Errorf("%s：GET=%v HEAD=%v，缺一方就不必比对处理链", path, hasGet, hasHead)
			continue
		}
		getNames, headNames := handlerNames(getRoute), handlerNames(headRoute)
		if strings.Join(getNames, ",") != strings.Join(headNames, ",") {
			t.Errorf("%s 处理链不一致：\n GET  %v\n HEAD %v", path, getNames, headNames)
		}
	}
}

// TestNoHeadOnMutatingOrApiRoutes 断言有副作用 / 纯 JSON 的端点没有被顺手加上 HEAD。
//
// 反向对照：把 /logout 的 app.Get 改成 HandleMany("GET HEAD")，本用例立即变红。
func TestNoHeadOnMutatingOrApiRoutes(t *testing.T) {
	table := routesBySource(t)

	for _, path := range []string{
		"/logout",                     // 有副作用：清登录态
		"/comment/publish",            // 前台写端点
		"/comment/praise",             // 前台写端点
		"/api/archive/detail",         // 前台 JSON API
		"/api/sms/send",               // 短信发码
		"/system/api/archive/list",    // 后台 API
		"/system/api/plugin/sms/test", // 后台写端点
	} {
		if _, ok := table["HEAD "+path]; ok {
			t.Errorf("不应注册 HEAD %s", path)
		}
	}
}

// TestHeadResponseBodyDiscarded 验证「HEAD 直接复用 GET 处理链」这个前提：
// 处理函数照常写响应体，net/http 会把 body 丢掉，但仍按写入长度回 Content-Length，
// 状态码与 Content-Type 也和 GET 一致 —— 这正是搜索引擎只看头部的场景需要的。
//
// 这条锁的是框架行为而非业务代码：如果哪天有人为了「省渲染」给 HEAD 单独开一条
// 只解析不渲染的捷径，渲染被跳过后的状态码差异就没人兜底了，届时请连带更新 base.go
// 里那段注释。
func TestHeadResponseBodyDiscarded(t *testing.T) {
	app := iris.New()
	h := func(ctx iris.Context) {
		ctx.StatusCode(iris.StatusOK)
		ctx.ContentType("text/html; charset=utf-8")
		_, _ = ctx.WriteString("<html><body>hello-anqicms</body></html>")
	}
	app.HandleMany("GET HEAD", "/probe", h)
	app.Build()

	srv := httptest.NewServer(app)
	defer srv.Close()

	type result struct {
		status int
		ctype  string
		clen   int64
		body   string
	}
	do := func(method string) result {
		req, err := http.NewRequest(method, srv.URL+"/probe", nil)
		if err != nil {
			t.Fatalf("%s 构造请求失败：%v", method, err)
		}
		res, err := http.DefaultClient.Do(req)
		if err != nil {
			t.Fatalf("%s 请求失败：%v", method, err)
		}
		defer res.Body.Close()
		body, _ := io.ReadAll(res.Body)
		return result{res.StatusCode, res.Header.Get("Content-Type"), res.ContentLength, string(body)}
	}

	get, head := do(http.MethodGet), do(http.MethodHead)
	if get.body == "" {
		t.Fatalf("GET 没有响应体，用例失去意义")
	}
	if head.status != get.status {
		t.Errorf("HEAD 状态码 %d ≠ GET %d", head.status, get.status)
	}
	if head.ctype != get.ctype {
		t.Errorf("HEAD Content-Type %q ≠ GET %q", head.ctype, get.ctype)
	}
	if head.body != "" {
		t.Errorf("HEAD 不该有响应体，实际拿到 %q", head.body)
	}
	if head.clen != get.clen {
		t.Errorf("HEAD Content-Length %d ≠ GET %d（HEAD 应报告与 GET 相同的长度）", head.clen, get.clen)
	}
}

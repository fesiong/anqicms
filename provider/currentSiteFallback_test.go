package provider

import (
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/kataras/iris/v12"
	"kandaoni.com/anqicms/config"
)

// TestCurrentSiteWithoutSiteOneDoesNotPanic 锁住 fallback 的 nil 兜底。
//
// 触发条件真实存在：站点 1 还在迁移（启动期）、站点 1 被停用、或站点 1 连库失败
// 没能注册。过去这里是 `cloneWithContext(websites.MustGet(1), ctx)`，
// MustGet 找不到返回 nil → 解引用直接 panic，recover 后只剩一条栈，
// 看不出"原来是站点 1 不在注册表里"。
func TestCurrentSiteWithoutSiteOneDoesNotPanic(t *testing.T) {
	prev1, had1 := websites.Get(1)
	if had1 {
		websites.Delete(1)
	}
	// 只留一个未初始化的站点 2：注册表非空（Len != 0），但没有站点 1。
	websites.Set(&Website{Id: 2, Name: "二号站", RootPath: config.ExecPath, BaseURI: "/"})
	defer func() {
		websites.Delete(2)
		if had1 {
			websites.Set(prev1)
		}
	}()

	app := iris.New()
	// GetLocale 必须在 i18n 有词条时才能工作（没有词条时 iris 内部会 panic，
	// 与本次改动无关）。生产环境的 bootstrap.Init 也会先 Load。
	if err := app.I18n.Load("../locales/*/*.yml", config.LoadLocales()...); err != nil {
		t.Fatalf("加载语言包失败：%v", err)
	}
	app.I18n.SetDefault("zh-CN")
	var got *Website
	app.Get("/probe", func(ctx iris.Context) {
		got = CurrentSite(ctx)
	})
	if err := app.Build(); err != nil {
		t.Fatalf("build: %v", err)
	}

	rec := httptest.NewRecorder()
	app.ServeHTTP(rec, httptest.NewRequest(http.MethodGet, "/probe", nil))

	if got == nil {
		t.Fatal("没有站点 1 时 CurrentSite 必须返回一个空站点（createDefaultWebsite），不能返回 nil")
	}
	if got.Id != 0 {
		t.Fatalf("兜底站点应为 Id 0（安企安装语义），实际 %d", got.Id)
	}
	if got.System == nil {
		t.Fatal("兜底站点必须带非 nil 的 System，否则 handler 读 w.System.X 立刻 panic")
	}
}

package manageController

import (
	"os"
	"strings"
	"testing"

	"kandaoni.com/anqicms/config"
	"kandaoni.com/anqicms/model"
	"kandaoni.com/anqicms/provider"
)

// TestBuildWebsiteDetailResponseToleratesNilSystem 锁定「半初始化站点」的详情页不得 500。
//
// 线上证据（2026-11-15 error.log）：139 条 recover 全部是
// `provider/website.go` 里 `currentSite.System.Language` 的空指针，
// 触发的是 /system/api/website/list。详情接口是同一个坑的另一个入口：
// 它原来只判 `website != nil` 就读 `website.System.BaseUrl`。
//
// 数据来源：InitWebsite 先 websites.Set(&w) 再 w.InitSetting()，而 InitSetting
// 在数据库连不上时直接 return，注册表里因此长期留着 System == nil 的站点。
func TestBuildWebsiteDetailResponseToleratesNilSystem(t *testing.T) {
	dbSite := &model.Website{Id: 8, Name: "连不上库的站点", RootPath: "/www/anqi8/", Status: 0}
	broken := &provider.Website{Id: 8} // 已注册、未初始化

	got := buildWebsiteDetailResponse(dbSite, broken, "admin")

	if got.Id != 8 || got.Name != "连不上库的站点" || got.RootPath != "/www/anqi8/" {
		t.Fatalf("库里字段应原样输出，实得 %+v", got)
	}
	if got.BaseUrl != "" || got.FrontUrl != "" {
		t.Fatalf("拿不到运行时配置时不得编造网址，实得 base_url=%q front_url=%q", got.BaseUrl, got.FrontUrl)
	}
	if got.Initialed {
		t.Fatal("半初始化站点不能报告 initialed=true")
	}

	// 反向对照：证明容错分支是承重墙——裸取真的会 panic。
	func() {
		defer func() {
			if recover() == nil {
				t.Fatal("裸取 website.System.BaseUrl 竟然没 panic，说明护栏在空转")
			}
		}()
		_ = broken.System.BaseUrl
	}()
}

// TestBuildWebsiteDetailResponseNormalSite 正常站点行为不得被容错改坏。
func TestBuildWebsiteDetailResponseNormalSite(t *testing.T) {
	dbSite := &model.Website{Id: 9, Name: "正常站点", RootPath: "/www/anqi9/", Status: 1}
	site := &provider.Website{Id: 9, Initialed: true}
	site.System = &config.SystemConfig{BaseUrl: "https://www.example.com", FrontUrl: "https://front.example.com"}

	got := buildWebsiteDetailResponse(dbSite, site, "boss")

	if got.BaseUrl != "https://www.example.com" || got.FrontUrl != "https://front.example.com" {
		t.Fatalf("正常站点应补全运行时网址，实得 base_url=%q front_url=%q", got.BaseUrl, got.FrontUrl)
	}
	if !got.Initialed {
		t.Fatal("已初始化站点应报告 initialed=true")
	}
	if got.AdminUser != "boss" {
		t.Fatalf("管理员用户名应透传，实得 %q", got.AdminUser)
	}
}

// TestGetWebsiteInfoDoesNotDerefSystem 两个 handler 都不得绕过构造函数裸取 System。
func TestGetWebsiteInfoDoesNotDerefSystem(t *testing.T) {
	src, err := os.ReadFile("website.go")
	if err != nil {
		t.Fatalf("读取 website.go 失败: %v", err)
	}
	body := string(src)
	for _, sig := range []string{"func GetWebsiteInfo(", "func GetWebsiteList("} {
		fn := funcBody(t, body, sig)
		if !strings.Contains(fn, "buildWebsiteDetailResponse") {
			t.Fatalf("%s 未走 buildWebsiteDetailResponse，断言失去意义", sig)
		}
		if strings.Contains(fn, ".System.") {
			t.Fatalf("%s 又直接解引用 System：请走 buildWebsiteDetailResponse 的 nil 保护", sig)
		}
	}
}

// funcBody 截出函数体，找不到直接失败（函数改名后护栏要跟着改，不能静默失效）。
func funcBody(t *testing.T, src, signature string) string {
	t.Helper()
	i := strings.Index(src, signature)
	if i < 0 {
		t.Fatalf("未找到 %s", signature)
	}
	fn := src[i:]
	if n := strings.Index(fn, "\n}\n"); n > 0 {
		fn = fn[:n]
	}
	return fn
}

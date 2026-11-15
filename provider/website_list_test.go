package provider

import (
	"os"
	"strings"
	"testing"

	"kandaoni.com/anqicms/config"
	"kandaoni.com/anqicms/model"
)

// TestApplySiteRuntimeInfoToleratesNilSystem 锁定「半初始化站点」不得让站点列表 500。
//
// 背景：InitWebsite 先 `websites.Set(&w)` 再 `w.InitSetting()`，而 InitSetting
// 在 `w.DB == nil`（库连不上）时直接 return → 注册表里留下 System == nil 的站点。
// GetDBWebsites 过去直接取 `currentSite.System.Language`，空指针 panic 被 recover
// 兜成 500「服务器内部错误」，且只在包含该行的分页上触发（第 1 页正常、翻页必挂）。
func TestApplySiteRuntimeInfoToleratesNilSystem(t *testing.T) {
	site := &model.Website{Id: 42, Status: 1, BaseUrl: "http://from-db.example.com"}
	broken := &Website{Id: 42} // 已注册、未初始化：System 为 nil

	applySiteRuntimeInfo(site, broken)

	if site.BaseUrl != "http://from-db.example.com" {
		t.Fatalf("拿不到运行时配置时应保留库里的值，实得 %q", site.BaseUrl)
	}
	if site.Status != 0 {
		t.Fatalf("未初始化站点应标记为不可用（Status=0），实得 %d", site.Status)
	}
}

// TestApplySiteRuntimeInfoHandlesUnregistered 覆盖「库里有行、注册表里没有」。
func TestApplySiteRuntimeInfoHandlesUnregistered(t *testing.T) {
	site := &model.Website{Id: 7, Status: 1, Name: "未注册站点"}

	applySiteRuntimeInfo(site, nil)

	if site.Status != 0 {
		t.Fatalf("未注册站点应标记为不可用，实得 %d", site.Status)
	}
	if site.Name != "未注册站点" {
		t.Fatalf("其余字段应原样保留，实得 %q", site.Name)
	}
}

// TestApplySiteRuntimeInfoCopiesRuntimeFields 保证正常站点的补全行为没被容错改坏。
func TestApplySiteRuntimeInfoCopiesRuntimeFields(t *testing.T) {
	sys := &config.SystemConfig{Language: "en-US", BaseUrl: "http://runtime.example.com"}
	site := &model.Website{Id: 1, Status: 0}
	cur := &Website{Id: 1, Initialed: true, System: sys}

	applySiteRuntimeInfo(site, cur)

	if site.Language != sys.Language {
		t.Fatalf("Language 未补全，实得 %q", site.Language)
	}
	if site.BaseUrl != sys.BaseUrl {
		t.Fatalf("BaseUrl 未补全，实得 %q", site.BaseUrl)
	}
	if site.Status != 0 {
		t.Fatalf("库里 Status 应原样保留（已初始化不强制归 0），实得 %d", site.Status)
	}
}

// TestNilSystemDerefReallyPanics 是上面三个用例的反向对照：
// 证明「直接读 System 字段」确实会 panic，即容错分支是承重墙而不是摆设。
func TestNilSystemDerefReallyPanics(t *testing.T) {
	defer func() {
		if recover() == nil {
			t.Fatal("裸取 currentSite.System.Language 竟然没 panic，说明容错分支在空转")
		}
	}()

	broken := &Website{Id: 42}
	_ = broken.System.Language
}

// TestWebsiteListDoesNotOverwriteFirstRowByPosition 防止「按位置改写第一行」的回归。
//
// 历史 bug：`sites[0].Mysql = config.Server.Mysql` 是位置判断而非身份判断，
// 翻到第 2 页时会把第 21 号站点的库信息改写成默认库（实测 it 报 anqicms/aitool 互换）。
// 站点 1 的库信息由 handler 按 id 补，provider 层不应按行号动手。
func TestWebsiteListDoesNotOverwriteFirstRowByPosition(t *testing.T) {
	src, err := os.ReadFile("website.go")
	if err != nil {
		t.Fatalf("读取 website.go 失败: %v", err)
	}

	// 只看代码行：注释里为了说明历史 bug 会引用这行原文，不能算命中。
	// （栅栏是「别真的这么写」，不是「别提到它」。）
	var code strings.Builder
	for _, line := range strings.Split(string(src), "\n") {
		if trimmed := strings.TrimSpace(line); strings.HasPrefix(trimmed, "//") {
			continue
		}
		code.WriteString(line)
		code.WriteString("\n")
	}

	if strings.Contains(code.String(), "sites[0].Mysql") {
		t.Fatal("GetDBWebsites 又按位置改写了第一行的 Mysql：请改为按站点身份判断")
	}
}

// extractFunc 按函数签名截出函数体，用于源码级回归护栏。
func extractFunc(t *testing.T, src, signature string) string {
	t.Helper()
	i := strings.Index(src, signature)
	if i < 0 {
		t.Fatalf("未找到 %s（函数被改名或删除？护栏需同步更新）", signature)
	}
	body := src[i:]
	if n := strings.Index(body, "\n}\n"); n > 0 {
		body = body[:n]
	}
	if !strings.Contains(body, "{") {
		t.Fatalf("%s 的函数体提取失败：断言会恒真", signature)
	}
	return body
}

// TestGetTemplateDirToleratesNilSystem 半初始化站点的模板目录不得 panic。
//
// 调用方（备份、模板读写、htmlCache 清理）大多没有 recover 兜底，
// 一个 System == nil 的站点就能把整个请求打崩。
func TestGetTemplateDirToleratesNilSystem(t *testing.T) {
	root := "/tmp/anqicms-site3/"
	broken := &Website{Id: 3, RootPath: root}

	got := broken.GetTemplateDir()
	want := root + "template" + string(os.PathSeparator) + "default"
	if got != want {
		t.Fatalf("System 为 nil 时应退回默认模板目录 %q，实得 %q", want, got)
	}

	// 正常站点行为不得改变（含「没设模板名则默认 default」的既有语义）
	ok := &Website{Id: 3, RootPath: root, System: &config.SystemConfig{TemplateName: "acme"}}
	if got := ok.GetTemplateDir(); got != root+"template"+string(os.PathSeparator)+"acme" {
		t.Fatalf("正常站点模板目录被改坏，实得 %q", got)
	}
	empty := &Website{Id: 3, RootPath: root, System: &config.SystemConfig{}}
	if got := empty.GetTemplateDir(); got != want {
		t.Fatalf("模板名空时应回落 default，实得 %q", got)
	}
}

// TestGetDBWebsiteInfoDoesNotDerefSystem 详情接口必须复用 nil 安全的那套逻辑。
//
// 线上 error.log（2026-11-15）里 139 条 panic 全部来自列表接口的
// `currentSite.System.Language`；GetDBWebsiteInfo 是同一个坑的另一个入口，
// 它曾按同样的写法裸取 System，detail 接口迟早也会 500。
func TestGetDBWebsiteInfoDoesNotDerefSystem(t *testing.T) {
	src, err := os.ReadFile("website.go")
	if err != nil {
		t.Fatalf("读取 website.go 失败: %v", err)
	}
	body := extractFunc(t, string(src), "func GetDBWebsiteInfo(")
	if !strings.Contains(body, "applySiteRuntimeInfo") {
		t.Fatal("GetDBWebsiteInfo 未走 applySiteRuntimeInfo：下面的断言会失去意义")
	}
	// 只看代码行：注释里为了说明历史 bug 会引用 `currentSite.System.Language` 原文。
	var code strings.Builder
	for _, line := range strings.Split(body, "\n") {
		if trimmed := strings.TrimSpace(line); strings.HasPrefix(trimmed, "//") {
			continue
		}
		code.WriteString(line)
		code.WriteString("\n")
	}
	if strings.Contains(code.String(), ".System.") {
		t.Fatal("GetDBWebsiteInfo 又直接解引用 System：请改用 applySiteRuntimeInfo 降级")
	}
}

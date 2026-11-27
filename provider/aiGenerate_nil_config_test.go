package provider

import (
	"os"
	"strings"
	"testing"
	"time"

	"gorm.io/driver/mysql"
	"gorm.io/gorm"
)

// lazyTestDB 返回一个「非 nil、但真的查询就会失败」的 *gorm.DB。
//
// 用途：验证某个函数在进入 DB 访问之前就返回了。连接失败不影响用例，
// 因为护栏命中时根本不会发查询。
func lazyTestDB(t *testing.T) *gorm.DB {
	t.Helper()
	db, err := gorm.Open(mysql.New(mysql.Config{
		DSN:                       "unused:unused@tcp(127.0.0.1:1)/unused",
		SkipInitializeWithVersion: true,
	}), &gorm.Config{DisableAutomaticPing: true})
	if err != nil {
		t.Fatalf("构造惰性 gorm.DB 失败: %v", err)
	}
	sqlDB, err := db.DB()
	if err == nil {
		t.Cleanup(func() { _ = sqlDB.Close() })
	}
	return db
}

// TestAiGenerateArticlesToleratesNilConfig 锁定「半初始化站点被 crond 扫到」不得 panic。
//
// 背景（2026-11-16 线上崩溃）：
//
//	panic: invalid memory address or nil pointer dereference  addr=0x1
//	provider.(*Website).AiGenerateArticles(0x0?)  aiGenerate.go:80
//	crond.CollectArticles.func1                    crond.go:160
//
// InitWebsite 是「先 websites.Set(&w)，再 w.InitSetting()」，而 AiGenerateConfig
// 只有 InitSetting → LoadAiGenerateSetting 才会赋值。中间那段窗口里站点已经
// Initialed=true && DB != nil，crond 每 10 分钟的 CollectArticles 会把它扫进来，
// 裸取 w.AiGenerateConfig.Open（Open 是结构体第 2 个字节 → addr=0x1）直接崩进程。
// 兄弟函数 CollectArticles 早就有 `w.CollectorConfig == nil` 的同类护栏，此处补齐。
func TestAiGenerateArticlesToleratesNilConfig(t *testing.T) {
	site := &Website{Id: 9, DB: lazyTestDB(t)} // 已注册、库已就绪、设置还没加载

	done := make(chan struct{})
	go func() {
		defer close(done)
		defer func() {
			if r := recover(); r != nil {
				t.Errorf("AiGenerateArticles 在半初始化站点上 panic 了: %v", r)
			}
		}()
		site.AiGenerateArticles()
	}()

	select {
	case <-done:
	case <-time.After(5 * time.Second):
		t.Fatal("AiGenerateArticles 在配置为 nil 时没有立即返回（护栏失效，可能去查库了）")
	}
}

// TestNilAiGenerateConfigDerefReallyPanics 是上面那个用例的反向对照：
// 证明「裸取 AiGenerateConfig.Open」确实会 panic，护栏不是摆设。
func TestNilAiGenerateConfigDerefReallyPanics(t *testing.T) {
	defer func() {
		if recover() == nil {
			t.Fatal("裸取 halfInit.AiGenerateConfig.Open 竟然没 panic，说明护栏在空转")
		}
	}()

	halfInit := &Website{Id: 9}
	_ = halfInit.AiGenerateConfig.Open
}

// TestWebsiteConstructorsInitAiGenerateConfig 是根因侧的护栏：
// 构造函数必须让 AiGenerateConfig 一开始就非 nil，这样「注册了但设置还没加载」
// 的站点读到的是零值（Open=false，什么都不做），而不是 nil。
func TestWebsiteConstructorsInitAiGenerateConfig(t *testing.T) {
	if got := createDefaultWebsite(nil); got.AiGenerateConfig == nil {
		t.Fatal("createDefaultWebsite 没有初始化 AiGenerateConfig：未初始化站点读它会 panic")
	}

	src, err := os.ReadFile("website.go")
	if err != nil {
		t.Fatalf("读取 website.go 失败: %v", err)
	}
	body := extractFunc(t, string(src), "func InitWebsite(mw *model.Website) {")
	// 先确认锚点落在正确的函数里，否则下面的断言可能恒真。
	if !strings.Contains(body, "websites.Set(&w)") {
		t.Fatal("extractFunc 没截到 InitWebsite 的函数体（断言会失去意义）")
	}
	var code strings.Builder
	for _, line := range strings.Split(body, "\n") {
		if trimmed := strings.TrimSpace(line); strings.HasPrefix(trimmed, "//") {
			continue
		}
		code.WriteString(line)
		code.WriteString("\n")
	}
	if !strings.Contains(code.String(), "AiGenerateConfig:") {
		t.Fatal("InitWebsite 的 Website 字面量又没给 AiGenerateConfig 占位：注册表窗口里读它必然 panic")
	}
}

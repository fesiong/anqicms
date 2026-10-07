package manageController

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// TestHtmlCacheGoroutinesGuardNilSite 静态缓存的四个异步构建入口必须防住两类 nil。
//
// 实测踩过（2026-10-03）：`cache_build_index` 让整个服务进程 panic 退出
// （SIGSEGV @ pluginHtmlCache.go:147，nil pointer dereference on addr=0x20），
// 之后所有 MCP 调用返回 502。两层根因：
//  1. provider.GetWebsite 在站点未登记/已注销时返回 nil
//     （provider.RemoveWebsite 内部本身就对它的结果做 nil 判断，说明 nil 是预期返回）；
//  2. 更隐蔽：w2 非 nil，但 HtmlCacheStatus 是 nil 指针字段 —— Build*Cache 在
//     PluginHtmlCache.Open==false 时直接 return，不分配 Status。
//
// 异步任务里的 panic 会带走整个进程，所以这两道守卫缺一不可。
func TestHtmlCacheGoroutinesGuardNilSite(t *testing.T) {
	src, err := os.ReadFile(filepath.Join("pluginHtmlCache.go"))
	if err != nil {
		t.Fatalf("读取源文件失败: %v", err)
	}
	text := string(src)
	for _, fn := range []string{
		"PluginHtmlCacheBuildIndex",
		"PluginHtmlCacheBuildCategory",
		"PluginHtmlCacheBuildArchive",
		"PluginHtmlCacheBuildTag",
	} {
		i := strings.Index(text, "func "+fn+"(")
		if i < 0 {
			t.Errorf("未找到 %s", fn)
			continue
		}
		// 取该函数体（到下一个顶层 func 之前）
		j := strings.Index(text[i+1:], "\nfunc ")
		if j < 0 {
			j = len(text) - i
		}
		body := text[i : i+j]
		if !strings.Contains(body, "w2 == nil") {
			t.Errorf("%s 的异步 goroutine 缺少 w2==nil 守卫，会导致 nil panic 打挂进程", fn)
		}
		if !strings.Contains(body, "GetWebsite") {
			t.Errorf("%s 应通过 GetWebsite 取站点（守卫才有意义）", fn)
		}
		// 第二层：必须用 nil 安全的 setter，不能裸写 w2.HtmlCacheStatus.XXX
		if strings.Contains(body, "w2.HtmlCacheStatus.") {
			t.Errorf("%s 仍在裸写 w2.HtmlCacheStatus，该字段可能是 nil 指针，应改用 MarkHtmlCacheFinished()", fn)
		}
		if !strings.Contains(body, "MarkHtmlCacheFinished") {
			t.Errorf("%s 未调用 nil 安全的 MarkHtmlCacheFinished()", fn)
		}
	}
}

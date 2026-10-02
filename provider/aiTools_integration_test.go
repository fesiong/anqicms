package provider

import (
	"context"
	"log/slog"
	"sync"
	"testing"
)

// initTestSite 初始化真实站点（库内 id=1）。
//
// ⚠️ 不要删：本函数是目前 provider 测试包里唯一的站点初始化入口，
// 且它所在的测试会先于 api_catalog_gen_test.go 等文件执行。移走它之后
// TestAPIMetaToolsWorkWithoutSource 会在 GetMcpConfig → CurrentSite(nil) 上
// nil 指针 panic（那些用例需要一个已初始化的站点）。
func initTestSite(t *testing.T) *Website {
	dbSite, err := GetDBWebsiteInfo(1)
	if err != nil {
		t.Fatal(err)
	}
	dbSite.Status = 1
	InitWebsite(dbSite)
	w := GetWebsite(1)
	return w
}

// testServiceWithSite creates an AiChatService with a real database for integration testing.
func testServiceWithSite(t *testing.T, w *Website) *AiChatService {
	t.Helper()
	svc := &AiChatService{
		mu:       sync.RWMutex{},
		sessions: make(map[string]*ChatSession),
		Logger:   slog.Default(),
		db:       w.DB,
		site:     w,
	}
	svc.Tools, svc.Handlers = svc.getEinoTools()
	return svc
}

// TestIntegration_NoAPI capsRemain 在真实站点上确认：端点型工具已全部退场，
// 只剩那批没有 REST 等价端点、必须保留为能力实现工具的工具仍有 handler。
//
// 这是对「方案 B」的收口断言 —— 如果哪天有人把 archive_list 之类加回 aiTools.go，
// 这里会立刻报错，避免两处实现再次并行漂移。
func TestIntegration_NoAPICapsRemain(t *testing.T) {
	w := initTestSite(t)
	svc := testServiceWithSite(t, w)

	kept := []string{
		"attachment_upload", "template_reload",
		"skill_search", "skill_install",
		"agent_create", "agent_list", "agent_delete", "agent_toggle", "agent_run", "agent_chat",
		"task",
	}
	for _, name := range kept {
		if _, ok := svc.Handlers[name]; !ok {
			t.Errorf("无端点能力 %s 必须保留 handler", name)
		}
	}

	gone := []string{
		"archive_list", "archive_get", "archive_create", "archive_update", "archive_delete",
		"category_list", "category_create", "tag_list", "module_update",
		"setting_system", "statistic_dashboard", "website_info", "template_get_info",
	}
	for _, name := range gone {
		if _, ok := svc.Handlers[name]; ok {
			t.Errorf("端点型工具 %s 应已删除（改由意图层直连端点）", name)
		}
	}

	// 保留下来的工具仍可被调用（不因站点已初始化而崩溃）。
	if h, ok := svc.Handlers["agent_list"]; ok {
		if _, err := h(context.Background(), `{}`); err != nil {
			t.Errorf("agent_list 调用失败: %v", err)
		}
	}
}

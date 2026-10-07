package provider

import (
	"context"
	"log/slog"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"testing"
)

// repoRootForTest 返回仓库根的绝对路径（单测 CWD 在 provider/，仓库根是其上一级）。
// 供 testService 设置 projectRoot：内置文件工具的路径校验只认绝对路径。
func repoRootForTest() string {
	cwd, err := os.Getwd()
	if err != nil {
		return ".."
	}
	return filepath.Dir(cwd)
}

// testService creates an AiChatService without a database (site=nil) for unit testing.
// All handlers will return "错误：站点未初始化" for DB-dependent operations.
func testService() *AiChatService {
	svc := &AiChatService{
		mu:       sync.RWMutex{},
		sessions: make(map[string]*ChatSession),
		Logger:   slog.Default(),
		db:       nil,
		site:     nil,
		// 内置文件工具（read_file/grep/glob 等）一律以 projectRoot 为根目录做路径校验，
		// 未配置时它们会直接返回「projectRoot 未配置」，文件类用例就全成了空转。
		// 且校验用的是绝对路径，必须给绝对仓库根（单测 CWD 在 provider/，仓库根是其上一级）。
		projectRoot: repoRootForTest(),
	}
	svc.Tools, svc.Handlers = svc.getEinoTools()
	// Also load built-in tools
	builtinTools, builtinHandlers := svc.getBuiltinEinoTools()
	svc.Tools = append(svc.Tools, builtinTools...)
	for name, handler := range builtinHandlers {
		svc.Handlers[name] = handler
	}
	return svc
}

// expectedTools 是 getEinoTools 必须提供的工具名。
//
// 只含"没有 REST 等价端点、必须保留为能力"的那批：端点型工具已改由意图层
// 经 cap_routes.go 直连后台端点，不再有工具定义。
//
// 断言方式是**成员校验**而非数量相等：工具增删是常态，写死总数会让每次
// 正常调整都把测试打红（这个坑在端点目录那边已经踩过）。
var expectedTools = []string{
	// 无 REST 等价端点的能力
	"attachment_upload", // 端点要 multipart 文件，AI 侧是 base64/URL/本地路径
	"template_reload",   // RestartChan 重载信号
	"agent_create",
	"agent_list",
	"agent_delete",
	"agent_toggle",
	"agent_run",
	"agent_chat",
	"skill_search",
	"skill_install",
	"task",
	// 内置文件/shell 工具
	"read_file",
	"write_file",
	"edit_file",
	"search_replace",
	"bash",
	"grep",
	"glob",
	"list_directory",
	// Web 工具
	"web_fetch",
	"web_search",
}

func TestGetEinoTools_AllDefined(t *testing.T) {
	svc := testService()

	// 已删除的端点型工具不应再出现在工具清单里（防止回退）。
	for _, gone := range []string{"archive_list", "category_create", "module_update", "setting_system"} {
		for _, ti := range svc.Tools {
			if ti.Name == gone {
				t.Errorf("端点型工具 %q 应已改为直连端点，不应再有工具定义", gone)
			}
		}
	}

	// Check that all expected tools exist
	nameSet := make(map[string]bool)
	for _, ti := range svc.Tools {
		nameSet[ti.Name] = true
		// Each tool must have a description
		if ti.Desc == "" {
			t.Errorf("tool %q has empty description", ti.Name)
		}
		// Each tool must have a parameter schema (even if nil/empty)
		if ti.ParamsOneOf == nil {
			t.Errorf("tool %q has nil ParamsOneOf", ti.Name)
		}
	}
	for _, name := range expectedTools {
		if !nameSet[name] {
			t.Errorf("expected tool %q not found in getEinoTools() output", name)
		}
	}

	// Check all handlers are registered
	for _, ti := range svc.Tools {
		if _, exists := svc.Handlers[ti.Name]; !exists {
			t.Errorf("handler for tool %q not registered", ti.Name)
		}
	}
}

func Test_GetAllTools_ReturnsAll(t *testing.T) {
	svc := testService()
	mcpTools := svc.GetAllTools()

	nameSet := make(map[string]bool)
	for _, mt := range mcpTools {
		nameSet[mt.Name] = true
	}
	for _, name := range expectedTools {
		if !nameSet[name] {
			t.Errorf("expected MCP tool %q not found", name)
		}
	}
}

// Test_UnknownTool 未注册的工具名必须查不到 handler。
func Test_UnknownTool(t *testing.T) {
	svc := testService()
	_, exists := svc.Handlers["nonexistent_tool"]
	if exists {
		t.Error("nonexistent_tool 不应存在 handler")
	}
}

// --- Argument parsing tests ---

// --- Built-in tool tests ---

func Test_ReadFile_ParseArgs(t *testing.T) {
	svc := testService()
	handler := svc.Handlers["read_file"]

	// Empty path
	result, err := handler(context.Background(), `{}`)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if result != "错误：文件路径不能为空" {
		t.Fatalf("expected '文件路径不能为空', got %q", result)
	}

	// Nonexistent file
	result, err = handler(context.Background(), `{"path":"nonexistent_file_xxx.go"}`)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !strings.Contains(result, "错误：文件不存在") {
		t.Fatalf("expected '文件不存在', got %q", result)
	}

	// Invalid JSON
	_, err = handler(context.Background(), `bad`)
	if err == nil {
		t.Fatal("expected error for invalid JSON")
	}
}

func Test_WriteFile_ParseArgs(t *testing.T) {
	svc := testService()
	handler := svc.Handlers["write_file"]

	// Empty path
	result, err := handler(context.Background(), `{}`)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if result != "错误：文件路径不能为空" {
		t.Fatalf("expected '文件路径不能为空', got %q", result)
	}

	// Missing content — will still try because struct defaults to empty
	result, err = handler(context.Background(), `{"path":"cache/test.txt"}`)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	// Invalid JSON
	_, err = handler(context.Background(), `invalid`)
	if err == nil {
		t.Fatal("expected error for invalid JSON")
	}
}

func Test_EditFile_ParseArgs(t *testing.T) {
	svc := testService()
	handler := svc.Handlers["edit_file"]

	// Empty path
	result, err := handler(context.Background(), `{"search":"old","replace":"new"}`)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if result != "错误：文件路径和搜索文本不能为空" {
		t.Fatalf("expected '文件路径...不能为空', got %q", result)
	}

	// Empty search — path is empty, so same error
	result, err = handler(context.Background(), `{"path":"x","search":"","replace":"y"}`)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if result != "错误：文件路径和搜索文本不能为空" {
		t.Fatalf("expected '文件路径...不能为空', got %q", result)
	}

	// Nonexistent file
	result, err = handler(context.Background(), `{"path":"nonexistent.go","search":"old","replace":"new"}`)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !strings.Contains(result, "错误：文件不存在") {
		t.Fatalf("expected '文件不存在', got %q", result)
	}

	// Invalid JSON
	_, err = handler(context.Background(), `bad`)
	if err == nil {
		t.Fatal("expected error for invalid JSON")
	}
}

func Test_SearchReplace_ParseArgs(t *testing.T) {
	svc := testService()
	handler := svc.Handlers["search_replace"]

	// Empty search
	result, err := handler(context.Background(), `{}`)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if result != "错误：搜索文本不能为空" {
		t.Fatalf("expected '搜索文本不能为空', got %q", result)
	}

	// Valid args — will try to glob and find no matches
	result, err = handler(context.Background(), `{"search":"old","replace":"new","glob":"*.nonexistent"}`)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if result != "未找到匹配的文件" {
		t.Fatalf("expected '未找到匹配的文件', got %q", result)
	}

	// Invalid JSON
	_, err = handler(context.Background(), `bad`)
	if err == nil {
		t.Fatal("expected error for invalid JSON")
	}
}

func Test_Bash_ParseArgs(t *testing.T) {
	svc := testService()
	handler := svc.Handlers["bash"]

	// Empty command
	result, err := handler(context.Background(), `{}`)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if result != "错误：命令不能为空" {
		t.Fatalf("expected '命令不能为空', got %q", result)
	}

	// Simple command
	result, err = handler(context.Background(), `{"command":"echo hello"}`)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !strings.Contains(result, "hello") {
		t.Fatalf("expected 'hello' in output, got %q", result)
	}

	// Invalid JSON
	_, err = handler(context.Background(), `bad`)
	if err == nil {
		t.Fatal("expected error for invalid JSON")
	}
}

func Test_Grep_ParseArgs(t *testing.T) {
	svc := testService()
	handler := svc.Handlers["grep"]

	// Empty pattern
	result, err := handler(context.Background(), `{}`)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if result != "错误：搜索模式不能为空" {
		t.Fatalf("expected '搜索模式不能为空', got %q", result)
	}

	// Search for something that exists in the current file
	result, err = handler(context.Background(), `{"pattern":"Test_Grep_ParseArgs","glob":"*_test.go"}`)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !strings.Contains(result, "Test_Grep_ParseArgs") {
		t.Fatalf("expected to find pattern, got %q", result)
	}

	// Invalid regex — now uses literal fallback instead of error
	// The `[invalid` is treated as literal and searches the whole project.
	// It may or may not find a match, but should never error.
	result, err = handler(context.Background(), `{"pattern":"[invalid","glob":"*.txt"}`)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}

func Test_Glob_ParseArgs(t *testing.T) {
	svc := testService()
	handler := svc.Handlers["glob"]

	// Empty pattern
	result, err := handler(context.Background(), `{}`)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if result != "错误：文件匹配模式不能为空" {
		t.Fatalf("expected '文件匹配模式不能为空', got %q", result)
	}

	// Find Go files
	result, err = handler(context.Background(), `{"pattern":"aiTools.go"}`)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !strings.Contains(result, "aiTools.go") {
		t.Fatalf("expected to find aiTools.go, got %q", result)
	}

	// Non-matching pattern
	result, err = handler(context.Background(), `{"pattern":"*.nonexistent_extension"}`)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !strings.Contains(result, "未找到匹配的文件") {
		t.Fatalf("expected '未找到匹配的文件', got %q", result)
	}
}

func Test_ListDirectory_ParseArgs(t *testing.T) {
	svc := testService()
	handler := svc.Handlers["list_directory"]

	// Default (root)
	result, err := handler(context.Background(), `{}`)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !strings.Contains(result, "📁") {
		t.Fatalf("expected directory listing, got %q", result)
	}

	// Point to a file (not a dir)
	result, err = handler(context.Background(), `{"path":"provider/aiTools.go"}`)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !strings.Contains(result, "错误") || !strings.Contains(result, "是一个文件") {
		t.Fatalf("expected '是一个文件', got %q", result)
	}

	// Nonexistent directory
	result, err = handler(context.Background(), `{"path":"nonexistent_dir_xyz"}`)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !strings.Contains(result, "错误：目录不存在") {
		t.Fatalf("expected '目录不存在', got %q", result)
	}
}

func Test_WebFetch_ParseArgs(t *testing.T) {
	svc := testService()
	handler := svc.Handlers["web_fetch"]

	// Empty URL
	result, err := handler(context.Background(), `{}`)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if result != "错误：URL 不能为空" {
		t.Fatalf("expected 'URL 不能为空', got %q", result)
	}

	// Invalid URL scheme
	result, err = handler(context.Background(), `{"url":"ftp://example.com"}`)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !strings.Contains(result, "错误：URL 格式不正确") {
		t.Fatalf("expected 'URL 格式不正确', got %q", result)
	}

	// Blocked localhost
	result, err = handler(context.Background(), `{"url":"http://localhost:8080"}`)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !strings.Contains(result, "错误：不允许访问内网地址") {
		t.Fatalf("expected '不允许访问内网地址', got %q", result)
	}

	// Invalid JSON
	_, err = handler(context.Background(), `bad`)
	if err == nil {
		t.Fatal("expected error for invalid JSON")
	}
}

func Test_WebSearch_ParseArgs(t *testing.T) {
	svc := testService()
	handler := svc.Handlers["web_search"]

	// Empty query
	result, err := handler(context.Background(), `{}`)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if result != "错误：搜索关键词不能为空" {
		t.Fatalf("expected '搜索关键词不能为空', got %q", result)
	}

	// Invalid JSON
	_, err = handler(context.Background(), `bad`)
	if err == nil {
		t.Fatal("expected error for invalid JSON")
	}
}

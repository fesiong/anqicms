package provider

import (
	"context"
	"log/slog"
	"strings"
	"sync"
	"testing"
)

// testService creates an AiChatService without a database (site=nil) for unit testing.
// All handlers will return "错误：站点未初始化" for DB-dependent operations.
func testService() *AiChatService {
	svc := &AiChatService{
		mu:       sync.RWMutex{},
		sessions: make(map[string]*ChatSession),
		Logger:   slog.Default(),
		mcpSrv:   nil,
		db:       nil,
		site:     nil,
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

// expectedTools defines the expected set of AI tool names.
var expectedTools = []string{
	"archive_list",
	"archive_get",
	"archive_create",
	"archive_delete",
	"archive_publish",
	"module_list",
	"module_get",
	"module_create",
	"module_delete",
	"category_list",
	"category_get",
	"category_create",
	"category_delete",
	"page_list",
	"page_get",
	"page_create",
	"page_delete",
	"tag_list",
	"tag_get",
	"tag_create",
	"tag_delete",
	"archive_tag_update",
	// Template tools
	"template_get_info",
	"template_get_file",
	"template_modify_file",
	"template_get_static",
	"template_modify_static",
	"template_reload",
	// Built-in file/shell tools
	"read_file",
	"write_file",
	"edit_file",
	"search_replace",
	"bash",
	"grep",
	"glob",
	"list_directory",
	// Web tools
	"web_fetch",
	"web_search",
}

func TestGetEinoTools_AllDefined(t *testing.T) {
	svc := testService()

	if len(svc.Tools) != len(expectedTools) {
		t.Errorf("expected %d tools, got %d", len(expectedTools), len(svc.Tools))
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

	if len(mcpTools) != len(expectedTools) {
		t.Errorf("expected %d MCP tools, got %d", len(expectedTools), len(mcpTools))
	}

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

// --- Argument parsing tests ---

func Test_ArchiveList_ParseArgs(t *testing.T) {
	svc := testService()
	handler := svc.Handlers["archive_list"]

	// Valid JSON with all optional params
	result, err := handler(context.Background(), `{"page":2,"page_size":5,"category_id":1}`)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if result != "错误：站点未初始化" {
		t.Fatalf("expected '站点未初始化', got %q", result)
	}

	// Empty JSON (all defaults)
	result, err = handler(context.Background(), `{}`)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if result != "错误：站点未初始化" {
		t.Fatalf("expected '站点未初始化', got %q", result)
	}

	// Invalid JSON
	_, err = handler(context.Background(), `not-json`)
	if err == nil {
		t.Fatal("expected error for invalid JSON")
	}
}

func Test_ArchiveGet_ParseArgs(t *testing.T) {
	svc := testService()
	handler := svc.Handlers["archive_get"]

	// Valid args
	result, err := handler(context.Background(), `{"archive_id":1}`)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if result != "错误：站点未初始化" {
		t.Fatalf("expected '站点未初始化', got %q", result)
	}

	// Missing archive_id — handler will still try GetArchiveById(0) which needs DB
	result, err = handler(context.Background(), `{}`)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	// Invalid JSON
	_, err = handler(context.Background(), `invalid`)
	if err == nil {
		t.Fatal("expected error for invalid JSON")
	}
}

func Test_ArchiveCreate_ParseArgs(t *testing.T) {
	svc := testService()
	handler := svc.Handlers["archive_create"]

	// Valid args
	result, err := handler(context.Background(), `{"title":"Test","content":"Content","category_id":1}`)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if result != "错误：站点未初始化" {
		t.Fatalf("expected '站点未初始化', got %q", result)
	}

	// Missing title → returns error message (not error)
	result, err = handler(context.Background(), `{"content":"Content","category_id":1}`)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if result != "错误：文档标题不能为空" {
		t.Fatalf("expected '文档标题不能为空', got %q", result)
	}

	// Missing content
	result, err = handler(context.Background(), `{"title":"Test","category_id":1}`)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if result != "错误：文档内容不能为空" {
		t.Fatalf("expected '文档内容不能为空', got %q", result)
	}

	// Missing category_id
	result, err = handler(context.Background(), `{"title":"Test","content":"Content"}`)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if result != "错误：请指定分类ID" {
		t.Fatalf("expected '请指定分类ID', got %q", result)
	}

	// Invalid JSON
	_, err = handler(context.Background(), `not-json`)
	if err == nil {
		t.Fatal("expected error for invalid JSON")
	}
}

func Test_ArchiveDelete_ParseArgs(t *testing.T) {
	svc := testService()
	handler := svc.Handlers["archive_delete"]

	// Valid args
	result, err := handler(context.Background(), `{"archive_id":5}`)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if result != "错误：站点未初始化" {
		t.Fatalf("expected '站点未初始化', got %q", result)
	}

	// Missing archive_id
	result, err = handler(context.Background(), `{}`)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	// Invalid JSON
	_, err = handler(context.Background(), `bad`)
	if err == nil {
		t.Fatal("expected error for invalid JSON")
	}
}

func Test_ArchivePublish_ParseArgs(t *testing.T) {
	svc := testService()
	handler := svc.Handlers["archive_publish"]

	// Valid args
	result, err := handler(context.Background(), `{"archive_id":1,"status":1}`)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if result != "错误：站点未初始化" {
		t.Fatalf("expected '站点未初始化', got %q", result)
	}

	// Missing status
	result, err = handler(context.Background(), `{"archive_id":1}`)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	// Invalid JSON
	_, err = handler(context.Background(), `bad`)
	if err == nil {
		t.Fatal("expected error for invalid JSON")
	}
}

func Test_CategoryList_ParseArgs(t *testing.T) {
	svc := testService()
	handler := svc.Handlers["category_list"]

	// Even empty args should work (no params required)
	result, err := handler(context.Background(), `{}`)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if result != "错误：站点未初始化" {
		t.Fatalf("expected '站点未初始化', got %q", result)
	}
}

func Test_CategoryGet_ParseArgs(t *testing.T) {
	svc := testService()
	handler := svc.Handlers["category_get"]

	// Valid args
	result, err := handler(context.Background(), `{"category_id":1}`)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if result != "错误：站点未初始化" {
		t.Fatalf("expected '站点未初始化', got %q", result)
	}

	// Missing category_id
	result, err = handler(context.Background(), `{}`)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	// Invalid JSON
	_, err = handler(context.Background(), `bad`)
	if err == nil {
		t.Fatal("expected error for invalid JSON")
	}
}

func Test_CategoryCreate_ParseArgs(t *testing.T) {
	svc := testService()
	handler := svc.Handlers["category_create"]

	// Valid args
	result, err := handler(context.Background(), `{"title":"Test Category","parent_id":0}`)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if result != "错误：站点未初始化" {
		t.Fatalf("expected '站点未初始化', got %q", result)
	}

	// Missing title
	result, err = handler(context.Background(), `{}`)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if result != "错误：分类名称不能为空" {
		t.Fatalf("expected '分类名称不能为空', got %q", result)
	}

	// Invalid JSON
	_, err = handler(context.Background(), `bad`)
	if err == nil {
		t.Fatal("expected error for invalid JSON")
	}
}

func Test_CategoryDelete_ParseArgs(t *testing.T) {
	svc := testService()
	handler := svc.Handlers["category_delete"]

	// Valid args
	result, err := handler(context.Background(), `{"category_id":1}`)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if result != "错误：站点未初始化" {
		t.Fatalf("expected '站点未初始化', got %q", result)
	}

	// Missing category_id
	result, err = handler(context.Background(), `{}`)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	// Invalid JSON
	_, err = handler(context.Background(), `bad`)
	if err == nil {
		t.Fatal("expected error for invalid JSON")
	}
}

func Test_TagList_ParseArgs(t *testing.T) {
	svc := testService()
	handler := svc.Handlers["tag_list"]

	// Even empty args should work
	result, err := handler(context.Background(), `{}`)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if result != "错误：站点未初始化" {
		t.Fatalf("expected '站点未初始化', got %q", result)
	}
}

func Test_TagGet_ParseArgs(t *testing.T) {
	svc := testService()
	handler := svc.Handlers["tag_get"]

	// Valid args
	result, err := handler(context.Background(), `{"tag_id":1}`)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if result != "错误：站点未初始化" {
		t.Fatalf("expected '站点未初始化', got %q", result)
	}

	// Missing tag_id
	result, err = handler(context.Background(), `{}`)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	// Invalid JSON
	_, err = handler(context.Background(), `bad`)
	if err == nil {
		t.Fatal("expected error for invalid JSON")
	}
}

func Test_TagCreate_ParseArgs(t *testing.T) {
	svc := testService()
	handler := svc.Handlers["tag_create"]

	// Valid args
	result, err := handler(context.Background(), `{"title":"Test Tag"}`)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if result != "错误：站点未初始化" {
		t.Fatalf("expected '站点未初始化', got %q", result)
	}

	// Missing title
	result, err = handler(context.Background(), `{}`)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if result != "错误：标签名称不能为空" {
		t.Fatalf("expected '标签名称不能为空', got %q", result)
	}

	// Invalid JSON
	_, err = handler(context.Background(), `bad`)
	if err == nil {
		t.Fatal("expected error for invalid JSON")
	}
}

func Test_TagDelete_ParseArgs(t *testing.T) {
	svc := testService()
	handler := svc.Handlers["tag_delete"]

	// Valid args
	result, err := handler(context.Background(), `{"tag_id":1}`)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if result != "错误：站点未初始化" {
		t.Fatalf("expected '站点未初始化', got %q", result)
	}

	// Missing tag_id
	result, err = handler(context.Background(), `{}`)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	// Invalid JSON
	_, err = handler(context.Background(), `bad`)
	if err == nil {
		t.Fatal("expected error for invalid JSON")
	}
}

func Test_UnknownTool(t *testing.T) {
	svc := testService()
	// Verify that unknown tool names return an appropriate error via the handler map
	_, exists := svc.Handlers["nonexistent_tool"]
	if exists {
		t.Fatal("handler for nonexistent tool should not exist")
	}
}

func Test_ModuleList_ParseArgs(t *testing.T) {
	svc := testService()
	handler := svc.Handlers["module_list"]

	// Even empty args should work (no params required)
	result, err := handler(context.Background(), `{}`)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if result != "错误：站点未初始化" {
		t.Fatalf("expected '站点未初始化', got %q", result)
	}
}

func Test_ModuleGet_ParseArgs(t *testing.T) {
	svc := testService()
	handler := svc.Handlers["module_get"]

	// Valid args
	result, err := handler(context.Background(), `{"module_id":1}`)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if result != "错误：站点未初始化" {
		t.Fatalf("expected '站点未初始化', got %q", result)
	}

	// Missing module_id
	result, err = handler(context.Background(), `{}`)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	// Invalid JSON
	_, err = handler(context.Background(), `bad`)
	if err == nil {
		t.Fatal("expected error for invalid JSON")
	}
}

func Test_ModuleCreate_ParseArgs(t *testing.T) {
	svc := testService()
	handler := svc.Handlers["module_create"]

	// Valid args
	result, err := handler(context.Background(), `{"title":"News","table_name":"news"}`)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if result != "错误：站点未初始化" {
		t.Fatalf("expected '站点未初始化', got %q", result)
	}

	// Missing title
	result, err = handler(context.Background(), `{"table_name":"news"}`)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if result != "错误：模型名称不能为空" {
		t.Fatalf("expected '模型名称不能为空', got %q", result)
	}

	// Missing table_name
	result, err = handler(context.Background(), `{"title":"News"}`)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if result != "错误：表名不能为空" {
		t.Fatalf("expected '表名不能为空', got %q", result)
	}

	// Invalid JSON
	_, err = handler(context.Background(), `bad`)
	if err == nil {
		t.Fatal("expected error for invalid JSON")
	}
}

func Test_ModuleDelete_ParseArgs(t *testing.T) {
	svc := testService()
	handler := svc.Handlers["module_delete"]

	// Valid args
	result, err := handler(context.Background(), `{"module_id":1}`)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if result != "错误：站点未初始化" {
		t.Fatalf("expected '站点未初始化', got %q", result)
	}

	// Missing module_id
	result, err = handler(context.Background(), `{}`)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	// Invalid JSON
	_, err = handler(context.Background(), `bad`)
	if err == nil {
		t.Fatal("expected error for invalid JSON")
	}
	// Invalid JSON
	_, err = handler(context.Background(), `bad`)
	if err == nil {
		t.Fatal("expected error for invalid JSON")
	}
}

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
	result, err = handler(context.Background(), `{"path":"test.txt"}`)
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

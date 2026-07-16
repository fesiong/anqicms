package tools

import (
	"context"
	"encoding/json"
	"fmt"

	"github.com/modelcontextprotocol/go-sdk/mcp"
)

// ArchiveProvider is the interface that provides archive data operations
type ArchiveProvider interface {
	ListArchives(req ArchiveListRequest) (*ArchiveListResult, error)
	GetArchive(id uint) (*ArchiveRecord, error)
	CreateArchive(req ArchiveCreateRequest) (uint, error)
	UpdateArchive(req ArchiveUpdateRequest) error
	DeleteArchive(id uint) error
	PublishArchive(id uint, status uint) error
}

// ArchiveRecord represents an archive (article) record
type ArchiveRecord struct {
	ID           uint      `json:"id"`
	Title        string    `json:"title"`
	Content      string    `json:"content"`
	CategoryID   uint      `json:"category_id"`
	Status       uint      `json:"status"`
	Keywords     string    `json:"keywords"`
	Description  string    `json:"description"`
	Cover        string    `json:"cover"`
	Author       string    `json:"author"`
	TagIDs       []uint    `json:"tag_ids"`
	CreatedTime  int64     `json:"created_time"`
	UpdatedTime  int64     `json:"updated_time"`
	PublishTime  int64     `json:"publish_time"`
	ViewCount    int       `json:"view_count"`
	IsTop        uint      `json:"is_top"`
}

// TagInfo represents a tag
type TagInfo struct {
	ID   uint   `json:"id"`
	Name string `json:"name"`
}

// ArchiveListRequest is the request for listing archives
type ArchiveListRequest struct {
	Page       int    `json:"page"`
	PageSize   int    `json:"page_size"`
	CategoryID uint   `json:"category_id"`
	Status     uint   `json:"status"`
	Keyword    string `json:"keyword"`
	OrderBy    string `json:"order_by"`
	OrderDir   string `json:"order_dir"`
}

// ArchiveListResult is the result for listing archives
type ArchiveListResult struct {
	Items  []ArchiveRecord `json:"items"`
	Total  int             `json:"total"`
	Page   int             `json:"page"`
	Total_ int             `json:"total_pages"`
}

// ArchiveCreateRequest is the request for creating an archive
type ArchiveCreateRequest struct {
	Title       string   `json:"title"`
	Content     string   `json:"content"`
	CategoryID  uint     `json:"category_id"`
	Keywords    string   `json:"keywords"`
	Description string   `json:"description"`
	Cover       string   `json:"cover"`
	Author      string   `json:"author"`
	Status      uint     `json:"status"`
	TagIDs      []uint   `json:"tag_ids"`
	PublishTime int64    `json:"publish_time"`
}

// ArchiveUpdateRequest is the request for updating an archive
type ArchiveUpdateRequest struct {
	ID           uint     `json:"id"`
	Title        string   `json:"title"`
	Content      string   `json:"content"`
	CategoryID   uint     `json:"category_id"`
	Keywords     string   `json:"keywords"`
	Description  string   `json:"description"`
	Cover        string   `json:"cover"`
	Author       string   `json:"author"`
	Status       uint     `json:"status"`
	TagIDs       []uint   `json:"tag_ids"`
	PublishTime  int64    `json:"publish_time"`
}

// ArchiveTools contains all archive-related MCP tools
type ArchiveTools struct {
	Provider ArchiveProvider
}

// NewArchiveTools creates a new ArchiveTools instance
func NewArchiveTools(provider ArchiveProvider) ArchiveTools {
	return ArchiveTools{Provider: provider}
}

// ToolDef is a pair of Tool and its handler
type ToolDef struct {
	Tool    *mcp.Tool
	Handler mcp.ToolHandler
}

// GetAll returns all archive tool definitions
func (at ArchiveTools) GetAll() []ToolDef {
	return []ToolDef{
		{archiveListTool(), at.handleList},
		{archiveGetTool(), at.handleGet},
		{archiveCreateTool(), at.handleCreate},
		{archiveUpdateTool(), at.handleUpdate},
		{archiveDeleteTool(), at.handleDelete},
		{archivePublishTool(), at.handlePublish},
	}
}

// schemaObj creates a JSON schema object
func schemaObj(props map[string]any, required []string) map[string]any {
	return map[string]any{
		"type":       "object",
		"properties": props,
		"required":   required,
	}
}

// prop creates a property definition
func prop(typ, desc string, def any) map[string]any {
	p := map[string]any{
		"type":        typ,
		"description": desc,
	}
	if def != nil {
		p["default"] = def
	}
	return p
}

// propRequired creates a required property definition
func propRequired(typ, desc string) map[string]any {
	return map[string]any{
		"type":        typ,
		"description": desc,
	}
}

// propArray creates an array property definition
func propArray(typ, desc string) map[string]any {
	return map[string]any{
		"type":        "array",
		"description": desc,
		"items":       map[string]any{"type": typ},
	}
}

// archiveListTool returns the archive_list tool definition
// extractArgs extracts and parses arguments from the request
func extractArgs(req *mcp.CallToolRequest) (map[string]any, error) {
	if len(req.Params.Arguments) == 0 {
		return make(map[string]any), nil
	}
	var args map[string]any
	if err := json.Unmarshal(req.Params.Arguments, &args); err != nil {
		return nil, fmt.Errorf("failed to parse arguments: %w", err)
	}
	return args, nil
}

// archiveListTool returns the archive_list tool definition
func archiveListTool() *mcp.Tool {
	return &mcp.Tool{
		Name:        "archive_list",
		Description: "分页获取文章列表，支持按分类、状态、关键词筛选和排序",
		InputSchema: schemaObj(map[string]any{
			"page":       prop("integer", "页码，从1开始", 1),
			"page_size":  prop("integer", "每页数量，最大100", 10),
			"category_id": prop("integer", "分类ID筛选", nil),
			"status":     prop("integer", "状态筛选：0=草稿，1=已发布，2=已下架", nil),
			"keyword":    prop("string", "关键词搜索（标题/内容）", nil),
			"order_by":   prop("string", "排序字段，默认created_time", "created_time"),
			"order_dir":  prop("string", "排序方向：asc或desc", "desc"),
		}, nil),
	}
}

// archiveGetTool returns the archive_get tool definition
func archiveGetTool() *mcp.Tool {
	return &mcp.Tool{
		Name:        "archive_get",
		Description: "获取单篇文章的完整详情",
		InputSchema: schemaObj(map[string]any{
			"archive_id": propRequired("integer", "文章ID"),
		}, []string{"archive_id"}),
	}
}

// archiveCreateTool returns the archive_create tool definition
func archiveCreateTool() *mcp.Tool {
	return &mcp.Tool{
		Name:        "archive_create",
		Description: "创建新文章，支持自动SEO分析和标签关联",
		InputSchema: schemaObj(map[string]any{
			"title":       propRequired("string", "文章标题"),
			"content":     propRequired("string", "文章内容（支持Markdown）"),
			"category_id": propRequired("integer", "分类ID"),
			"keywords":    prop("string", "关键词，逗号分隔", nil),
			"description": prop("string", "文章描述", nil),
			"cover":       prop("string", "封面图URL", nil),
			"author":      prop("string", "作者", nil),
			"status":      prop("integer", "状态：0草稿，1发布，2下架", 0),
			"tag_ids":     propArray("integer", "标签ID列表"),
			"publish_time": prop("integer", "发布时间戳，不传则使用当前时间", nil),
		}, []string{"title", "content", "category_id"}),
	}
}

// archiveUpdateTool returns the archive_update tool definition
func archiveUpdateTool() *mcp.Tool {
	return &mcp.Tool{
		Name:        "archive_update",
		Description: "更新文章信息",
		InputSchema: schemaObj(map[string]any{
			"id":           propRequired("integer", "文章ID"),
			"title":        prop("string", "文章标题", nil),
			"content":      prop("string", "文章内容（支持Markdown）", nil),
			"category_id":  prop("integer", "分类ID", nil),
			"keywords":     prop("string", "关键词，逗号分隔", nil),
			"description":  prop("string", "文章描述", nil),
			"cover":        prop("string", "封面图URL", nil),
			"author":       prop("string", "作者", nil),
			"status":       prop("integer", "状态：0草稿，1发布，2下架", nil),
			"tag_ids":      propArray("integer", "标签ID列表"),
			"publish_time": prop("integer", "发布时间戳", nil),
		}, []string{"id"}),
	}
}

// archiveDeleteTool returns the archive_delete tool definition
func archiveDeleteTool() *mcp.Tool {
	return &mcp.Tool{
		Name:        "archive_delete",
		Description: "删除文章（软删除）",
		InputSchema: schemaObj(map[string]any{
			"archive_id": propRequired("integer", "文章ID"),
		}, []string{"archive_id"}),
	}
}

// archivePublishTool returns the archive_publish tool definition
func archivePublishTool() *mcp.Tool {
	return &mcp.Tool{
		Name:        "archive_publish",
		Description: "发布或下架文章",
		InputSchema: schemaObj(map[string]any{
			"archive_id": propRequired("integer", "文章ID"),
			"status":     prop("integer", "状态：1=发布，2=下架", 1),
		}, []string{"archive_id"}),
	}
}

// handleList handles archive_list tool calls
func (at ArchiveTools) handleList(ctx context.Context, req *mcp.CallToolRequest) (*mcp.CallToolResult, error) {
	args, err := extractArgs(req)
	if err != nil {
		return &mcp.CallToolResult{
			Content: []mcp.Content{&mcp.TextContent{Text: fmt.Sprintf("invalid parameters: %v", err)}},
			IsError: true,
		}, nil
	}

	var page, pageSize int
	if v, ok := args["page"].(float64); ok {
		page = int(v)
	}
	if v, ok := args["page_size"].(float64); ok {
		pageSize = int(v)
	}

	listReq := ArchiveListRequest{
		Page:     page,
		PageSize: pageSize,
	}
	if v, ok := args["category_id"].(float64); ok {
		listReq.CategoryID = uint(v)
	}
	if v, ok := args["status"].(float64); ok {
		listReq.Status = uint(v)
	}
	if v, ok := args["keyword"].(string); ok {
		listReq.Keyword = v
	}
	if v, ok := args["order_by"].(string); ok {
		listReq.OrderBy = v
	}
	if v, ok := args["order_dir"].(string); ok {
		listReq.OrderDir = v
	}

	// Defaults
	if listReq.Page < 1 {
		listReq.Page = 1
	}
	if listReq.PageSize < 1 || listReq.PageSize > 100 {
		listReq.PageSize = 10
	}

	result, err := at.Provider.ListArchives(listReq)
	if err != nil {
		res := &mcp.CallToolResult{
			Content: []mcp.Content{&mcp.TextContent{Text: fmt.Sprintf("failed to list archives: %v", err)}},
			IsError: true,
		}
		return res, nil
	}

	content := &mcp.TextContent{Text: fmt.Sprintf("Found %d archives", result.Total)}
	structured, _ := json.Marshal(result)

	return &mcp.CallToolResult{
		Content:           []mcp.Content{content},
		StructuredContent: structured,
	}, nil
}

// handleGet handles archive_get tool calls
func (at ArchiveTools) handleGet(ctx context.Context, req *mcp.CallToolRequest) (*mcp.CallToolResult, error) {
	args, err := extractArgs(req)
	if err != nil {
		return &mcp.CallToolResult{
			Content: []mcp.Content{&mcp.TextContent{Text: fmt.Sprintf("invalid parameters: %v", err)}},
			IsError: true,
		}, nil
	}

	archiveID, _ := args["archive_id"].(float64)
	id := uint(archiveID)

	archive, err := at.Provider.GetArchive(id)
	if err != nil {
		res := &mcp.CallToolResult{
			Content: []mcp.Content{&mcp.TextContent{Text: fmt.Sprintf("failed to get archive: %v", err)}},
			IsError: true,
		}
		return res, nil
	}

	content := &mcp.TextContent{Text: fmt.Sprintf("Archive %d: %s", id, archive.Title)}
	structured, _ := json.Marshal(archive)

	return &mcp.CallToolResult{
		Content:           []mcp.Content{content},
		StructuredContent: structured,
	}, nil
}

// handleCreate handles archive_create tool calls
func (at ArchiveTools) handleCreate(ctx context.Context, req *mcp.CallToolRequest) (*mcp.CallToolResult, error) {
	args, err := extractArgs(req)
	if err != nil {
		return &mcp.CallToolResult{
			Content: []mcp.Content{&mcp.TextContent{Text: fmt.Sprintf("invalid parameters: %v", err)}},
			IsError: true,
		}, nil
	}

	title, _ := args["title"].(string)
	content, _ := args["content"].(string)
	categoryID, _ := args["category_id"].(float64)

	createReq := ArchiveCreateRequest{
		Title:      title,
		Content:    content,
		CategoryID: uint(categoryID),
	}
	if v, ok := args["keywords"].(string); ok {
		createReq.Keywords = v
	}
	if v, ok := args["description"].(string); ok {
		createReq.Description = v
	}
	if v, ok := args["cover"].(string); ok {
		createReq.Cover = v
	}
	if v, ok := args["author"].(string); ok {
		createReq.Author = v
	}
	if v, ok := args["status"].(float64); ok {
		createReq.Status = uint(v)
	}
	if v, ok := args["tag_ids"].([]any); ok {
		tagIDs := make([]uint, 0, len(v))
		for _, item := range v {
			if num, ok := item.(float64); ok {
				tagIDs = append(tagIDs, uint(num))
			}
		}
		createReq.TagIDs = tagIDs
	}
	if v, ok := args["publish_time"].(float64); ok {
		createReq.PublishTime = int64(v)
	}

	if err := ValidateArchiveCreate(createReq); err != nil {
		res := &mcp.CallToolResult{
			Content: []mcp.Content{&mcp.TextContent{Text: fmt.Sprintf("validation error: %v", err)}},
			IsError: true,
		}
		return res, nil
	}

	id, err := at.Provider.CreateArchive(createReq)
	if err != nil {
		res := &mcp.CallToolResult{
			Content: []mcp.Content{&mcp.TextContent{Text: fmt.Sprintf("failed to create archive: %v", err)}},
			IsError: true,
		}
		return res, nil
	}

	contentText := &mcp.TextContent{Text: fmt.Sprintf("Archive created with ID: %d", id)}
	result := map[string]any{"success": true, "archive_id": id}
	structured, _ := json.Marshal(result)

	return &mcp.CallToolResult{
		Content:           []mcp.Content{contentText},
		StructuredContent: structured,
	}, nil
}

// handleUpdate handles archive_update tool calls
func (at ArchiveTools) handleUpdate(ctx context.Context, req *mcp.CallToolRequest) (*mcp.CallToolResult, error) {
	args, err := extractArgs(req)
	if err != nil {
		return &mcp.CallToolResult{
			Content: []mcp.Content{&mcp.TextContent{Text: fmt.Sprintf("invalid parameters: %v", err)}},
			IsError: true,
		}, nil
	}

	id, _ := args["id"].(float64)
	updateReq := ArchiveUpdateRequest{ID: uint(id)}

	if v, ok := args["title"].(string); ok {
		updateReq.Title = v
	}
	if v, ok := args["content"].(string); ok {
		updateReq.Content = v
	}
	if v, ok := args["category_id"].(float64); ok {
		updateReq.CategoryID = uint(v)
	}
	if v, ok := args["keywords"].(string); ok {
		updateReq.Keywords = v
	}
	if v, ok := args["description"].(string); ok {
		updateReq.Description = v
	}
	if v, ok := args["cover"].(string); ok {
		updateReq.Cover = v
	}
	if v, ok := args["author"].(string); ok {
		updateReq.Author = v
	}
	if v, ok := args["status"].(float64); ok {
		updateReq.Status = uint(v)
	}
	if v, ok := args["tag_ids"].([]any); ok {
		tagIDs := make([]uint, 0, len(v))
		for _, item := range v {
			if num, ok := item.(float64); ok {
				tagIDs = append(tagIDs, uint(num))
			}
		}
		updateReq.TagIDs = tagIDs
	}
	if v, ok := args["publish_time"].(float64); ok {
		updateReq.PublishTime = int64(v)
	}

	if err := ValidateArchiveUpdate(updateReq); err != nil {
		res := &mcp.CallToolResult{
			Content: []mcp.Content{&mcp.TextContent{Text: fmt.Sprintf("validation error: %v", err)}},
			IsError: true,
		}
		return res, nil
	}

	if err := at.Provider.UpdateArchive(updateReq); err != nil {
		res := &mcp.CallToolResult{
			Content: []mcp.Content{&mcp.TextContent{Text: fmt.Sprintf("failed to update archive: %v", err)}},
			IsError: true,
		}
		return res, nil
	}

	contentText := &mcp.TextContent{Text: fmt.Sprintf("Archive %d updated successfully", id)}
	result := map[string]any{"success": true, "archive_id": id}
	structured, _ := json.Marshal(result)

	return &mcp.CallToolResult{
		Content:           []mcp.Content{contentText},
		StructuredContent: structured,
	}, nil
}

// handleDelete handles archive_delete tool calls
func (at ArchiveTools) handleDelete(ctx context.Context, req *mcp.CallToolRequest) (*mcp.CallToolResult, error) {
	args, err := extractArgs(req)
	if err != nil {
		return &mcp.CallToolResult{
			Content: []mcp.Content{&mcp.TextContent{Text: fmt.Sprintf("invalid parameters: %v", err)}},
			IsError: true,
		}, nil
	}

	archiveID, _ := args["archive_id"].(float64)
	id := uint(archiveID)

	if err := at.Provider.DeleteArchive(id); err != nil {
		res := &mcp.CallToolResult{
			Content: []mcp.Content{&mcp.TextContent{Text: fmt.Sprintf("failed to delete archive: %v", err)}},
			IsError: true,
		}
		return res, nil
	}

	contentText := &mcp.TextContent{Text: fmt.Sprintf("Archive %d deleted successfully", id)}
	result := map[string]any{"success": true, "archive_id": id}
	structured, _ := json.Marshal(result)

	return &mcp.CallToolResult{
		Content:           []mcp.Content{contentText},
		StructuredContent: structured,
	}, nil
}

// handlePublish handles archive_publish tool calls
func (at ArchiveTools) handlePublish(ctx context.Context, req *mcp.CallToolRequest) (*mcp.CallToolResult, error) {
	args, err := extractArgs(req)
	if err != nil {
		return &mcp.CallToolResult{
			Content: []mcp.Content{&mcp.TextContent{Text: fmt.Sprintf("invalid parameters: %v", err)}},
			IsError: true,
		}, nil
	}

	archiveID, _ := args["archive_id"].(float64)
	id := uint(archiveID)
	status, _ := args["status"].(float64)

	if err := at.Provider.PublishArchive(id, uint(status)); err != nil {
		res := &mcp.CallToolResult{
			Content: []mcp.Content{&mcp.TextContent{Text: fmt.Sprintf("failed to publish archive: %v", err)}},
			IsError: true,
		}
		return res, nil
	}

	statusText := "published"
	if uint(status) == 2 {
		statusText = "unpublished"
	}
	contentText := &mcp.TextContent{Text: fmt.Sprintf("Archive %d %s successfully", id, statusText)}
	result := map[string]any{"success": true, "archive_id": id, "status": status}
	structured, _ := json.Marshal(result)

	return &mcp.CallToolResult{
		Content:           []mcp.Content{contentText},
		StructuredContent: structured,
	}, nil
}

// ValidateArchiveCreate validates the archive creation request
func ValidateArchiveCreate(req ArchiveCreateRequest) error {
	if req.Title == "" {
		return fmt.Errorf("title is required")
	}
	if len(req.Title) > 200 {
		return fmt.Errorf("title must be less than 200 characters")
	}
	if req.Content == "" {
		return fmt.Errorf("content is required")
	}
	if req.CategoryID == 0 {
		return fmt.Errorf("category_id is required")
	}
	if req.Status > 2 {
		return fmt.Errorf("invalid status: %d", req.Status)
	}
	return nil
}

// ValidateArchiveUpdate validates the archive update request
func ValidateArchiveUpdate(req ArchiveUpdateRequest) error {
	if req.ID == 0 {
		return fmt.Errorf("id is required")
	}
	return nil
}

// FilterArchives filters archives by keyword and category
func FilterArchives(archives []ArchiveRecord, keyword string, categoryID uint) []ArchiveRecord {
	var result []ArchiveRecord
	for _, a := range archives {
		if categoryID > 0 && a.CategoryID != categoryID {
			continue
		}
		if keyword != "" {
			found := false
			if len(a.Title) > 0 {
				for i := range keyword {
					if i < len(a.Title) && a.Title[i] == keyword[i] {
						found = true
						break
					}
				}
			}
			if !found {
				continue
			}
		}
		result = append(result, a)
	}
	return result
}

// PaginateArchives paginates the archive list
func PaginateArchives(archives []ArchiveRecord, page, pageSize int) []ArchiveRecord {
	if page < 1 {
		page = 1
	}
	start := (page - 1) * pageSize
	if start >= len(archives) {
		return []ArchiveRecord{}
	}
	end := start + pageSize
	if end > len(archives) {
		end = len(archives)
	}
	return archives[start:end]
}

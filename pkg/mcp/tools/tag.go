package tools

import (
	"context"
	"encoding/json"
	"fmt"

	"github.com/modelcontextprotocol/go-sdk/mcp"
)

// TagProvider defines the interface for tag operations
type TagProvider interface {
	GetTag(id uint) (*TagRecord, error)
	ListTags(req TagListRequest) ([]TagRecord, error)
	CreateTag(req TagCreateRequest) (uint, error)
	UpdateTag(id uint, req TagUpdateRequest) error
	DeleteTag(id uint) error
}

// TagRecord represents a tag
type TagRecord struct {
	Id           uint   `json:"id"`
	Title        string `json:"title"`
	CategoryId   uint   `json:"category_id"`
	SeoTitle     string `json:"seo_title"`
	Keywords     string `json:"keywords"`
	UrlToken     string `json:"url_token"`
	Description  string `json:"description"`
	FirstLetter  string `json:"first_letter"`
	Template     string `json:"template"`
	Logo         string `json:"logo"`
	Status       uint   `json:"status"`
	Thumb        string `json:"thumb,omitempty"`
	ArchiveCount int    `json:"archive_count"`
}

// TagListRequest defines parameters for listing tags
type TagListRequest struct {
	CategoryId uint   `json:"category_id"`
	Status     *uint  `json:"status"`
	Keyword    string `json:"keyword"`
}

// TagCreateRequest defines parameters for creating a tag
type TagCreateRequest struct {
	Title       string `json:"title"`
	CategoryId  uint   `json:"category_id"`
	SeoTitle    string `json:"seo_title"`
	Keywords    string `json:"keywords"`
	UrlToken    string `json:"url_token"`
	Description string `json:"description"`
	Status      uint   `json:"status"`
	Template    string `json:"template"`
	Logo        string `json:"logo"`
}

// TagUpdateRequest defines parameters for updating a tag
type TagUpdateRequest struct {
	Title       string `json:"title"`
	CategoryId  uint   `json:"category_id"`
	SeoTitle    string `json:"seo_title"`
	Keywords    string `json:"keywords"`
	UrlToken    string `json:"url_token"`
	Description string `json:"description"`
	Status      uint   `json:"status"`
	Template    string `json:"template"`
	Logo        string `json:"logo"`
}

// TagTools holds tag tool definitions
type TagTools struct {
	Provider TagProvider
}

// NewTagTools creates tag tool definitions
func NewTagTools(provider TagProvider) TagTools {
	return TagTools{Provider: provider}
}

// GetAll returns all tag tool definitions with handlers
func (tt TagTools) GetAll() []ToolDef {
	return []ToolDef{
		{Tool: tagListTool(), Handler: tt.handleList},
		{Tool: tagDetailTool(), Handler: tt.handleDetail},
		{Tool: tagCreateTool(), Handler: tt.handleCreate},
		{Tool: tagUpdateTool(), Handler: tt.handleUpdate},
		{Tool: tagDeleteTool(), Handler: tt.handleDelete},
	}
}

// tagListTool returns the tag_list tool definition
func tagListTool() *mcp.Tool {
	return &mcp.Tool{
		Name:        "tag_list",
		Description: "获取标签列表，支持按分类、状态和关键词筛选",
		InputSchema: schemaObj(map[string]any{
			"category_id": prop("integer", "分类ID筛选", nil),
			"status":      prop("integer", "状态筛选", nil),
			"keyword":     prop("string", "关键词搜索", nil),
		}, nil),
	}
}

// tagDetailTool returns the tag_detail tool definition
func tagDetailTool() *mcp.Tool {
	return &mcp.Tool{
		Name:        "tag_detail",
		Description: "获取单个标签的详细信息",
		InputSchema: schemaObj(map[string]any{
			"tag_id": propRequired("integer", "标签ID"),
		}, []string{"tag_id"}),
	}
}

// tagCreateTool returns the tag_create tool definition
func tagCreateTool() *mcp.Tool {
	return &mcp.Tool{
		Name:        "tag_create",
		Description: "创建新标签",
		InputSchema: schemaObj(map[string]any{
			"title":       propRequired("string", "标签标题"),
			"category_id": prop("integer", "关联分类ID", nil),
			"seo_title":   prop("string", "SEO标题", nil),
			"keywords":    prop("string", "关键词，逗号分隔", nil),
			"url_token":   prop("string", "URL标识，不传则自动生成", nil),
			"description": prop("string", "标签描述", nil),
			"status":      prop("integer", "状态：1=启用，0=禁用", 1),
			"template":    prop("string", "模板文件", nil),
			"logo":        prop("string", "标签LOGO URL", nil),
		}, []string{"title"}),
	}
}

// tagUpdateTool returns the tag_update tool definition
func tagUpdateTool() *mcp.Tool {
	return &mcp.Tool{
		Name:        "tag_update",
		Description: "更新标签信息",
		InputSchema: schemaObj(map[string]any{
			"tag_id":        propRequired("integer", "标签ID"),
			"title":         prop("string", "标签标题", nil),
			"category_id":   prop("integer", "关联分类ID", nil),
			"seo_title":     prop("string", "SEO标题", nil),
			"keywords":      prop("string", "关键词，逗号分隔", nil),
			"url_token":     prop("string", "URL标识", nil),
			"description":   prop("string", "标签描述", nil),
			"status":        prop("integer", "状态：1=启用，0=禁用", nil),
			"template":      prop("string", "模板文件", nil),
			"logo":          prop("string", "标签LOGO URL", nil),
		}, []string{"tag_id"}),
	}
}

// tagDeleteTool returns the tag_delete tool definition
func tagDeleteTool() *mcp.Tool {
	return &mcp.Tool{
		Name:        "tag_delete",
		Description: "删除标签",
		InputSchema: schemaObj(map[string]any{
			"tag_id": propRequired("integer", "标签ID"),
		}, []string{"tag_id"}),
	}
}

// handleList handles tag_list calls
func (tt TagTools) handleList(ctx context.Context, req *mcp.CallToolRequest) (*mcp.CallToolResult, error) {
	args, err := extractArgs(req)
	if err != nil {
		return &mcp.CallToolResult{
			Content:           []mcp.Content{&mcp.TextContent{Text: fmt.Sprintf("invalid parameters: %v", err)}},
			IsError:           true,
			StructuredContent: nil,
		}, nil
	}

	listReq := TagListRequest{}

	if v, ok := args["category_id"].(float64); ok {
		listReq.CategoryId = uint(v)
	}
	if v, ok := args["status"].(float64); ok {
		s := uint(v)
		listReq.Status = &s
	}
	if v, ok := args["keyword"].(string); ok {
		listReq.Keyword = v
	}

	tags, err := tt.Provider.ListTags(listReq)
	if err != nil {
		return &mcp.CallToolResult{
			Content:           []mcp.Content{&mcp.TextContent{Text: fmt.Sprintf("failed to list tags: %v", err)}},
			IsError:           true,
			StructuredContent: nil,
		}, nil
	}

	content := &mcp.TextContent{Text: fmt.Sprintf("Found %d tags", len(tags))}
	structured, _ := json.Marshal(tags)

	return &mcp.CallToolResult{
		Content:           []mcp.Content{content},
		StructuredContent: structured,
	}, nil
}

// handleDetail handles tag_detail calls
func (tt TagTools) handleDetail(ctx context.Context, req *mcp.CallToolRequest) (*mcp.CallToolResult, error) {
	args, err := extractArgs(req)
	if err != nil {
		return &mcp.CallToolResult{
			Content:           []mcp.Content{&mcp.TextContent{Text: fmt.Sprintf("invalid parameters: %v", err)}},
			IsError:           true,
			StructuredContent: nil,
		}, nil
	}

	tagID, _ := args["tag_id"].(float64)
	id := uint(tagID)

	tag, err := tt.Provider.GetTag(id)
	if err != nil {
		return &mcp.CallToolResult{
			Content:           []mcp.Content{&mcp.TextContent{Text: fmt.Sprintf("failed to get tag: %v", err)}},
			IsError:           true,
			StructuredContent: nil,
		}, nil
	}

	content := &mcp.TextContent{Text: fmt.Sprintf("Tag: %s", tag.Title)}
	structured, _ := json.Marshal(tag)

	return &mcp.CallToolResult{
		Content:           []mcp.Content{content},
		StructuredContent: structured,
	}, nil
}

// handleCreate handles tag_create calls
func (tt TagTools) handleCreate(ctx context.Context, req *mcp.CallToolRequest) (*mcp.CallToolResult, error) {
	args, err := extractArgs(req)
	if err != nil {
		return &mcp.CallToolResult{
			Content:           []mcp.Content{&mcp.TextContent{Text: fmt.Sprintf("invalid parameters: %v", err)}},
			IsError:           true,
			StructuredContent: nil,
		}, nil
	}

	title, _ := args["title"].(string)
	createReq := TagCreateRequest{Title: title}

	if v, ok := args["category_id"].(float64); ok {
		createReq.CategoryId = uint(v)
	}
	if v, ok := args["seo_title"].(string); ok {
		createReq.SeoTitle = v
	}
	if v, ok := args["keywords"].(string); ok {
		createReq.Keywords = v
	}
	if v, ok := args["url_token"].(string); ok {
		createReq.UrlToken = v
	}
	if v, ok := args["description"].(string); ok {
		createReq.Description = v
	}
	if v, ok := args["status"].(float64); ok {
		createReq.Status = uint(v)
	}
	if v, ok := args["template"].(string); ok {
		createReq.Template = v
	}
	if v, ok := args["logo"].(string); ok {
		createReq.Logo = v
	}

	if err := ValidateTagCreate(createReq); err != nil {
		return &mcp.CallToolResult{
			Content:           []mcp.Content{&mcp.TextContent{Text: fmt.Sprintf("validation failed: %v", err)}},
			IsError:           true,
			StructuredContent: nil,
		}, nil
	}

	id, err := tt.Provider.CreateTag(createReq)
	if err != nil {
		return &mcp.CallToolResult{
			Content:           []mcp.Content{&mcp.TextContent{Text: fmt.Sprintf("failed to create tag: %v", err)}},
			IsError:           true,
			StructuredContent: nil,
		}, nil
	}

	content := &mcp.TextContent{Text: fmt.Sprintf("Tag created with ID: %d", id)}
	result := map[string]any{"id": id, "success": true}
	structured, _ := json.Marshal(result)

	return &mcp.CallToolResult{
		Content:           []mcp.Content{content},
		StructuredContent: structured,
	}, nil
}

// handleUpdate handles tag_update calls
func (tt TagTools) handleUpdate(ctx context.Context, req *mcp.CallToolRequest) (*mcp.CallToolResult, error) {
	args, err := extractArgs(req)
	if err != nil {
		return &mcp.CallToolResult{
			Content:           []mcp.Content{&mcp.TextContent{Text: fmt.Sprintf("invalid parameters: %v", err)}},
			IsError:           true,
			StructuredContent: nil,
		}, nil
	}

	tagID, _ := args["tag_id"].(float64)
	id := uint(tagID)
	updateReq := TagUpdateRequest{}

	if v, ok := args["title"].(string); ok {
		updateReq.Title = v
	}
	if v, ok := args["category_id"].(float64); ok {
		updateReq.CategoryId = uint(v)
	}
	if v, ok := args["seo_title"].(string); ok {
		updateReq.SeoTitle = v
	}
	if v, ok := args["keywords"].(string); ok {
		updateReq.Keywords = v
	}
	if v, ok := args["url_token"].(string); ok {
		updateReq.UrlToken = v
	}
	if v, ok := args["description"].(string); ok {
		updateReq.Description = v
	}
	if v, ok := args["status"].(float64); ok {
		updateReq.Status = uint(v)
	}
	if v, ok := args["template"].(string); ok {
		updateReq.Template = v
	}
	if v, ok := args["logo"].(string); ok {
		updateReq.Logo = v
	}

	if err := tt.Provider.UpdateTag(id, updateReq); err != nil {
		return &mcp.CallToolResult{
			Content:           []mcp.Content{&mcp.TextContent{Text: fmt.Sprintf("failed to update tag: %v", err)}},
			IsError:           true,
			StructuredContent: nil,
		}, nil
	}

	content := &mcp.TextContent{Text: "Tag updated successfully"}
	result := map[string]any{"success": true, "tag_id": id}
	structured, _ := json.Marshal(result)

	return &mcp.CallToolResult{
		Content:           []mcp.Content{content},
		StructuredContent: structured,
	}, nil
}

// handleDelete handles tag_delete calls
func (tt TagTools) handleDelete(ctx context.Context, req *mcp.CallToolRequest) (*mcp.CallToolResult, error) {
	args, err := extractArgs(req)
	if err != nil {
		return &mcp.CallToolResult{
			Content:           []mcp.Content{&mcp.TextContent{Text: fmt.Sprintf("invalid parameters: %v", err)}},
			IsError:           true,
			StructuredContent: nil,
		}, nil
	}

	tagID, _ := args["tag_id"].(float64)
	id := uint(tagID)

	if err := tt.Provider.DeleteTag(id); err != nil {
		return &mcp.CallToolResult{
			Content:           []mcp.Content{&mcp.TextContent{Text: fmt.Sprintf("failed to delete tag: %v", err)}},
			IsError:           true,
			StructuredContent: nil,
		}, nil
	}

	content := &mcp.TextContent{Text: fmt.Sprintf("Tag %d deleted successfully", id)}
	result := map[string]any{"success": true, "tag_id": id}
	structured, _ := json.Marshal(result)

	return &mcp.CallToolResult{
		Content:           []mcp.Content{content},
		StructuredContent: structured,
	}, nil
}

// ValidateTagCreate validates tag creation request
func ValidateTagCreate(req TagCreateRequest) error {
	if req.Title == "" {
		return fmt.Errorf("title is required")
	}
	if len(req.Title) > 255 {
		return fmt.Errorf("title must be less than 255 characters")
	}
	return nil
}

// ValidateTagUpdate validates tag update request
func ValidateTagUpdate(req TagUpdateRequest) error {
	if req.Title == "" {
		return fmt.Errorf("title is required")
	}
	return nil
}

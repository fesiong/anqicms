package tools

import (
	"context"
	"encoding/json"
	"fmt"

	"github.com/modelcontextprotocol/go-sdk/mcp"
)

// CategoryProvider defines the interface for category operations
type CategoryProvider interface {
	GetCategory(id uint) (*CategoryRecord, error)
	ListCategories(req CategoryListRequest) ([]CategoryRecord, error)
	CreateCategory(req CategoryCreateRequest) (uint, error)
	UpdateCategory(id uint, req CategoryUpdateRequest) error
	DeleteCategory(id uint) error
	GetCategoryTree() ([]CategoryRecord, error)
}

// CategoryRecord represents a category
type CategoryRecord struct {
	Id           uint   `json:"id"`
	Title        string `json:"title"`
	UrlToken     string `json:"url_token"`
	SeoTitle     string `json:"seo_title"`
	Keywords     string `json:"keywords"`
	Description  string `json:"description"`
	Pid          uint   `json:"pid"`
	ParentTitle  string `json:"parent_title,omitempty"`
	SortId       int    `json:"sort_id"`
	Status       uint   `json:"status"`
	Template     string `json:"template"`
	Logo         string `json:"logo"`
	Thumb        string `json:"thumb,omitempty"`
	Children     []uint `json:"children,omitempty"`
	ArchiveCount int    `json:"archive_count"`
}

// CategoryListRequest defines parameters for listing categories
type CategoryListRequest struct {
	Pid      uint  `json:"pid"`
	Status   *uint `json:"status"`
	WithTree bool  `json:"with_tree"`
}

// CategoryCreateRequest defines parameters for creating a category
type CategoryCreateRequest struct {
	Title       string `json:"title"`
	UrlToken    string `json:"url_token"`
	SeoTitle    string `json:"seo_title"`
	Keywords    string `json:"keywords"`
	Description string `json:"description"`
	Pid         uint   `json:"pid"`
	SortId      int    `json:"sort_id"`
	Status      uint   `json:"status"`
	Template    string `json:"template"`
	Logo        string `json:"logo"`
}

// CategoryUpdateRequest defines parameters for updating a category
type CategoryUpdateRequest struct {
	Title       string `json:"title"`
	UrlToken    string `json:"url_token"`
	SeoTitle    string `json:"seo_title"`
	Keywords    string `json:"keywords"`
	Description string `json:"description"`
	Pid         uint   `json:"pid"`
	SortId      int    `json:"sort_id"`
	Status      uint   `json:"status"`
	Template    string `json:"template"`
	Logo        string `json:"logo"`
}

// CategoryTools holds category tool definitions
type CategoryTools struct {
	Provider CategoryProvider
}

// NewCategoryTools creates category tool definitions
func NewCategoryTools(provider CategoryProvider) CategoryTools {
	return CategoryTools{Provider: provider}
}

// GetAll returns all category tool definitions with handlers
func (ct CategoryTools) GetAll() []ToolDef {
	return []ToolDef{
		{Tool: categoryListTool(), Handler: ct.handleList},
		{Tool: categoryDetailTool(), Handler: ct.handleDetail},
		{Tool: categoryCreateTool(), Handler: ct.handleCreate},
		{Tool: categoryUpdateTool(), Handler: ct.handleUpdate},
		{Tool: categoryDeleteTool(), Handler: ct.handleDelete},
	}
}

// categoryListTool returns the category_list tool definition
func categoryListTool() *mcp.Tool {
	return &mcp.Tool{
		Name:        "category_list",
		Description: "获取分类列表，支持按父级ID、状态筛选或获取树形结构",
		InputSchema: schemaObj(map[string]any{
			"pid":       prop("integer", "父级ID，0为顶级", 0),
			"status":    prop("integer", "状态筛选", nil),
			"with_tree": prop("boolean", "是否返回树形结构", false),
		}, nil),
	}
}

// categoryDetailTool returns the category_detail tool definition
func categoryDetailTool() *mcp.Tool {
	return &mcp.Tool{
		Name:        "category_detail",
		Description: "获取单个分类的详细信息",
		InputSchema: schemaObj(map[string]any{
			"category_id": propRequired("integer", "分类ID"),
		}, []string{"category_id"}),
	}
}

// categoryCreateTool returns the category_create tool definition
func categoryCreateTool() *mcp.Tool {
	return &mcp.Tool{
		Name:        "category_create",
		Description: "创建新分类",
		InputSchema: schemaObj(map[string]any{
			"title":       propRequired("string", "分类标题"),
			"url_token":   prop("string", "URL标识，不传则自动生成", nil),
			"seo_title":   prop("string", "SEO标题", nil),
			"keywords":    prop("string", "关键词，逗号分隔", nil),
			"description": prop("string", "分类描述", nil),
			"pid":         prop("integer", "父级ID，0为顶级", 0),
			"sort_id":     prop("integer", "排序号，越大越靠前", 0),
			"status":      prop("integer", "状态：1=启用，0=禁用", 1),
			"template":    prop("string", "模板文件", nil),
			"logo":        prop("string", "分类LOGO URL", nil),
		}, []string{"title"}),
	}
}

// categoryUpdateTool returns the category_update tool definition
func categoryUpdateTool() *mcp.Tool {
	return &mcp.Tool{
		Name:        "category_update",
		Description: "更新分类信息",
		InputSchema: schemaObj(map[string]any{
			"category_id": propRequired("integer", "分类ID"),
			"title":       prop("string", "分类标题", nil),
			"url_token":   prop("string", "URL标识", nil),
			"seo_title":   prop("string", "SEO标题", nil),
			"keywords":    prop("string", "关键词，逗号分隔", nil),
			"description": prop("string", "分类描述", nil),
			"pid":         prop("integer", "父级ID", nil),
			"sort_id":     prop("integer", "排序号", nil),
			"status":      prop("integer", "状态：1=启用，0=禁用", nil),
			"template":    prop("string", "模板文件", nil),
			"logo":        prop("string", "分类LOGO URL", nil),
		}, []string{"category_id"}),
	}
}

// categoryDeleteTool returns the category_delete tool definition
func categoryDeleteTool() *mcp.Tool {
	return &mcp.Tool{
		Name:        "category_delete",
		Description: "删除分类（需确保无子分类）",
		InputSchema: schemaObj(map[string]any{
			"category_id": propRequired("integer", "分类ID"),
		}, []string{"category_id"}),
	}
}

// handleList handles category_list calls
func (ct CategoryTools) handleList(ctx context.Context, req *mcp.CallToolRequest) (*mcp.CallToolResult, error) {
	args, err := extractArgs(req)
	if err != nil {
		return &mcp.CallToolResult{
			Content:           []mcp.Content{&mcp.TextContent{Text: fmt.Sprintf("invalid parameters: %v", err)}},
			IsError:           true,
			StructuredContent: nil,
		}, nil
	}

	var pid uint
	if v, ok := args["pid"].(float64); ok {
		pid = uint(v)
	}

	var status *uint
	if v, ok := args["status"].(float64); ok {
		s := uint(v)
		status = &s
	}

	withTree := false
	if v, ok := args["with_tree"].(bool); ok {
		withTree = v
	} else if v, ok := args["with_tree"].(float64); ok {
		withTree = v == 1
	}

	listReq := CategoryListRequest{
		Pid:      pid,
		Status:   status,
		WithTree: withTree,
	}

	categories, err := ct.Provider.ListCategories(listReq)
	if err != nil {
		return &mcp.CallToolResult{
			Content:           []mcp.Content{&mcp.TextContent{Text: fmt.Sprintf("failed to list categories: %v", err)}},
			IsError:           true,
			StructuredContent: nil,
		}, nil
	}

	content := &mcp.TextContent{Text: fmt.Sprintf("Found %d categories", len(categories))}
	structured, _ := json.Marshal(categories)

	return &mcp.CallToolResult{
		Content:           []mcp.Content{content},
		StructuredContent: structured,
	}, nil
}

// handleDetail handles category_detail calls
func (ct CategoryTools) handleDetail(ctx context.Context, req *mcp.CallToolRequest) (*mcp.CallToolResult, error) {
	args, err := extractArgs(req)
	if err != nil {
		return &mcp.CallToolResult{
			Content:           []mcp.Content{&mcp.TextContent{Text: fmt.Sprintf("invalid parameters: %v", err)}},
			IsError:           true,
			StructuredContent: nil,
		}, nil
	}

	categoryID, _ := args["category_id"].(float64)
	id := uint(categoryID)

	category, err := ct.Provider.GetCategory(id)
	if err != nil {
		return &mcp.CallToolResult{
			Content:           []mcp.Content{&mcp.TextContent{Text: fmt.Sprintf("failed to get category: %v", err)}},
			IsError:           true,
			StructuredContent: nil,
		}, nil
	}

	content := &mcp.TextContent{Text: fmt.Sprintf("Category: %s", category.Title)}
	structured, _ := json.Marshal(category)

	return &mcp.CallToolResult{
		Content:           []mcp.Content{content},
		StructuredContent: structured,
	}, nil
}

// handleCreate handles category_create calls
func (ct CategoryTools) handleCreate(ctx context.Context, req *mcp.CallToolRequest) (*mcp.CallToolResult, error) {
	args, err := extractArgs(req)
	if err != nil {
		return &mcp.CallToolResult{
			Content:           []mcp.Content{&mcp.TextContent{Text: fmt.Sprintf("invalid parameters: %v", err)}},
			IsError:           true,
			StructuredContent: nil,
		}, nil
	}

	title, _ := args["title"].(string)
	createReq := CategoryCreateRequest{Title: title}

	if v, ok := args["url_token"].(string); ok {
		createReq.UrlToken = v
	}
	if v, ok := args["seo_title"].(string); ok {
		createReq.SeoTitle = v
	}
	if v, ok := args["keywords"].(string); ok {
		createReq.Keywords = v
	}
	if v, ok := args["description"].(string); ok {
		createReq.Description = v
	}
	if v, ok := args["pid"].(float64); ok {
		createReq.Pid = uint(v)
	}
	if v, ok := args["sort_id"].(float64); ok {
		createReq.SortId = int(v)
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

	if err := ValidateCategoryCreate(createReq); err != nil {
		return &mcp.CallToolResult{
			Content:           []mcp.Content{&mcp.TextContent{Text: fmt.Sprintf("validation failed: %v", err)}},
			IsError:           true,
			StructuredContent: nil,
		}, nil
	}

	id, err := ct.Provider.CreateCategory(createReq)
	if err != nil {
		return &mcp.CallToolResult{
			Content:           []mcp.Content{&mcp.TextContent{Text: fmt.Sprintf("failed to create category: %v", err)}},
			IsError:           true,
			StructuredContent: nil,
		}, nil
	}

	content := &mcp.TextContent{Text: fmt.Sprintf("Category created with ID: %d", id)}
	result := map[string]any{"id": id, "success": true}
	structured, _ := json.Marshal(result)

	return &mcp.CallToolResult{
		Content:           []mcp.Content{content},
		StructuredContent: structured,
	}, nil
}

// handleUpdate handles category_update calls
func (ct CategoryTools) handleUpdate(ctx context.Context, req *mcp.CallToolRequest) (*mcp.CallToolResult, error) {
	args, err := extractArgs(req)
	if err != nil {
		return &mcp.CallToolResult{
			Content:           []mcp.Content{&mcp.TextContent{Text: fmt.Sprintf("invalid parameters: %v", err)}},
			IsError:           true,
			StructuredContent: nil,
		}, nil
	}

	categoryID, _ := args["category_id"].(float64)
	id := uint(categoryID)
	updateReq := CategoryUpdateRequest{}

	if v, ok := args["title"].(string); ok {
		updateReq.Title = v
	}
	if v, ok := args["url_token"].(string); ok {
		updateReq.UrlToken = v
	}
	if v, ok := args["seo_title"].(string); ok {
		updateReq.SeoTitle = v
	}
	if v, ok := args["keywords"].(string); ok {
		updateReq.Keywords = v
	}
	if v, ok := args["description"].(string); ok {
		updateReq.Description = v
	}
	if v, ok := args["pid"].(float64); ok {
		updateReq.Pid = uint(v)
	}
	if v, ok := args["sort_id"].(float64); ok {
		updateReq.SortId = int(v)
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

	if err := ct.Provider.UpdateCategory(id, updateReq); err != nil {
		return &mcp.CallToolResult{
			Content:           []mcp.Content{&mcp.TextContent{Text: fmt.Sprintf("failed to update category: %v", err)}},
			IsError:           true,
			StructuredContent: nil,
		}, nil
	}

	content := &mcp.TextContent{Text: "Category updated successfully"}
	result := map[string]any{"success": true, "category_id": id}
	structured, _ := json.Marshal(result)

	return &mcp.CallToolResult{
		Content:           []mcp.Content{content},
		StructuredContent: structured,
	}, nil
}

// handleDelete handles category_delete calls
func (ct CategoryTools) handleDelete(ctx context.Context, req *mcp.CallToolRequest) (*mcp.CallToolResult, error) {
	args, err := extractArgs(req)
	if err != nil {
		return &mcp.CallToolResult{
			Content:           []mcp.Content{&mcp.TextContent{Text: fmt.Sprintf("invalid parameters: %v", err)}},
			IsError:           true,
			StructuredContent: nil,
		}, nil
	}

	categoryID, _ := args["category_id"].(float64)
	id := uint(categoryID)

	if err := ct.Provider.DeleteCategory(id); err != nil {
		return &mcp.CallToolResult{
			Content:           []mcp.Content{&mcp.TextContent{Text: fmt.Sprintf("failed to delete category: %v", err)}},
			IsError:           true,
			StructuredContent: nil,
		}, nil
	}

	content := &mcp.TextContent{Text: fmt.Sprintf("Category %d deleted successfully", id)}
	result := map[string]any{"success": true, "category_id": id}
	structured, _ := json.Marshal(result)

	return &mcp.CallToolResult{
		Content:           []mcp.Content{content},
		StructuredContent: structured,
	}, nil
}

// ValidateCategoryCreate validates category creation request
func ValidateCategoryCreate(req CategoryCreateRequest) error {
	if req.Title == "" {
		return fmt.Errorf("title is required")
	}
	if len(req.Title) > 255 {
		return fmt.Errorf("title must be less than 255 characters")
	}
	return nil
}

// ValidateCategoryUpdate validates category update request
func ValidateCategoryUpdate(req CategoryUpdateRequest) error {
	if req.Title == "" {
		return fmt.Errorf("title is required")
	}
	return nil
}

// BuildCategoryTree builds a tree structure from flat category list
func BuildCategoryTree(cats []CategoryRecord) []CategoryRecord {
	if len(cats) == 0 {
		return cats
	}
	categoryMap := make(map[uint]*CategoryRecord)
	var roots []CategoryRecord

	for i := range cats {
		cats[i].Children = nil
		categoryMap[cats[i].Id] = &cats[i]
	}

	for i := range cats {
		cat := &cats[i]
		if cat.Pid == 0 {
			roots = append(roots, cats[i])
		} else if parent, ok := categoryMap[cat.Pid]; ok {
			parent.Children = append(parent.Children, cat.Id)
		}
	}

	return roots
}

package tools

import (
	"context"
	"encoding/json"
	"fmt"

	"github.com/modelcontextprotocol/go-sdk/mcp"
)

// AttachmentProvider defines the interface for attachment operations
type AttachmentProvider interface {
	GetAttachment(id uint) (*AttachmentRecord, error)
	ListAttachments(req AttachmentListRequest) (*AttachmentListResult, error)
	DeleteAttachment(id uint) error
	GetAttachmentURL(id uint) (string, error)
}

// AttachmentRecord represents an attachment
type AttachmentRecord struct {
	Id           uint   `json:"id"`
	CreatedTime  int64  `json:"created_time"`
	UpdatedTime  int64  `json:"updated_time"`
	UserId       uint   `json:"user_id"`
	FileName     string `json:"file_name"`
	FileLocation string `json:"file_location"`
	FileSize     int64  `json:"file_size"`
	FileMd5      string `json:"file_md5"`
	Width        int    `json:"width"`
	Height       int    `json:"height"`
	CategoryId   uint   `json:"category_id"`
	IsImage      int    `json:"is_image"`
	Status       uint   `json:"status"`
	Thumb        string `json:"thumb,omitempty"`
	Url          string `json:"url,omitempty"`
}

// AttachmentListRequest defines parameters for listing attachments
type AttachmentListRequest struct {
	Page       int   `json:"page"`
	PageSize   int   `json:"page_size"`
	CategoryId uint  `json:"category_id"`
	UserId     uint  `json:"user_id"`
	Status     *uint `json:"status"`
}

// AttachmentListResult is the paginated result
type AttachmentListResult struct {
	Total    int                 `json:"total"`
	Page     int                 `json:"page"`
	PageSize int                 `json:"page_size"`
	Items    []AttachmentRecord  `json:"items"`
}

// AttachmentTools holds attachment tool definitions
type AttachmentTools struct {
	Provider AttachmentProvider
}

// NewAttachmentTools creates attachment tool definitions
func NewAttachmentTools(provider AttachmentProvider) AttachmentTools {
	return AttachmentTools{Provider: provider}
}

// GetAll returns all attachment tool definitions with handlers
func (at AttachmentTools) GetAll() []ToolDef {
	return []ToolDef{
		{Tool: attachmentListTool(), Handler: at.handleList},
		{Tool: attachmentDeleteTool(), Handler: at.handleDelete},
		{Tool: attachmentURLTool(), Handler: at.handleURL},
	}
}

// attachmentListTool returns the attachment_list tool definition
func attachmentListTool() *mcp.Tool {
	return &mcp.Tool{
		Name:        "attachment_list",
		Description: "获取附件列表，支持分页和按分类/用户筛选",
		InputSchema: schemaObj(map[string]any{
			"page":        prop("integer", "页码", 1),
			"page_size":   prop("integer", "每页数量", 20),
			"category_id": prop("integer", "分类ID筛选", nil),
			"user_id":     prop("integer", "用户ID筛选", nil),
			"status":      prop("integer", "状态筛选", nil),
		}, nil),
	}
}

// attachmentDeleteTool returns the attachment_delete tool definition
func attachmentDeleteTool() *mcp.Tool {
	return &mcp.Tool{
		Name:        "attachment_delete",
		Description: "删除附件",
		InputSchema: schemaObj(map[string]any{
			"attachment_id": propRequired("integer", "附件ID"),
		}, []string{"attachment_id"}),
	}
}

// attachmentURLTool returns the attachment_url tool definition
func attachmentURLTool() *mcp.Tool {
	return &mcp.Tool{
		Name:        "attachment_url",
		Description: "获取附件的完整访问URL",
		InputSchema: schemaObj(map[string]any{
			"attachment_id": propRequired("integer", "附件ID"),
		}, []string{"attachment_id"}),
	}
}

// handleList handles attachment_list calls
func (at AttachmentTools) handleList(ctx context.Context, req *mcp.CallToolRequest) (*mcp.CallToolResult, error) {
	args, err := extractArgs(req)
	if err != nil {
		return &mcp.CallToolResult{
			Content: []mcp.Content{&mcp.TextContent{Text: fmt.Sprintf("invalid parameters: %v", err)}},
			IsError: true,
		}, nil
	}

	input := AttachmentListRequest{}
	if v, ok := args["page"].(float64); ok {
		input.Page = int(v)
	}
	if v, ok := args["page_size"].(float64); ok {
		input.PageSize = int(v)
	}
	if v, ok := args["category_id"].(float64); ok {
		input.CategoryId = uint(v)
	}
	if v, ok := args["user_id"].(float64); ok {
		input.UserId = uint(v)
	}
	if v, ok := args["status"].(float64); ok {
		status := uint(v)
		input.Status = &status
	}

	if input.Page < 1 {
		input.Page = 1
	}
	if input.PageSize < 1 || input.PageSize > 100 {
		input.PageSize = 20
	}

	result, err := at.Provider.ListAttachments(input)
	if err != nil {
		res := &mcp.CallToolResult{
			Content: []mcp.Content{&mcp.TextContent{Text: fmt.Sprintf("failed to list attachments: %v", err)}},
			IsError: true,
		}
		return res, nil
	}

	content := &mcp.TextContent{Text: fmt.Sprintf("Found %d attachments", result.Total)}
	structured, _ := json.Marshal(result)

	return &mcp.CallToolResult{
		Content:           []mcp.Content{content},
		StructuredContent: structured,
	}, nil
}

// handleDelete handles attachment_delete calls
func (at AttachmentTools) handleDelete(ctx context.Context, req *mcp.CallToolRequest) (*mcp.CallToolResult, error) {
	args, err := extractArgs(req)
	if err != nil {
		return &mcp.CallToolResult{
			Content: []mcp.Content{&mcp.TextContent{Text: fmt.Sprintf("invalid parameters: %v", err)}},
			IsError: true,
		}, nil
	}

	attachmentID, _ := args["attachment_id"].(float64)
	id := uint(attachmentID)

	if err := at.Provider.DeleteAttachment(id); err != nil {
		res := &mcp.CallToolResult{
			Content: []mcp.Content{&mcp.TextContent{Text: fmt.Sprintf("failed to delete attachment: %v", err)}},
			IsError: true,
		}
		return res, nil
	}

	content := &mcp.TextContent{Text: fmt.Sprintf("Attachment %d deleted successfully", id)}
	result := map[string]any{"success": true, "attachment_id": id}
	structured, _ := json.Marshal(result)

	return &mcp.CallToolResult{
		Content:           []mcp.Content{content},
		StructuredContent: structured,
	}, nil
}

// handleURL handles attachment_url calls
func (at AttachmentTools) handleURL(ctx context.Context, req *mcp.CallToolRequest) (*mcp.CallToolResult, error) {
	args, err := extractArgs(req)
	if err != nil {
		return &mcp.CallToolResult{
			Content: []mcp.Content{&mcp.TextContent{Text: fmt.Sprintf("invalid parameters: %v", err)}},
			IsError: true,
		}, nil
	}

	attachmentID, _ := args["attachment_id"].(float64)
	id := uint(attachmentID)

	url, err := at.Provider.GetAttachmentURL(id)
	if err != nil {
		res := &mcp.CallToolResult{
			Content: []mcp.Content{&mcp.TextContent{Text: fmt.Sprintf("failed to get attachment URL: %v", err)}},
			IsError: true,
		}
		return res, nil
	}

	result := map[string]any{
		"attachment_id": id,
		"url":           url,
		"success":       true,
	}
	structured, _ := json.Marshal(result)

	content := &mcp.TextContent{Text: url}
	return &mcp.CallToolResult{
		Content:           []mcp.Content{content},
		StructuredContent: structured,
	}, nil
}

// ValidateAttachmentName validates file name
func ValidateAttachmentName(name string) error {
	if name == "" {
		return fmt.Errorf("file name is required")
	}
	if len(name) > 250 {
		return fmt.Errorf("file name must be less than 250 characters")
	}
	return nil
}

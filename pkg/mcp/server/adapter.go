package server

import (
	"context"
	"encoding/json"
	"fmt"

	"github.com/cloudwego/eino/schema"
	"github.com/modelcontextprotocol/go-sdk/mcp"
)

// EinoToolInfoToMCPTool 将 Eino 的 *schema.ToolInfo 转为 mcp-go 的 *mcp.Tool。
//
// 现有 95 个 Eino 工具全部使用 NewParamsOneOfByParams 形式。
// Eino 的 ParamsOneOf.ToJSONSchema() 返回 eino-contrib/jsonschema 的 Schema，
// 通过 JSON 序列化/反序列化转为通用 map[string]any，赋给 mcp.Tool.InputSchema (any 类型)。
func EinoToolInfoToMCPTool(ti *schema.ToolInfo) (*mcp.Tool, error) {
	if ti == nil {
		return nil, fmt.Errorf("tool info is nil")
	}

	tool := &mcp.Tool{
		Name:        ti.Name,
		Description: ti.Desc,
	}

	if ti.ParamsOneOf == nil {
		// 无参数工具：返回空对象 schema
		tool.InputSchema = map[string]any{
			"type":       "object",
			"properties": map[string]any{},
		}
		return tool, nil
	}

	// 用 Eino 的 ToJSONSchema() 获取 JSON Schema
	einoSchema, err := ti.ParamsOneOf.ToJSONSchema()
	if err != nil {
		return nil, fmt.Errorf("tool %s: failed to convert ParamsOneOf to JSON schema: %w", ti.Name, err)
	}
	if einoSchema == nil {
		tool.InputSchema = map[string]any{
			"type":       "object",
			"properties": map[string]any{},
		}
		return tool, nil
	}

	// 序列化 einoSchema 为 JSON，再反序列化为 map[string]any
	raw, err := json.Marshal(einoSchema)
	if err != nil {
		return nil, fmt.Errorf("tool %s: failed to marshal JSON schema: %w", ti.Name, err)
	}

	var inputSchema map[string]any
	if err := json.Unmarshal(raw, &inputSchema); err != nil {
		return nil, fmt.Errorf("tool %s: failed to unmarshal JSON schema: %w", ti.Name, err)
	}

	// 确保 type 为 object
	if inputSchema["type"] == nil {
		inputSchema["type"] = "object"
	}

	tool.InputSchema = inputSchema
	return tool, nil
}

// toolHandler 是 Eino 工具的 handler 签名（与 provider/aiTools.go 一致）。
type toolHandler func(ctx context.Context, argsJSON string) (string, error)

// AdaptHandler 将现有 Eino toolHandler 适配为 mcp-go 的 ToolHandler。
// 现有 handler 接收 JSON 字符串、返回 (string, error)；
// mcp-go ToolHandler 接收 *mcp.CallToolRequest、返回 (*mcp.CallToolResult, error)。
//
// 参数类型使用通用 func 签名而非 toolHandler 别名，
// 以兼容 provider 包内定义的 toolHandler（Go 不支持结构化类型等价）。
func AdaptHandler(h func(context.Context, string) (string, error)) mcp.ToolHandler {
	return func(ctx context.Context, req *mcp.CallToolRequest) (*mcp.CallToolResult, error) {
		var argsJSON []byte
		if req.Params.Arguments != nil {
			var err error
			argsJSON, err = json.Marshal(req.Params.Arguments)
			if err != nil {
				return &mcp.CallToolResult{
					Content: []mcp.Content{&mcp.TextContent{Text: fmt.Sprintf("failed to marshal arguments: %v", err)}},
					IsError: true,
				}, nil
			}
		} else {
			argsJSON = []byte("{}")
		}

		result, err := h(ctx, string(argsJSON))
		if err != nil {
			return &mcp.CallToolResult{
				Content: []mcp.Content{&mcp.TextContent{Text: err.Error()}},
				IsError: true,
			}, nil
		}

		return &mcp.CallToolResult{
			Content: []mcp.Content{&mcp.TextContent{Text: result}},
		}, nil
	}
}

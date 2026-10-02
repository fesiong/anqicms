package intent

import (
	"context"
	"encoding/json"
	"fmt"
	"reflect"
	"strings"
	"time"

	"github.com/modelcontextprotocol/go-sdk/mcp"
)

// mcp.go 是意图内核的 MCP 通道适配器：把声明式意图注册为 mcp.Tool，
// 并把中性 Execute 结果包装为 *mcp.CallToolResult。MCP 通道专用。

// structuredObject 保证写入 StructuredContent 的值一定能序列化为 JSON 对象。
//
// vendored SDK 对 CallToolResult.StructuredContent 的契约是：
// "It must marshal to a JSON object."。裸数组/标量违反该契约，
// 严格校验的客户端会直接报 "expected record, received array" 之类的错误。
// 这里在适配器边界统一兜底：非对象一律包进 {"result": ...}，
// 使任何意图（包括未来新增的）都无法触发该类协议错误。
func structuredObject(data any) any {
	if data == nil {
		return nil
	}
	v := reflect.ValueOf(data)
	for v.Kind() == reflect.Ptr || v.Kind() == reflect.Interface {
		if v.IsNil() {
			return nil
		}
		v = v.Elem()
	}
	switch v.Kind() {
	case reflect.Map, reflect.Struct:
		return data
	default:
		return map[string]any{"result": data}
	}
}

// RegisterAll 把所有命中的意图与 meta 意图注册进给定 mcp.Server。
func (k *Kernel) RegisterAll(server *mcp.Server) {
	k.mu.Lock()
	k.reg = nil
	k.mu.Unlock()
	k.registerDomainIntents(server)
	k.registerMeta(server)
}

func (k *Kernel) registerDomainIntents(server *mcp.Server) {
	for _, name := range k.order {
		spec := k.intents[name]
		if !k.allowed(spec) {
			continue
		}
		if k.scope != nil && !k.scope[spec.Domain] {
			continue
		}
		full := k.cfg.ToolListMode != "summary"
		server.AddTool(k.buildTool(spec, full), k.makeHandler(name, spec))
		k.track(name)
	}
}

// Reregister 两阶段：移除当前注册的工具后按新 scope 重注册（供 mcp_set_scope 调用）。
func (k *Kernel) Reregister(server *mcp.Server) {
	names := append([]string{}, k.reg...)
	if len(names) > 0 {
		server.RemoveTools(names...)
	}
	k.mu.Lock()
	k.reg = nil
	k.mu.Unlock()
	k.registerDomainIntents(server)
	k.registerMeta(server)
}

func (k *Kernel) makeHandler(name string, spec *IntentSpec) mcp.ToolHandler {
	return func(ctx context.Context, req *mcp.CallToolRequest) (*mcp.CallToolResult, error) {
		start := time.Now()
		argsJSON := marshalArgs(req)
		res, cerr := k.Execute(ctx, name, argsJSON)
		if k.audit != nil {
			k.audit(ctx, name, string(spec.Risk), argsJSON, cerr, start)
		}
		if cerr != nil {
			return &mcp.CallToolResult{
				Content: []mcp.Content{&mcp.TextContent{Text: cerr.Error()}},
				IsError: true,
			}, cerr
		}
		out := &mcp.CallToolResult{
			Content: []mcp.Content{&mcp.TextContent{Text: res.Text}},
		}
		if res.Data != nil {
			out.StructuredContent = structuredObject(res.Data)
		}
		return out, nil
	}
}

func (k *Kernel) buildTool(spec *IntentSpec, full bool) *mcp.Tool {
	desc := spec.Desc
	var schema any
	if full {
		schema = BuildInputSchema(spec.Params, spec.Required)
	} else {
		schema = map[string]any{"type": "object", "properties": map[string]any{}}
		desc = fmt.Sprintf("[%s / 风险:%s] %s", DomainLabel(spec.Domain), spec.Risk, spec.Desc)
	}
	t := &mcp.Tool{
		Name:        spec.Name,
		Title:       spec.Title,
		Description: desc,
		InputSchema: schema,
	}
	ro := spec.Risk == RiskRead
	dh := spec.Risk == RiskDestructive || spec.Risk == RiskSystem
	t.Annotations = &mcp.ToolAnnotations{
		ReadOnlyHint:    ro,
		DestructiveHint: boolPtr(dh),
		IdempotentHint:  spec.Idempotent,
		OpenWorldHint:   boolPtr(spec.Domain == DomainBuiltin && (spec.Name == "web" || spec.Name == "shell_exec")),
	}
	if full && spec.Output != nil {
		if os := BuildOutputSchema(spec.Output); os != nil {
			t.OutputSchema = os
		}
	}
	return t
}

// registerMeta 注册两个 meta 意图（不在 catalog 中，始终可用）。
func (k *Kernel) registerMeta(server *mcp.Server) {
	// mcp_set_scope：两阶段能力域选择
	scopeTool := &mcp.Tool{
		Name:        "mcp_set_scope",
		Title:       "设置能力域范围",
		Description: "两阶段 tools/list：声明要使用的意图所属能力域（content/seo/structure/media/user/system/agent/builtin 等）。声明后 tools/list 仅返回这些域的完整 schema；传空数组恢复全部。未知意图始终可用 mcp_list_intents 发现。",
		InputSchema: map[string]any{
			"type": "object",
			"properties": map[string]any{
				"domains": map[string]any{"type": "array", "items": map[string]any{"type": "string"}, "description": "能力域列表，如 [\"content\",\"seo\"]；空数组恢复全部"},
			},
			"required": []string{"domains"},
		},
	}
	server.AddTool(scopeTool, func(ctx context.Context, req *mcp.CallToolRequest) (*mcp.CallToolResult, error) {
		start := time.Now()
		argsJSON := marshalArgs(req)
		var a struct {
			Domains []string `json:"domains"`
		}
		_ = json.Unmarshal([]byte(argsJSON), &a)
		k.SetScope(a.Domains)
		k.Reregister(server)
		msg := "已应用能力域范围: 全部"
		if len(a.Domains) > 0 {
			msg = "已应用能力域范围: " + strings.Join(a.Domains, ", ")
		}
		if k.audit != nil {
			k.audit(ctx, "mcp_set_scope", "system", argsJSON, nil, start)
		}
		return &mcp.CallToolResult{Content: []mcp.Content{&mcp.TextContent{Text: msg}}}, nil
	})
	k.track("mcp_set_scope")

	// mcp_list_intents：发现全部意图（不受 scope 限制）
	//
	// 注意：返回体必须是 JSON 对象（StructuredContent 契约），
	// 因此把意图数组包在 {"total":N,"intents":[...]} 里，并声明匹配的 outputSchema。
	listTool := &mcp.Tool{
		Name:        "mcp_list_intents",
		Title:       "列出全部意图",
		Description: "返回当前站点所有意图导向工作流工具的摘要（name/title/domain/risk/desc），用于在不加载完整 schema 的情况下发现能力。",
		InputSchema: map[string]any{"type": "object", "properties": map[string]any{}},
		OutputSchema: map[string]any{
			"type":     "object",
			"required": []string{"total", "intents"},
			"properties": map[string]any{
				"total": map[string]any{"type": "integer", "description": "意图总数"},
				"intents": map[string]any{
					"type":        "array",
					"description": "意图摘要列表",
					"items": map[string]any{
						"type": "object",
						"properties": map[string]any{
							"name":   map[string]any{"type": "string", "description": "意图名称，用于实际调用"},
							"title":  map[string]any{"type": "string"},
							"domain": map[string]any{"type": "string"},
							"risk":   map[string]any{"type": "string"},
							"desc":   map[string]any{"type": "string"},
						},
					},
				},
			},
		},
	}
	server.AddTool(listTool, func(ctx context.Context, req *mcp.CallToolRequest) (*mcp.CallToolResult, error) {
		payload := k.listIntentsPayload()
		return &mcp.CallToolResult{
			Content:           []mcp.Content{&mcp.TextContent{Text: textifyIntents(payload)}},
			StructuredContent: payload,
		}, nil
	})
	k.track("mcp_list_intents")
}

// textifyIntents 把意图清单渲染成纯文本明细。
//
// 存在意义是**优雅降级**：部分 MCP 宿主对 structuredContent 校验严格，
// 一旦失败就只剩一句「共 N 个意图」，模型拿不到任何可下手的线索。
// 这里把 name/title/domain/risk/desc 一并写入文本，即使结构化通道不可用，
// 调用方仍能据此挑出目标意图并直接发起调用。
func textifyIntents(payload map[string]any) string {
	intents, _ := payload["intents"].([]map[string]any)
	var sb strings.Builder
	sb.WriteString(fmt.Sprintf("共 %d 个意图：\n", len(intents)))
	for _, it := range intents {
		sb.WriteString(fmt.Sprintf("- %v（%v）[%v/%v] %v\n",
			it["name"], it["title"], it["domain"], it["risk"], it["desc"]))
	}
	return sb.String()
}

// listIntentsPayload 构建意图发现工具的返回体：{"total":N,"intents":[...]}。
// 抽成独立方法既便于单测，也提醒调用方——此处不能返回裸数组。
func (k *Kernel) listIntentsPayload() map[string]any {
	list := make([]map[string]any, 0, len(k.order))
	for _, n := range k.order {
		list = append(list, Summary(k.intents[n]))
	}
	return map[string]any{"total": len(list), "intents": list}
}

func marshalArgs(req *mcp.CallToolRequest) string {
	if req == nil || req.Params == nil || req.Params.Arguments == nil {
		return "{}"
	}
	b, err := json.Marshal(req.Params.Arguments)
	if err != nil {
		return "{}"
	}
	return string(b)
}

// Dispatch 兼容包装：执行意图并包装为标准 MCP 结果，供 mcp 适配器内部与旧测试使用。
func (k *Kernel) Dispatch(ctx context.Context, name, argsJSON string) (*mcp.CallToolResult, error) {
	res, cerr := k.Execute(ctx, name, argsJSON)
	if cerr != nil {
		return &mcp.CallToolResult{
			Content: []mcp.Content{&mcp.TextContent{Text: cerr.Error()}},
			IsError: true,
		}, cerr
	}
	out := &mcp.CallToolResult{
		Content: []mcp.Content{&mcp.TextContent{Text: res.Text}},
	}
	if res.Data != nil {
		out.StructuredContent = structuredObject(res.Data)
	}
	return out, nil
}

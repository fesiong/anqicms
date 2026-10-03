package intent

import (
	"context"
	"encoding/json"
	"fmt"
	"reflect"
	"strconv"
	"strings"
	"time"

	"github.com/modelcontextprotocol/go-sdk/mcp"
)

// mcp.go 是意图内核的 MCP 通道适配器：把声明式意图注册为 mcp.Tool，
// 并把中性 Execute 结果包装为 *mcp.CallToolResult。MCP 通道专用。

// structuredObject 保证写入 StructuredContent 的值一定能序列化为 JSON 对象，
// 且形状对 AI 是统一的。
//
// vendored SDK 对 CallToolResult.StructuredContent 的契约是：
// "It must marshal to a JSON object."。裸数组/标量违反该契约，
// 严格校验的客户端会直接报 "expected record, received array" 之类的错误。
//
// 在此之上还有一层形状统一（2026-10-03）。此前各 Compose 往 Result.Data 里
// 塞的形状五花八门：
//
//	{"result": [...]}          —— 列表类（switchCompose 拿到裸数组）
//	{模板字段直接铺开}          —— 端点已返回信封时被二次解析
//	{code,msg,ok,status}      —— 端点信封原样透传
//	{data: …}                  —— 自定义 Compose 回填
//
// AI 每换一个意图就得重新猜一次形状，`switchCompose` 那套「自动提取 data」
// 的约定也失效（因为有的走 result、有的走 data）。这里统一成标准信封：
//
//	{code, msg, ok, status, data}
//
// data 保持原样（对象或数组均可），业务判断一律看 ok/code，data 只管载荷。
// 已经是标准信封的（带 ok 键）原样透传，避免二次包裹。
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
	// 已经是标准信封（含 ok 键）则原样透传。
	if m, ok := v.Interface().(map[string]any); ok {
		if _, hasOK := m["ok"]; hasOK {
			return m
		}
		return envelopeOf(m)
	}
	switch v.Kind() {
	case reflect.Map:
		return envelopeOf(v.Interface())
	case reflect.Struct:
		return envelopeOf(v.Interface())
	default:
		// 裸数组/标量：包成信封，载荷放 data。
		return map[string]any{
			"code": 0, "msg": "", "ok": true, "status": 200, "data": data,
		}
	}
}

// envelopeOf 把业务载荷包进标准信封。
func envelopeOf(payload any) map[string]any {
	return map[string]any{
		"code":   0,
		"msg":    "",
		"ok":     true,
		"status": 200,
		"data":   payload,
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
		server.AddTool(k.buildTool(spec, full), k.makeHandler(name, spec, server))
		k.track(name)
	}
}

// Reregister 两阶段：移除当前注册的工具后按新 scope 重注册（供 mcp_set_scope 调用）。
//
// ⚠️ scope 是**进程级全局**的，不是会话级——Reregister 摘除/重注册的是
// 共享 mcp.Server 的全局工具表。实测（2026-10-03）：一个客户端调 set_scope 后，
// **所有并发客户端**的 tools/list 都被收窄，且新会话也恢复不了（须重启服务）。
// MCP 服务端是 HTTP 无状态的，SDK 的 listTools 是私有方法、不支持按连接过滤，
// 在本层做不到会话隔离。
//
// 之所以可以接受：scope 只影响 tools/list 的**视图裁剪**（省 token），
// 不影响可调用性——被裁掉的工具仍可按名字直接调用（tools/call 走 makeHandler，
// 不校验 scope）。安全性由 allowed()（风险 + 暴露门禁）负责，与 scope 无关。
// 但视图被全局收窄仍是实质危害：其他客户端会发现自己的工具「凭空消失」。
//
// 缓解措施：记录最近一次 set_scope 的时间（scopeTouched），
// 供 RestoreScopeIfStale 做自动恢复——长期无人再设置时自动还原为全部工具，
// 避免一次临时设置永久影响所有人。恢复时机见 RestoreScopeIfStale 的说明。
func (k *Kernel) Reregister(server *mcp.Server) {
	names := append([]string{}, k.reg...)
	if len(names) > 0 {
		server.RemoveTools(names...)
	}
	k.mu.Lock()
	k.reg = nil
	k.scopeTouched = time.Now()
	k.mu.Unlock()
	k.registerDomainIntents(server)
	k.registerMeta(server)
}

func (k *Kernel) makeHandler(name string, spec *IntentSpec, server *mcp.Server) mcp.ToolHandler {
	return func(ctx context.Context, req *mcp.CallToolRequest) (*mcp.CallToolResult, error) {
		// 每次工具调用前检查 scope 是否已空闲超时，过期则自动恢复全部工具。
		// scope 是全局的（一次设置影响所有客户端且需重启才能恢复，见 Reregister），
		// 长期滞留会让他人的工具「凭空消失」。放在请求路径上恢复，
		// 既不需要后台定时器，恢复时机也天然对齐「有人在用」。
		if k.RestoreScopeIfStale() {
			k.Reregister(server)
		}
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
		out := &mcp.CallToolResult{}
		if res.Data != nil {
			// Text 与 StructuredContent 必须同源。
			//
			// 背景（2026-10-03）：res.Text 是端点原始响应，res.Data 是 Compose
			// 加工后的结构，两者形状可能不同。实测 content_article list：
			//   - StructuredContent.data = {list,total,page,page_size,count}
			//   - Text 里的 data          = 裸数组，total 漂在信封顶层
			// 同一份结果、两个通道形状不一致，AI 走 text 回退通道时
			// 就拿不到分页信息（且会以为 total 是顶层字段）。
			// 故有 Data 时以 Data 为准，序列化后同时供两个通道使用。
			obj := structuredObject(res.Data)
			out.StructuredContent = obj
			if b, err := json.Marshal(obj); err == nil {
				out.Content = []mcp.Content{&mcp.TextContent{Text: string(b)}}
			} else {
				// 序列化失败不该让整次调用失败，退回原始文本。
				out.Content = []mcp.Content{&mcp.TextContent{Text: res.Text}}
			}
		} else {
			out.Content = []mcp.Content{&mcp.TextContent{Text: res.Text}}
		}
		// 端点业务失败时 Compose 往往**不**返回 Go error（api_invoke 把 5xx /
		// ok=false 都表达成"成功返回的 JSON"），于是 cerr 为 nil、IsError 保持 false。
		//
		// 后果（2026-10-03 实测）：system_multilang action=cache_delete 端点回
		// status=500，响应体里 ok:false，但 MCP 协议层 isError 是空 —— 严格
		// 依赖 isError 的客户端会把它当**成功**处理。isError 在 SDK 里是
		// `json:"isError,omitempty"`，不显式置 true 就等于没告知。
		//
		// 这里补一道统一判定：只要原始响应里带着端点失败信号就标 isError。
		// content_article 系列的 Compose 已在内部把失败转成 Go error（早于本处），
		// 因此不会重复标注。
		if !out.IsError && res.Text != "" {
			if msg := endpointFailure(res.Text); msg != "" {
				out.IsError = true
			}
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
		// 字符串形态也接受：模型很容易把 ["content","seo"] 写成 "content,seo"。
		// 但不能静默 —— 传错类型却回「已应用：全部」会让调用方以为设置成功，
		// 实际 scope 没变（实测踩过：传 "content" 得到「全部」）。
		var raw struct {
			Domains json.RawMessage `json:"domains"`
		}
		_ = json.Unmarshal([]byte(argsJSON), &raw)
		domains, err := parseScopeDomains(raw.Domains)
		if err != nil {
			return &mcp.CallToolResult{
				Content: []mcp.Content{&mcp.TextContent{Text: "能力域参数无效：" + err.Error() +
					"。正确写法：domains 为字符串数组，如 [\"content\",\"seo\"]；传空数组恢复全部"}},
				IsError: true,
			}, nil
		}
		k.SetScope(domains)
		k.Reregister(server)
		msg := "已应用能力域范围: 全部"
		if len(domains) > 0 {
			msg = "已应用能力域范围: " + strings.Join(domains, ", ")
			// 必须说清作用域是全局的：它会影响**所有**客户端的 tools/list
			// （共享同一个 server 实例），且不限于当前会话。调用方（尤其是 AI）
			// 若不知道这点，会误以为「我设了范围就只影响我」——
			// 实测踩过：设窄之后新会话也恢复不了，必须重启服务。
			msg += "。注意：此设置对**所有** MCP 客户端生效（进程级），" +
				"被隐藏的工具仍可按名字直接调用；" +
				strconv.Itoa(int(ScopeStaleAfter/time.Minute)) + " 分钟无新设置后会自动恢复全部。"
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

// parseScopeDomains 解析 mcp_set_scope 的 domains 参数。
//
// 存在的理由：schema 声明的是字符串数组，但模型经常写成单个字符串或逗号分隔串。
// 早先的做法是 json.Unmarshal 失败即忽略，于是传 "content" 会静默回退成
// 「已应用能力域范围: 全部」—— 调用方以为设置生效了，实际 scope 一点没变。
// 现在两种形态都接受，形态彻底无法识别时报错。
func parseScopeDomains(raw json.RawMessage) ([]string, error) {
	if len(raw) == 0 || string(raw) == "null" {
		return nil, nil
	}
	// 标准形态：["content","seo"]
	var arr []string
	if err := json.Unmarshal(raw, &arr); err == nil {
		return arr, nil
	}
	// 宽松形态："content" 或 "content,seo"
	var one string
	if err := json.Unmarshal(raw, &one); err == nil {
		one = strings.TrimSpace(one)
		if one == "" {
			return nil, nil
		}
		var out []string
		for _, p := range strings.Split(one, ",") {
			if p = strings.TrimSpace(p); p != "" {
				out = append(out, p)
			}
		}
		return out, nil
	}
	return nil, fmt.Errorf("收到 %s，既不是字符串数组也不是单个字符串", string(raw))
}

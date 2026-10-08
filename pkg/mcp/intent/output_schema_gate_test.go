package intent

import (
	"context"
	"encoding/json"
	"errors"
	"strings"
	"testing"

	"github.com/modelcontextprotocol/go-sdk/mcp"
)

// 端点信封的真实形状：structuredContent 顶层恒为它，业务载荷在 data 里。
// 这里写死类型而不是从代码推导，正因为要防的是「声明与真实不符」。
var envelopeTypes = map[string]string{
	"code":   "integer",
	"msg":    "string",
	"ok":     "boolean",
	"status": "integer",
	"data":   "object",
}

// TestDeclaredOutputSchemaMatchesEnvelope 锁住一条踩过的契约：
// spec.Output 推导出的 outputSchema 会由宿主用来校验 structuredContent，
// 类型对不上时整次调用被判非法——服务端审计记 ok=1，客户端只看到
// "MCP tool invocation did not complete"（2026-10-08 实测 content_article：
// ArticleOut.Status 声明 string，信封里 status 恒为 HTTP 状态码整数 200）。
func TestDeclaredOutputSchemaMatchesEnvelope(t *testing.T) {
	for i := range IntentCatalog {
		spec := IntentCatalog[i]
		if spec.Output == nil {
			continue
		}
		schema, ok := BuildOutputSchema(spec.Output).(map[string]any)
		if !ok {
			t.Errorf("%s: Output 未能推导出 object schema", spec.Name)
			continue
		}
		props, _ := schema["properties"].(map[string]any)
		for name, want := range envelopeTypes {
			p, declared := props[name]
			if !declared {
				continue
			}
			pm, _ := p.(map[string]any)
			if got, _ := pm["type"].(string); got != want {
				t.Errorf("%s: outputSchema 声明 %s 为 %q，而 structuredContent 信封里它是 %q —— "+
					"严格客户端会据此丢弃整个结果", spec.Name, name, got, want)
			}
		}
	}
}

// TestMultiActionIntentsDeclareNoOutputSchema 多 action 意图的输出形状随 action 而变
// （list 是分页对象、get 是单文档、save 是回执），单一静态 schema 无法覆盖，
// 声明了就只能保证违约。这里锁定「不声明」这个决定。
func TestMultiActionIntentsDeclareNoOutputSchema(t *testing.T) {
	k := NewKernel(Config{}, nil, nil)
	for _, name := range []string{"content_article", "content_manage", "seo", "media"} {
		spec, ok := k.intents[name]
		if !ok {
			t.Fatalf("意图 %s 不在内核里", name)
		}
		if spec.Output != nil {
			t.Errorf("%s: 多 action 意图不应声明 Output，实际=%T", name, spec.Output)
		}
		if tool := k.buildTool(spec, true); tool.OutputSchema != nil {
			t.Errorf("%s: tools/list 仍在对外广告 outputSchema=%v，会招来宿主端校验", name, tool.OutputSchema)
		}
	}
}

// TestGoErrorStaysInToolResult 内核失败的中文理由必须留在 tool result 里。
//
// 走 JSON-RPC 协议错误的话 CallToolResult 会被 SDK 整个丢弃，宿主只剩一个
// "-32603 / did not complete"，模型据此无法调整参数（2026-10-08）。
func TestGoErrorStaysInToolResult(t *testing.T) {
	reason := "端点 GET /system/api/siteinfo 不在开放范围内（mode_off）：请先把 mode 设为 read"
	spec := &IntentSpec{
		Name: "wire_gate", Title: "门禁接线", Domain: DomainSystem, Risk: RiskRead,
		Params:   map[string]ParamSpec{"action": {Type: "string", Desc: "操作", Required: true}},
		Required: []string{"action"},
		Compose: func(ctx context.Context, args map[string]any, cap CapInvoker) (*Result, error) {
			return nil, errors.New(reason)
		},
	}
	k := NewKernel(Config{}, nil, nil)
	k.intents[spec.Name] = spec
	req := &mcp.CallToolRequest{
		Params: &mcp.CallToolParamsRaw{Name: "wire_gate", Arguments: json.RawMessage(`{"action":"x"}`)},
	}

	res, err := k.makeHandler(spec.Name, spec, mcp.NewServer(&mcp.Implementation{Name: "t", Version: "v"}, nil))(
		context.Background(), req)
	if err != nil {
		t.Fatalf("失败必须以 tool-level 错误表达，不能返回 Go error（SDK 会升级成 JSON-RPC 协议错误并丢弃结果）：%v", err)
	}
	if !res.IsError {
		t.Errorf("isError 应为 true，实际=%+v", res)
	}
	txt := textOf(res)
	if !strings.Contains(txt, reason) {
		t.Errorf("拒绝理由必须在 Content 里回传，实际=%q", txt)
	}
}

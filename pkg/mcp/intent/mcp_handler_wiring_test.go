package intent

import (
	"context"
	"encoding/json"
	"errors"
	"testing"

	"github.com/modelcontextprotocol/go-sdk/mcp"
)

// newWireProbeKernel 造一个只含单个探针意图的 Kernel。
//
// 为什么单独造：真 Kernel 的意图来自全局 IntentCatalog（内容都是生产声明），
// 无法让某个 action 稳定返回"端点失败"来验证 handler 的 isError 接线。
// 这里用一个自造的 spec 直接控制 Compose 的返回值，把接线本身单独测掉。
func newWireProbeKernel(t *testing.T, text string) *mcp.CallToolResult {
	t.Helper()
	spec := &IntentSpec{
		Name: "wire_probe", Title: "接线探针", Domain: DomainContent, Risk: RiskRead,
		Params:   map[string]ParamSpec{"action": {Type: "string", Desc: "操作", Required: true}},
		Required: []string{"action"},
		Compose: func(ctx context.Context, args map[string]any, cap CapInvoker) (*Result, error) {
			// 关键：返回 nil error —— 复现 api_invoke 的真实行为
			// （端点失败被表达成"成功返回的 JSON"，Go 层不报错）
			return &Result{Text: text}, nil
		},
	}
	k := NewKernel(Config{}, nil, nil)
	k.intents[spec.Name] = spec // 同包测试可直接写私有字段

	req := &mcp.CallToolRequest{
		Params: &mcp.CallToolParamsRaw{
			Name:      "wire_probe",
			Arguments: json.RawMessage(`{"action":"x"}`),
		},
	}
	handler := k.makeHandler("wire_probe", spec)
	res, err := handler(context.Background(), req)
	if err != nil {
		t.Fatalf("handler 不应返回 Go error（失败应以 isError 表达）：%v", err)
	}
	return res
}

// TestHandlerWiringMarksEndpointFailure 端到端验证 handler 真的会把
// 「Compose 返回 nil error + Text 里带端点失败信号」标成 isError。
//
// 这条测试存在的理由：2026-10-03 修 isError 时，纯函数级测试（endpointFailure
// 判定）全绿，但**handler 接线是空的**也没人发现——纯函数测试压根碰不到 handler。
// 判定逻辑与接线是两件事，必须各有一个测试。
func TestHandlerWiringMarksEndpointFailure(t *testing.T) {
	cases := []struct {
		name     string
		text     string
		wantErr  bool
	}{
		{"端点 5xx", `{"ok":false,"status":500,"msg":"","data":null,"code":0}`, true},
		{"端点 ok=false", `{"ok":false,"status":200,"msg":"失败了","data":null,"code":0}`, true},
		{"控制器 code 非 0", `{"ok":true,"status":200,"code":0,"msg":"","data":{"code":-1,"msg":"not found","data":null}}`, true},
		{"正常成功", `{"ok":true,"status":200,"code":0,"msg":"","data":{"list":[],"total":0}}`, false},
		{"空响应（cap 回落）", ``, false},
	}
	for _, c := range cases {
		res := newWireProbeKernel(t, c.text)
		if res.IsError != c.wantErr {
			t.Errorf("%s: isError = %v，期望 %v（res=%+v）", c.name, res.IsError, c.wantErr, res)
		}
	}
}

// TestHandlerWiringMarksGoErrorAsIsError Compose 返回 Go error 时也必须标 isError。
// 这是修复前就有的行为，顺带锁住防回归。
func TestHandlerWiringMarksGoErrorAsIsError(t *testing.T) {
	spec := &IntentSpec{
		Name: "wire_probe_err", Title: "接线探针", Domain: DomainContent, Risk: RiskRead,
		Params:   map[string]ParamSpec{"action": {Type: "string", Desc: "操作", Required: true}},
		Required: []string{"action"},
		Compose: func(ctx context.Context, args map[string]any, cap CapInvoker) (*Result, error) {
			return nil, errors.New("参数缺失")
		},
	}
	k := NewKernel(Config{}, nil, nil)
	k.intents[spec.Name] = spec
	req := &mcp.CallToolRequest{
		Params: &mcp.CallToolParamsRaw{Name: "wire_probe_err", Arguments: json.RawMessage(`{"action":"x"}`)},
	}
	res, _ := k.makeHandler("wire_probe_err", spec)(context.Background(), req)
	if !res.IsError {
		t.Errorf("Compose 返回 Go error 时 isError 应为 true，实际=%+v", res)
	}
}

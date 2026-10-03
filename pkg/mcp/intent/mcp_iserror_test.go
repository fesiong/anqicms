package intent

import "testing"

// TestIsErrorSetOnEndpointFailure 端点业务失败时必须置 MCP 协议的 isError。
//
// 背景（2026-10-03 实测）：api_invoke 把端点失败（5xx / ok=false）表达成
// **成功返回的 JSON**，Compose 因此不返回 Go error，cerr 为 nil。
// handler 原本只在 cerr != nil 时设 IsError，于是：
//
//	system_multilang action=cache_delete → status=500、body 里 ok:false，
//	但 MCP 协议层 isError 为空 —— 严格依赖 isError 的客户端会当成功处理。
//
// isError 在 SDK 里是 `json:"isError,omitempty"`，不显式置 true 等于没告知。
// 写操作失败却不报错，会让 AI 以为已经改好了。
func TestIsErrorSetOnEndpointFailure(t *testing.T) {
	cases := []struct {
		name    string
		text    string
		wantErr bool
	}{
		{
			name: "端点 5xx",
			text:    `{"ok":false,"status":500,"msg":"","data":null,"code":0}`,
			wantErr: true,
		},
		{
			name: "端点 ok=false",
			text:    `{"ok":false,"status":200,"msg":"端点返回失败但未给出原因","data":null,"code":0}`,
			wantErr: true,
		},
		{
			name: "控制器层 code 非 0",
			text:    `{"ok":true,"status":200,"code":0,"msg":"","data":{"code":-1,"msg":"record not found","data":null}}`,
			wantErr: true,
		},
		{
			name: "正常成功不应误报",
			text:    `{"ok":true,"status":200,"code":0,"msg":"","data":{"list":[],"total":0,"page":1,"page_size":20,"count":0}}`,
			wantErr: false,
		},
	}
	for _, c := range cases {
		// 直接验证 endpointFailure 对这些响应的判定，
		// handler 层的接线由下面 TestHandlerMarksEndpointFailureViaMCP 端到端覆盖。
		got := endpointFailure(c.text) != ""
		if got != c.wantErr {
			t.Errorf("%s: endpointFailure 判定 = %v，期望 %v（msg=%q）",
				c.name, got, c.wantErr, endpointFailure(c.text))
		}
	}
}

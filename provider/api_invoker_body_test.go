package provider

import (
	"io"
	"net/http"
	"testing"
)

// TestBuildRequestBodyEmptyPostSendsEmptyJSON POST/PUT 零参数时必须发 `{}`，不能发 nil body。
//
// 背景（2026-10-04 实测）：buildRequestBody 在 len(fields)==0 时返回 nil reader，
// http.NewRequest 因此把 req.Body 置为 nil；iris 在 handler 的 ctx.ReadJSON 里
// 对 nil Body 做 io.ReadAll，直接 panic：
//
//	runtime error: invalid memory address or nil pointer dereference
//	controller/manageController/anqi.go:143
//
// 随后被 recover 兜成 500「服务器内部错误」，而 recover 的 hint 又写着
// 「通常不是参数问题」—— 真实原因恰恰是缺参数，AI 被带偏。
//
// 这不是个别端点的问题：anqi/template/download、anqi/skill/edit、website/save
// 空 body 调用全是同一个 panic。
//
// 修好后端点能走正常参数校验分支，返回「XX 不能为空」这类可读错误。
func TestBuildRequestBodyEmptyPostSendsEmptyJSON(t *testing.T) {
	for _, method := range []string{http.MethodPost, http.MethodPut, http.MethodPatch} {
		t.Run(method, func(t *testing.T) {
			body, ct, err := buildRequestBody(method, nil, nil)
			if err != nil {
				t.Fatalf("%s 零参数不应报错: %v", method, err)
			}
			if body == nil {
				t.Fatalf("%s 零参数返回了 nil body —— iris 读 nil Body 会 panic", method)
			}
			if ct != "application/json" {
				t.Errorf("%s Content-Type = %q，期望 application/json（否则 ReadJSON 仍会失败）", method, ct)
			}
			raw, err := io.ReadAll(body)
			if err != nil {
				t.Fatalf("读取 body 失败: %v", err)
			}
			if string(raw) != "{}" {
				t.Errorf("%s 零参数 body = %q，期望 \"{}\"", method, string(raw))
			}
		})
	}
}

// TestBuildRequestBodyGetDeleteStayBodyless GET/DELETE 仍不应有 body。
//
// 上一条修复不能顺手把 GET/DELETE 也塞上 `{}`：那会给本该无请求体的动词
// 加上 body，部分端点/中间件对 GET 带 body 敏感，且属于超出问题范围的改动。
func TestBuildRequestBodyGetDeleteStayBodyless(t *testing.T) {
	for _, method := range []string{http.MethodGet, http.MethodDelete} {
		body, ct, err := buildRequestBody(method, nil, map[string]any{"id": 1})
		if err != nil {
			t.Fatalf("%s 不应报错: %v", method, err)
		}
		if body != nil {
			t.Errorf("%s 不应返回 body，实际 %v", method, body)
		}
		if ct != "" {
			t.Errorf("%s Content-Type = %q，期望空", method, ct)
		}
	}
}

// TestBuildRequestBodyKeepsGivenFields 有参数时行为不变（回归护栏）。
func TestBuildRequestBodyKeepsGivenFields(t *testing.T) {
	body, ct, err := buildRequestBody(http.MethodPost, nil, map[string]any{"template_id": 1})
	if err != nil {
		t.Fatalf("不应报错: %v", err)
	}
	if body == nil || ct != "application/json" {
		t.Fatalf("有参数时应返回 json body，实际 body=%v ct=%q", body, ct)
	}
	raw, _ := io.ReadAll(body)
	if string(raw) != `{"template_id":1}` {
		t.Errorf("body = %q，期望 {\"template_id\":1}", string(raw))
	}
}

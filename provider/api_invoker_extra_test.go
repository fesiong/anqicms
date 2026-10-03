package provider

import (
	"fmt"
	"testing"
)

// TestInvokeAdminAPIKeepsEnvelopeExtras 端到端验证 InvokeResult.Extra 真的被填充。
//
// 这条断言不能只测 envelopeExtras 这个纯函数——2026-10-03 的反向对照里，
// 纯函数测试在「调用点被删掉」的情况下依然全绿（空转），
// 必须打到真实 handler 才能证明接线是通的。
//
// 背景：后台 list 端点（ArchiveList 等）把 total/exact 放在信封顶层，
// 旧实现只解 code/msg/data，分页信息被静默丢弃。
func TestInvokeAdminAPIKeepsEnvelopeExtras(t *testing.T) {
	site := mockInvokeSite()
	mockAdminAPIApp(t, site)

	res, err := site.InvokeAdminAPI(2, "GET", "/system/api/paginated", map[string]any{"id": 1})
	if err != nil {
		t.Fatalf("调用失败: %v", err)
	}
	if !res.OK {
		t.Fatalf("期望成功，实得 status=%d code=%d msg=%s", res.Status, res.Code, res.Msg)
	}
	if res.Extra == nil {
		t.Fatal("Extra 为 nil：分页信息（total/exact）被丢弃，调用方无法翻页")
	}
	if fmt.Sprint(res.Extra["total"]) != "268" {
		t.Errorf("total 应被保留，实际=%#v", res.Extra["total"])
	}
	if res.Extra["exact"] != true {
		t.Errorf("exact 应被保留，实际=%#v", res.Extra["exact"])
	}
	// 已固定的三个键不得混进 Extra
	for _, k := range []string{"code", "msg", "data"} {
		if _, ok := res.Extra[k]; ok {
			t.Errorf("%q 不应出现在 Extra 里", k)
		}
	}
	// data 本身仍要正常解析
	data, _ := res.Data.(map[string]any)
	if data == nil || data["id"] != "1" {
		t.Errorf("data 解析异常：%#v", res.Data)
	}
}

// TestEnvelopeExtrasKeepsPaginationFields envelopeExtras 纯函数的字段级断言。
func TestEnvelopeExtrasKeepsPaginationFields(t *testing.T) {
	raw := `{"code":0,"msg":"","total":268,"exact":true,"pages":14,"data":[{"id":1}]}`
	got := envelopeExtras(raw)
	if got == nil {
		t.Fatal("Extra 不应为 nil")
	}
	if fmt.Sprint(got["total"]) != "268" {
		t.Errorf("total 应被保留，实际=%#v", got["total"])
	}
	if got["exact"] != true {
		t.Errorf("exact 应被保留，实际=%#v", got["exact"])
	}
	if fmt.Sprint(got["pages"]) != "14" {
		t.Errorf("pages 应被保留，实际=%#v", got["pages"])
	}
	// 已知的三个键必须剔除，否则会与 InvokeResult 的固定字段重复
	for _, k := range []string{"code", "msg", "data"} {
		if _, ok := got[k]; ok {
			t.Errorf("%q 不应出现在 Extra 里", k)
		}
	}
}

// TestEnvelopeExtrasEdgeCases 空信封与非 JSON 都不能 panic。
func TestEnvelopeExtrasEdgeCases(t *testing.T) {
	// 无额外字段时返回 nil，不制造空 map
	if envelopeExtras(`{"code":0,"msg":"","data":{}}`) != nil {
		t.Error("无额外字段时 Extra 应为 nil")
	}
	if envelopeExtras(`<html>500</html>`) != nil {
		t.Error("非 JSON 应返回 nil")
	}
	if envelopeExtras(``) != nil {
		t.Error("空响应体应返回 nil")
	}
	if envelopeExtras(`[1,2,3]`) != nil {
		t.Error("顶层是数组时应返回 nil（不是 map）")
	}
}

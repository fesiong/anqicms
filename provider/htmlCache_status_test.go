package provider

import "testing"

// TestMarkHtmlCacheFinishedNilSafe 静态缓存未开启时 HtmlCacheStatus 是 nil，
// MarkHtmlCacheFinished 必须自己兜住而不是 panic。
//
// 实测踩过（2026-10-03）：Build*Cache 在 PluginHtmlCache.Open==false 时直接 return，
// 不分配 HtmlCacheStatus；四个手动构建入口随后裸写 w2.HtmlCacheStatus.FinishedTime，
// 触发 SIGSEGV 打挂整个服务进程（panic 后所有 MCP 调用返回 502）。
func TestMarkHtmlCacheFinishedNilSafe(t *testing.T) {
	// 1. 零值 Website（模拟 Open=false 从未分配过 Status 的情况）
	var w Website
	w.HtmlCacheStatus = nil
	w.MarkHtmlCacheFinished() // 不 panic 即通过
	if w.HtmlCacheStatus == nil {
		t.Fatal("MarkHtmlCacheFinished 应分配 HtmlCacheStatus")
	}
	if w.HtmlCacheStatus.FinishedTime == 0 {
		t.Error("FinishedTime 应被写入非零值")
	}
	if w.HtmlCacheStatus.StartTime == 0 {
		t.Error("懒分配时应同时补上 StartTime")
	}

	// 2. 已有 Status 时只更新时间，不重置其它字段
	w2 := Website{HtmlCacheStatus: &HtmlCacheStatus{
		StartTime: 12345, Total: 7, Current: "生成中", ErrorMsg: "保留",
	}}
	w2.MarkHtmlCacheFinished()
	s := w2.HtmlCacheStatus
	if s.StartTime != 12345 {
		t.Errorf("StartTime 不应被重写，实际=%d", s.StartTime)
	}
	if s.Total != 7 || s.Current != "生成中" || s.ErrorMsg != "保留" {
		t.Errorf("其它字段被意外改动：%+v", s)
	}

	// 3. nil 接收者也不能 panic（防御性：调用方可能拿到 nil *Website）
	var nilW *Website
	nilW.MarkHtmlCacheFinished()
}

package middleware

import (
	"os"
	"strconv"
	"strings"
	"testing"
)

// TestRecoverWritesResponseBody recover 中间件必须在 panic 时写出可读响应体。
//
// 背景（2026-10-03 实测）：recover 只设了 StatusCode(500) 而没写 body，
// iris 于是输出默认的纯文本 "500 Internal Server Error"；这个响应经过
// api_invoke 包装后变成：
//
//	{"code":0,"msg":"","ok":false,"status":500}
//
// 「code 是成功值 0、ok 却是 false、status 又是 500」的矛盾体，调用方
// （尤其是 AI）完全无法判读到底发生了什么——实测有 6 个 action 返回的都是它。
//
// 真实触发场景：POST 请求体为空时 ctx.ReadJSON(&req) 会 panic
//（iris 内部对 nil Body 做 io.ReadAll）：
//
//	contentops_translate/text_delete（不传任何参数）
//	interaction/guestbook_status
//	contentops_collector/combination、replace
//	contentops_material/import、contentops_import/template
//	system_multilang/cache_delete
//
// 这里断言「写响应体」这个动作存在，且写的是合法 JSON 信封。
func TestRecoverWritesResponseBody(t *testing.T) {
	src := readRecoverSource(t)

	// 写响应体：WriteString / Write 是 recover 分支里唯一的对外输出手段
	if !strings.Contains(src, "ctx.WriteString(") && !strings.Contains(src, "ctx.Write(") {
		t.Error("recover 必须写响应体，否则 api_invoke 只能拿到 code:0+status:500 的矛盾响应")
	}
	// 必须是标准信封：调用方靠 ok/code 判断成败
	for _, key := range []string{`"ok":false`, `"code":`, `"msg":`, `"status":500`} {
		if !strings.Contains(src, key) {
			t.Errorf("recover 的响应体应含 %s 字段（AI 靠它判读失败）", key)
		}
	}
	// 必须含 hint：告诉调用方下一步该做什么（看日志 / 联系管理员）
	if !strings.Contains(src, `"hint"`) {
		t.Error("recover 的响应体应含 hint，告诉调用方失败后该做什么")
	}
	// 真实错误细节只进日志，不回给调用方：响应体里的 msg 必须是固定文案，
	// 不能把 err 或 stacktrace 拼进去（会泄漏内部路径与实现细节）。
	i := strings.Index(src, "ctx.WriteString(")
	if i < 0 {
		t.Fatal("未找到写响应体的代码")
	}
	seg := src[i:]
	if n := strings.Index(seg, "))"); n > 0 {
		seg = seg[:n]
	}
	if strings.Contains(seg, "stacktrace") || strings.Contains(seg, "string(err)") {
		t.Errorf("响应体不应回显栈或原始错误：%s", seg)
	}
	if !strings.Contains(seg, "error.log") {
		t.Error("提示语应指引调用方查看 error.log")
	}
}

// TestRecoverJSONStrEscaping jsonStr 必须用 json.Marshal 而不是手写引号包裹。
func TestRecoverJSONStrEscaping(t *testing.T) {
	// 提示语里含中文标点与斜杠（cache/error.log），手写转义极易漏
	got := jsonStr("详情见 cache/error.log，含 \"引号\" 与 \\ 反斜杠")
	if !strings.HasPrefix(got, `"`) || !strings.HasSuffix(got, `"`) {
		t.Errorf("jsonStr 应返回带引号的 JSON 字符串，实际=%s", got)
	}
	// 内部引号必须被转义成 \"，否则拼进 JSON 信封就会破坏结构
	if !strings.Contains(got, `\"`) {
		t.Errorf("jsonStr 应把内部引号转义为 \\\"，实际=%s", got)
	}
	// 反斜杠必须成对转义
	if !strings.Contains(got, `\\`) {
		t.Errorf("jsonStr 应把反斜杠转义为 \\\\，实际=%s", got)
	}
	// 转义后仍应能被 JSON 解析回原值
	if unq, err := strconv.Unquote(got); err != nil {
		t.Errorf("jsonStr 的输出应能被 JSON 解析：%v", err)
	} else if unq != "详情见 cache/error.log，含 \"引号\" 与 \\ 反斜杠" {
		t.Errorf("往返后内容不一致：%q", unq)
	}
}

// readFile 读同目录文件（测试里多处需要读源码做文本断言）。
func readFile(name string) (string, error) {
	b, err := os.ReadFile(name)
	return string(b), err
}

// readRecoverSource 读 recover.go 源码。
// 用读源码而非行为断言：触发 panic 需要完整的 iris 上下文 + 数据库，
// 单测里构造不出；而「有没有写响应体」是纯文本可判的事实。
func readRecoverSource(t *testing.T) string {
	t.Helper()
	b, err := readFile("recover.go")
	if err != nil {
		t.Fatalf("读 recover.go 失败: %v", err)
	}
	return b
}

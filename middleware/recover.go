package middleware

import (
	"encoding/json"
	"fmt"
	"runtime"
	"strconv"
	"time"

	"github.com/kataras/iris/v12/context"
	"kandaoni.com/anqicms/config"
	"kandaoni.com/anqicms/library"
)

func NewRecover() context.Handler {
	return func(ctx *context.Context) {
		defer func() {
			if err := recover(); err != nil {
				if ctx.IsStopped() {
					return
				}

				var stacktrace string
				for i := 1; ; i++ {
					_, f, l, got := runtime.Caller(i)
					if !got {
						break
					}

					stacktrace += fmt.Sprintf("%s:%d\n", f, l)
				}

				// when stack finishes
				logMessage := fmt.Sprintf("Recovered from a route's Handler('%s')\n", ctx.HandlerName())
				logMessage += fmt.Sprintf("At Request: %s\n", getRequestLogs(ctx))
				logMessage += fmt.Sprintf("Trace: %s\n", err)
				logMessage += fmt.Sprintf("\n%s", stacktrace)
				library.DebugLog(config.ExecPath+"cache/", "error.log", time.Now().Format("2006-01-02 15:04:05"), logMessage)
				ctx.Application().Logger().Warn(logMessage)
				ctx.Values().Set("message", err)
				ctx.StatusCode(500)
				// 必须写响应体：只设状态码不写 body 的话 iris 会输出默认的
				// "500 Internal Server Error" 纯文本，经过 api_invoke 包装后变成
				// {"code":0,"msg":"","ok":false,"status":500} ——
				// 「成功码 + 失败标志」的矛盾响应，调用方（尤其 AI）无从判读出了什么事。
				// 实测有 6 个 action 返回的都是这个形状。
				//
				// 已知触发场景：POST 请求体为空时 ctx.ReadJSON(&req) 会 panic
				//（iris 内部对 nil Body 做 io.ReadAll），如 contentops_translate/text_delete
				// 不传任何参数、interaction/guestbook_status 等。
				//
				// 这里写标准信封，让调用方至少能判读失败并知道下一步。
				// 真实错误细节（栈、内部路径）只进 error.log，不回给调用方，避免泄漏实现细节。
				_, _ = ctx.WriteString(fmt.Sprintf(
					`{"code":%d,"msg":%s,"ok":false,"status":500,"hint":%s}`,
					config.StatusFailed,
					jsonStr("服务器内部错误：处理请求时发生异常，详情见服务端 cache/error.log"),
					jsonStr("该错误由异常恢复机制捕获，通常不是参数问题；若参数确认无误，请联系管理员查 error.log"),
				))
				ctx.StopExecution()
			}
		}()

		ctx.Next()
	}
}

// jsonStr 把字符串编码成 JSON 字面量（含引号转义）。
// 用 encoding/json 而不是手写拼接：提示语里有中文标点与斜杠，手写极易漏转义。
func jsonStr(s string) string {
	b, err := json.Marshal(s)
	if err != nil {
		return `""`
	}
	return string(b)
}

func getRequestLogs(ctx *context.Context) string {
	var status, ip, method, path string
	status = strconv.Itoa(ctx.GetStatusCode())
	path = ctx.Path()
	method = ctx.Method()
	ip = ctx.RemoteAddr()
	// the date should be logged by iris' Logger, so we skip them
	return fmt.Sprintf("%v %s %s %s", status, path, method, ip)
}

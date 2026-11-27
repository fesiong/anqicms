package controller

import (
	"strconv"

	"github.com/kataras/iris/v12"
	"github.com/kataras/iris/v12/context"
)

// 前台 HTML 页面的统一出口。
//
// 为什么需要这里：前台页面一定要显式回填 Content-Length，否则只能走
// net/http 的 Transfer-Encoding: chunked。两条独立原因（实测于 2026-11-19）：
//
//  1. 页面用 ctx.Recorder() 记录整页 HTML 交给 CacheHtmlData，而 iris v12.2.11 的
//     ResponseRecorder.FlushResponse 顺序是「先 ResponseWriter.FlushResponse()
//     （内部 WriteHeader）再 Write(chunks)」。响应头一旦发出，net/http 就再也没机会
//     按 body 长度补 Content-Length —— 与 body 多大无关，小页面一样是 chunked。
//  2. 即使不走 recorder，只要 handler 结束时 body 超过 net/http 的
//     bufferBeforeChunkingSize(2048) 缓冲，也会自动转 chunked。前台页面普遍
//     10k~25k，必然命中。
//
// 回填时机必须在 FlushResponse 之前：此刻 ctx.writer 还是 recorder，
// ctx.Header() 写进的是 recorder 的 header map，FlushResponse 会把它整体
// 复制到下层再 WriteHeader，所以长度能被真正发出。
// 详见 controller/content_length_guard_test.go 的正反对照。

// writeCachedHTML 输出命中的 HTML 缓存（只有一次写，长度已知）。
func writeCachedHTML(ctx iris.Context, html []byte) {
	ctx.ContentType(context.ContentHTMLHeaderValue)
	if len(html) > 0 {
		ctx.Header("Content-Length", strconv.Itoa(len(html)))
	}
	_, _ = ctx.Write(html)
}

// renderView 渲染模板页面，并在成功时按已录制的 body 长度补 Content-Length。
//
// 用 ctx.Recorder() 而不是直接 ctx.View：不录制就拿不到 body 长度，
// 也就无法在头发出前补上长度。recorder 是幂等的（ctx.Record() 对
// *ResponseRecorder 直接返回），不会二次包装。
func renderView(ctx iris.Context, viewPath string) error {
	recorder := ctx.Recorder()
	err := ctx.View(viewPath)
	if err != nil {
		return err
	}
	if body := recorder.Body(); len(body) > 0 {
		ctx.Header("Content-Length", strconv.Itoa(len(body)))
	}
	return nil
}

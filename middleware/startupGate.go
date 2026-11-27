package middleware

// 启动闸门：站点初始化（含库表迁移）没跑完之前，未就绪的站点返回 503。
//
// 放行粒度是**站点**，不是进程：106 个站点串行迁移时，前几个迁完就应该能正常
// 服务，不该陪着后面的站点一起停机（详见 provider/siteAccess.go）。
//
// 背景见 provider/startup.go 的头注释。这里只强调一件事：
// 闸门必须是**第一个**注册的中间件。它一旦让请求往下走，
// 就会落到 controller.ReRouteContext / CurrentSite 上，
// 而那个时刻的站点注册表是空的或者只有部分站点 ——
// 空表会走"全新安装"语义（一堆 404），部分站点会让未注册站点的请求
// 落到 `websites.MustGet(1)` 的 nil 兜底上（拿 A 站内容回应 B 站域名）。
// 所以"放行"必须建立在"这个请求归属的站点已经在注册表里"之上。

import (
	"encoding/json"
	"html"
	"strconv"
	"strings"

	"github.com/kataras/iris/v12"
	"kandaoni.com/anqicms/config"
	"kandaoni.com/anqicms/provider"
)

// StartupGate 在启动期按站点判定放行。
//
// 放行规则：
//   - 未开始初始化（Idle）：全新安装/测试，不挡；
//   - 已就绪（Ready）：全部站点处理完，不挡；
//   - 正在初始化（Running）：**只有该请求归属的站点已迁移完才放行**；
//   - 初始化 panic（Failed）：一律挡住。panic 意味着注册表状态未知，
//     宁可全站 503 等重启，也不要让请求读到半迁移的表（重启会按站点重试）。
//
// 状态端点 /__startup 由 StartupStatus 接管，永远不往下走。
func StartupGate(ctx iris.Context) {
	if ctx.RequestPath(false) == provider.StartupStatusPath {
		StartupStatus(ctx)
		return
	}
	if provider.StartupReady() {
		ctx.Next()
		return
	}
	// 整体还没就绪：按站点放行（判定与边界见 provider/siteAccess.go）。
	if provider.RequestServable(ctx) {
		ctx.Next()
		return
	}

	snap := provider.StartupSnapshotNow()
	// Retry-After 让浏览器/负载均衡知道这是"暂时不可用、稍后再试"，
	// 而不是把这个节点当成故障下线；no-store 防止 503 页被缓存下来。
	ctx.Header("Retry-After", strconv.Itoa(10))
	ctx.Header("Cache-Control", "no-store, must-revalidate")

	if wantsJSON(ctx) {
		body, _ := json.Marshal(snap)
		ctx.StatusCode(iris.StatusServiceUnavailable)
		ctx.ContentType("application/json; charset=utf-8")
		_, _ = ctx.Write(body)
		return
	}

	ctx.StatusCode(iris.StatusServiceUnavailable)
	ctx.ContentType("text/html; charset=utf-8")
	_, _ = ctx.WriteString(startupPage(snap))
}

// StartupStatus 是 GET /__startup 的处理器。
//
// 它同时充当**就绪探针**：未就绪返回 503，就绪返回 200，响应体是进度 JSON。
// 不要改成"永远 200 + 一个 ready 字段"——负载均衡和 K8s readinessProbe
// 只看状态码，永远 200 就等于把未迁移完的节点放进轮询。
func StartupStatus(ctx iris.Context) {
	snap := provider.StartupSnapshotNow()
	body, _ := json.Marshal(snap)
	ctx.Header("Cache-Control", "no-store, must-revalidate")
	if snap.Ready {
		ctx.StatusCode(iris.StatusOK)
	} else {
		ctx.StatusCode(iris.StatusServiceUnavailable)
	}
	ctx.ContentType("application/json; charset=utf-8")
	_, _ = ctx.Write(body)
}

func wantsJSON(ctx iris.Context) bool {
	if strings.HasPrefix(ctx.RequestPath(false), "/api/") {
		return true
	}
	return strings.Contains(strings.ToLower(ctx.GetHeader("Accept")), "application/json")
}

// startupPage 是给人看的那一版。不依赖任何模板：模板目录要等站点初始化
// 才可用，而这个页面恰恰是"站点还没初始化"时唯一的出口。
func startupPage(snap provider.StartupSnapshot) string {
	brand := html.EscapeString(config.Brand().Name)
	title := "该站点正在升级"
	summary := snap.Message
	if snap.Phase == provider.StartupPhaseFailed.String() {
		title = "站点初始化失败"
	}

	progress := ""
	if snap.Total > 0 {
		progress = `<div class="bar"><i style="width:` + percent(snap.Processed, snap.Total) + `%"></i></div>` +
			`<p class="meta">已处理 ` + strconv.FormatInt(snap.Processed, 10) + ` / ` + strconv.FormatInt(snap.Total, 10) +
			` 个站点 · 已耗时 ` + html.EscapeString(snap.Elapsed) + `</p>`
	} else if snap.Elapsed != "" && snap.Elapsed != "0s" {
		progress = `<p class="meta">已耗时 ` + html.EscapeString(snap.Elapsed) + `</p>`
	}

	current := ""
	if snap.Current != "" {
		current = `<p class="meta">当前：` + html.EscapeString(snap.Current) + `</p>`
	}

	hint := "本页每 5 秒自动刷新。已完成升级的站点不受影响，升级期间请勿重启服务。"
	if snap.Phase == provider.StartupPhaseFailed.String() {
		hint = "请查看服务端日志（进程输出或 cache/error.log），修复后重启服务；已完成的站点不会重复迁移。"
	}

	return `<!doctype html>
<html lang="zh-CN">
<head>
<meta charset="utf-8">
<meta name="viewport" content="width=device-width,initial-scale=1">
<meta http-equiv="refresh" content="5">
<title>` + html.EscapeString(title) + ` · ` + brand + `</title>
<style>
*{box-sizing:border-box}
body{margin:0;min-height:100vh;display:flex;align-items:center;justify-content:center;
background:#f5f6f8;color:#1f2329;font:15px/1.6 -apple-system,BlinkMacSystemFont,"Segoe UI","PingFang SC","Hiragino Sans GB","Microsoft YaHei",sans-serif}
.card{width:min(92vw,520px);background:#fff;border:1px solid #e5e6eb;border-radius:12px;
padding:32px 28px;box-shadow:0 6px 24px rgba(31,35,41,.06)}
h1{margin:0 0 8px;font-size:20px;font-weight:600}
p{margin:0 0 12px}
.summary{color:#4e5969}
.bar{height:6px;border-radius:3px;background:#eef0f3;overflow:hidden;margin:18px 0 10px}
.bar>i{display:block;height:100%;border-radius:3px;background:#1890ff;transition:width .4s ease}
.meta{color:#86909c;font-size:13px;margin:0 0 6px}
.hint{margin-top:18px;padding-top:14px;border-top:1px solid #f0f1f3;color:#86909c;font-size:13px}
</style>
</head>
<body>
<div class="card">
<h1>` + html.EscapeString(title) + `</h1>
<p class="summary">` + html.EscapeString(summary) + `</p>
` + progress + current + `
<p class="hint">` + hint + `</p>
</div>
</body>
</html>`
}

func percent(done, total int64) string {
	if total <= 0 {
		return "0"
	}
	if done < 0 {
		done = 0
	}
	if done > total {
		done = total
	}
	return strconv.FormatInt(done*100/total, 10)
}

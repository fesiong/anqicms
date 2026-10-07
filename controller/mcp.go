package controller

import (
	"context"

	"github.com/kataras/iris/v12"
	"kandaoni.com/anqicms/provider"
)

// McpStreamableHTTP 处理 MCP Streamable HTTP 请求。
//
// middleware.McpTokenAuth 已完成 token 校验与目标站点（site_id）解析，
// 并把目标站点与审计信息写入 iris ctx.Values()。
// 本 handler 取出目标站点，将审计信息注入 http.Request.Context，
// 然后按目标站点 ID 从 SiteMcpPool 获取缓存的 StreamableHTTPHandler 处理请求。
//
// 关键：Streamable HTTP 的 session 绑定在 handler 实例上，
// 必须复用同一 handler 才能跨请求保持 session（initialize → tools/list → tools/call）。
// 使用目标站点 ID 作为池键，可保证同一目标站点的 session 一致。
func McpStreamableHTTP(ctx iris.Context) {
	pool := provider.GetMcpPool()
	if pool == nil {
		ctx.StatusCode(iris.StatusServiceUnavailable)
		ctx.JSON(iris.Map{"error": "MCP pool not initialized"})
		return
	}

	// 目标站点由 middleware 解析（支持 site_id 切换）
	var site *provider.Website
	if v, ok := ctx.Values().Get("mcpSite").(*provider.Website); ok && v != nil {
		site = v
	} else {
		site = provider.CurrentSite(ctx)
	}
	if site == nil || site.McpSrv == nil {
		ctx.StatusCode(iris.StatusServiceUnavailable)
		ctx.JSON(iris.Map{"error": "MCP server not initialized for this site"})
		return
	}

	// 将审计信息注入 http.Request.Context，供工具 handler 闭包里的审计包装器读取
	r := ctx.Request()
	rctx := r.Context()
	if v, ok := ctx.Values().Get("mcpSiteID").(uint); ok {
		rctx = context.WithValue(rctx, provider.McpAuditSiteIDKey, v)
	}
	if v, ok := ctx.Values().Get("mcpTokenMask").(string); ok {
		rctx = context.WithValue(rctx, provider.McpAuditTokenMaskKey, v)
	}
	if v, ok := ctx.Values().Get("mcpClientIP").(string); ok {
		rctx = context.WithValue(rctx, provider.McpAuditClientIPKey, v)
	}
	if v, ok := ctx.Values().Get("mcpUserAgent").(string); ok {
		rctx = context.WithValue(rctx, provider.McpAuditUserAgentKey, v)
	}
	r = r.WithContext(rctx)

	// 按目标站点 ID 获取或创建缓存的 StreamableHTTPHandler
	handler, err := pool.GetOrCreate(r.Context(), site.Id, site.McpSrv.GetServer())
	if err != nil || handler == nil {
		ctx.StatusCode(iris.StatusInternalServerError)
		ctx.JSON(iris.Map{"error": "failed to get MCP handler"})
		return
	}

	handler.ServeHTTP(ctx.ResponseWriter(), r)
}

package controller

import (
	"github.com/kataras/iris/v12"
	"kandaoni.com/anqicms/provider"
)

// McpStreamableHTTP 处理 MCP Streamable HTTP 请求。
//
// middleware.McpTokenAuth 已完成站点解析（按域名）和 token 校验。
// 本 handler 直接通过 provider.CurrentSite(ctx) 重新解析站点，
// 然后从 SiteMcpPool 获取缓存的 StreamableHTTPHandler 处理请求。
//
// 关键：Streamable HTTP 的 session 绑定在 handler 实例上，
// 必须复用同一 handler 才能跨请求保持 session（initialize → tools/list → tools/call）。
func McpStreamableHTTP(ctx iris.Context) {
	pool := provider.GetMcpPool()
	if pool == nil {
		ctx.StatusCode(iris.StatusServiceUnavailable)
		ctx.JSON(iris.Map{"error": "MCP pool not initialized"})
		return
	}

	// 直接解析站点，不依赖 middleware 注入的 Values
	site := provider.CurrentSite(ctx)
	if site == nil {
		ctx.StatusCode(iris.StatusUnauthorized)
		ctx.JSON(iris.Map{"error": "site not resolved"})
		return
	}

	// 站点的 mcp.Server 在初始化时已创建（Website.McpSrv）
	if site.McpSrv == nil {
		ctx.StatusCode(iris.StatusServiceUnavailable)
		ctx.JSON(iris.Map{"error": "MCP server not initialized for this site"})
		return
	}

	// 从池中获取或创建缓存的 StreamableHTTPHandler
	handler, err := pool.GetOrCreate(ctx.Request().Context(), site.Id, site.McpSrv.GetServer())
	if err != nil || handler == nil {
		ctx.StatusCode(iris.StatusInternalServerError)
		ctx.JSON(iris.Map{"error": "failed to get MCP handler"})
		return
	}

	handler.ServeHTTP(ctx.ResponseWriter(), ctx.Request())
}

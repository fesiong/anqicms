package middleware

import (
	"strconv"
	"strings"

	"github.com/kataras/iris/v12"
	"kandaoni.com/anqicms/provider"
)

// McpTokenAuth 校验 MCP 请求的鉴权 token，并解析目标站点（site_id 切换）。
//
// 鉴权：仅在主站点（Id==1）配置一份 token，所有站点共用该配置。
// 多站点（多语言）模式下，主站点拥有所有站点的操作权限，
// 因此通过校验后可将请求切换到任意存在的 site_id。
//
// 流程：
//  1. 读取主站点 ai_setting.Mcp 配置
//  2. 校验 Mcp.Enabled == true 且 token 一致
//  3. 解析 site_id（query site_id > header X-Site-Id > header Sub-Site-Id），
//     解析失败或目标站点不存在则拒绝
//  4. （可选）按 token 掩码做每分钟调用限流
//  5. 将目标站点与审计信息注入 ctx.Values() 供 controller 使用
func McpTokenAuth(ctx iris.Context) {
	defaultSite := provider.CurrentSite(nil)
	if defaultSite == nil {
		ctx.StatusCode(iris.StatusNotFound)
		ctx.JSON(iris.Map{"error": "site not found"})
		return
	}

	aiSetting := defaultSite.LoadAiSetting("")
	mcpCfg := aiSetting.Mcp

	if !mcpCfg.Enabled {
		ctx.StatusCode(iris.StatusForbidden)
		ctx.JSON(iris.Map{"error": "MCP disabled for this site"})
		return
	}

	// token 校验：空 token 禁止访问
	token := strings.TrimSpace(strings.TrimPrefix(ctx.GetHeader("Authorization"), "Bearer "))
	if mcpCfg.Token == "" {
		ctx.StatusCode(iris.StatusForbidden)
		ctx.JSON(iris.Map{"error": "MCP token not configured"})
		return
	}
	if token != mcpCfg.Token {
		ctx.StatusCode(iris.StatusUnauthorized)
		ctx.JSON(iris.Map{"error": "invalid token"})
		return
	}

	// 解析目标站点（site_id 切换）
	siteID := resolveMcpSiteID(ctx)
	var targetSite *provider.Website
	if siteID == 0 {
		targetSite = provider.CurrentSite(ctx)
	} else {
		targetSite = provider.ResolveMcpSite(siteID)
		if targetSite == nil {
			ctx.StatusCode(iris.StatusNotFound)
			ctx.JSON(iris.Map{"error": "target site not found"})
			return
		}
	}

	// 限流：按 token 掩码做每分钟固定窗口计数
	if mcpCfg.RateLimit > 0 {
		mask := provider.MaskMcpToken(mcpCfg.Token)
		if !provider.CheckMcpRateLimit(mask, mcpCfg.RateLimit) {
			ctx.StatusCode(iris.StatusTooManyRequests)
			ctx.JSON(iris.Map{"error": "rate limit exceeded"})
			return
		}
	}

	// 注入目标站点与审计上下文
	mask := provider.MaskMcpToken(mcpCfg.Token)
	ctx.Values().Set("mcpSite", targetSite)
	ctx.Values().Set("mcpSiteID", targetSite.Id)
	ctx.Values().Set("mcpTokenMask", mask)
	ctx.Values().Set("mcpClientIP", clientIP(ctx))
	ctx.Values().Set("mcpUserAgent", ctx.GetHeader("User-Agent"))
	// 让 provider.CurrentSite(ctx) 也解析到目标站点
	ctx.Values().Set("siteId", targetSite.Id)

	ctx.Next()
}

// resolveMcpSiteID 从请求中解析 site_id（多站点切换）。
// 优先级：query 参数 site_id > 头 X-Site-Id > 头 Sub-Site-Id（沿用多语言约定）。
func resolveMcpSiteID(ctx iris.Context) uint {
	raw := ctx.URLParam("site_id")
	if raw == "" {
		raw = ctx.GetHeader("X-Site-Id")
	}
	if raw == "" {
		raw = ctx.GetHeader("Sub-Site-Id")
	}
	if raw == "" {
		return 0
	}
	id, err := strconv.ParseUint(raw, 10, 64)
	if err != nil || id == 0 {
		return 0
	}
	return uint(id)
}

// clientIP 返回调用方真实 IP，依次尝试 X-Forwarded-For / X-Real-IP / RemoteAddr。
func clientIP(ctx iris.Context) string {
	if xff := ctx.GetHeader("X-Forwarded-For"); xff != "" {
		if idx := strings.IndexByte(xff, ','); idx >= 0 {
			return strings.TrimSpace(xff[:idx])
		}
		return strings.TrimSpace(xff)
	}
	if xri := ctx.GetHeader("X-Real-IP"); xri != "" {
		return strings.TrimSpace(xri)
	}
	return ctx.RemoteAddr()
}

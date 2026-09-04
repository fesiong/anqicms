package middleware

import (
	"strings"

	"github.com/kataras/iris/v12"
	"kandaoni.com/anqicms/provider"
)

// McpTokenAuth 校验 MCP 请求的鉴权 token。
//
// 流程：
//  1. 通过 provider.CurrentSite(ctx) 按域名解析当前站点
//  2. 读取该站点的 ai_setting.Mcp 配置
//  3. 校验 Mcp.Enabled == true
//  4. 校验 Authorization: Bearer {token} 与 Mcp.Token 一致
//
// 校验通过后将 *provider.Website 注入 ctx.Values() 供后续 handler 使用。
func McpTokenAuth(ctx iris.Context) {
	site := provider.CurrentSite(ctx)
	if site == nil {
		ctx.StatusCode(iris.StatusNotFound)
		ctx.JSON(iris.Map{"error": "site not found"})
		return
	}

	// 仅默认站点（Id==1）存储 ai_setting；非默认站点走默认站点配置
	defaultSite := provider.CurrentSite(nil)
	if defaultSite == nil {
		defaultSite = site
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

	// 注入站点供后续 handler 使用
	ctx.Values().Set("mcpSite", site)
	ctx.Next()
}

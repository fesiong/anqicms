package provider

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"strings"
	"sync"
	"time"

	"kandaoni.com/anqicms/model"
	"kandaoni.com/anqicms/pkg/ai/eino"
)

// MCP 审计所需的请求上下文 key。
// 这些值由 controller.McpStreamableHTTP 从 iris 上下文取出后注入到
// http.Request.Context，最终被工具 handler 闭包里的审计包装器读取。
type mcpCtxKey string

const (
	McpAuditSiteIDKey    mcpCtxKey = "mcp.audit.site_id"
	McpAuditTokenMaskKey mcpCtxKey = "mcp.audit.token_mask"
	McpAuditClientIPKey  mcpCtxKey = "mcp.audit.client_ip"
	McpAuditUserAgentKey mcpCtxKey = "mcp.audit.user_agent"
)

// GetMcpConfig 返回主站点（Id==1）的 MCP 配置。
// 设计上 MCP 仅在主站点配置一份 token，所有站点共用该配置。
func GetMcpConfig() eino.McpConfig {
	defaultSite := CurrentSite(nil)
	if defaultSite == nil {
		return eino.McpConfig{}
	}
	return defaultSite.LoadAiSetting("").Mcp
}

// ResolveMcpSite 按 site_id 解析目标站点。
// 多站点（多语言）模式下，主站点（Id==1）拥有所有站点的操作权限，
// 因此只要目标站点存在即允许切换；不存在返回 nil。
func ResolveMcpSite(siteID uint) *Website {
	if siteID == 0 {
		return nil
	}
	return GetWebsite(siteID)
}

// MaskMcpToken 对 token 打码：保留前后各 4 位，中间以 * 填充。
// 短于等于 8 位的 token 整体打码。审计日志只存掩码。
func MaskMcpToken(token string) string {
	if token == "" {
		return ""
	}
	if len(token) <= 8 {
		return strings.Repeat("*", len(token))
	}
	return token[:4] + strings.Repeat("*", len(token)-8) + token[len(token)-4:]
}

// ---- 限流器：固定窗口（每分钟）----

type mcpRateBucket struct {
	count   int
	resetAt time.Time
}

var (
	mcpRateBuckets sync.Map // key(string) -> *mcpRateBucket
	mcpRateMu      sync.Mutex
)

// CheckMcpRateLimit 检查并递增某 key 的调用计数（固定窗口 1 分钟）。
// limit<=0 表示不限制，始终放行。返回 true 表示允许调用。
func CheckMcpRateLimit(key string, limit int) bool {
	if limit <= 0 {
		return true
	}
	now := time.Now()
	v, _ := mcpRateBuckets.LoadOrStore(key, &mcpRateBucket{resetAt: now.Add(time.Minute)})
	b := v.(*mcpRateBucket)
	mcpRateMu.Lock()
	defer mcpRateMu.Unlock()
	if now.After(b.resetAt) {
		b.count = 0
		b.resetAt = now.Add(time.Minute)
	}
	if b.count >= limit {
		return false
	}
	b.count++
	return true
}

// mcpToolRiskLevel 根据工具名粗粒度分级，用于审计标注。
func mcpToolRiskLevel(name string) string {
	n := strings.ToLower(name)
	if strings.Contains(n, "delete") || strings.Contains(n, "remove") ||
		strings.Contains(n, "drop") || strings.Contains(n, "reset") ||
		strings.Contains(n, "truncate") || strings.Contains(n, "clear") ||
		strings.Contains(n, "empty") {
		return "destructive"
	}
	if strings.HasPrefix(n, "setting_") || strings.HasPrefix(n, "plugin_backup") ||
		strings.HasPrefix(n, "plugin_migrate") || strings.HasPrefix(n, "user_") ||
		strings.HasPrefix(n, "admin_") || n == "version" || n == "anqi_info" ||
		n == "rewrite_form" {
		return "system"
	}
	if strings.Contains(n, "create") || strings.Contains(n, "update") ||
		strings.Contains(n, "save") || strings.Contains(n, "publish") ||
		strings.Contains(n, "import") || strings.Contains(n, "sort") ||
		strings.Contains(n, "status") || strings.Contains(n, "toggle") ||
		strings.Contains(n, "restore") || strings.Contains(n, "approve") ||
		strings.Contains(n, "run") {
		return "write"
	}
	return "read"
}

// recordMcpAudit 将一次意图调用写入目标站点的审计表。
// 调用方（intent.Kernel 的审计回调）已通过 context 注入 clientIP / tokenMask / userAgent，
// 并传入由 spec.Risk 精确给出的风险等级。
func (w *Website) recordMcpAudit(ctx context.Context, tool, risk, argsJSON string, callErr error, start time.Time) {
	if w.DB == nil {
		return
	}
	tokenMask, _ := ctx.Value(McpAuditTokenMaskKey).(string)
	clientIP, _ := ctx.Value(McpAuditClientIPKey).(string)
	ua, _ := ctx.Value(McpAuditUserAgentKey).(string)

	if risk == "" {
		risk = mcpToolRiskLevel(tool)
	}
	digest := sha256.Sum256([]byte(argsJSON))
	preview := argsJSON
	if len(preview) > 500 {
		preview = preview[:500]
	}
	ok := 1
	errMsg := ""
	if callErr != nil {
		ok = 0
		errMsg = callErr.Error()
	}

	log := &model.AiMcpAuditLog{
		SiteId:      w.Id,
		TokenMask:   tokenMask,
		Tool:        tool,
		Risk:        risk,
		ArgsDigest:  hex.EncodeToString(digest[:]),
		ArgsPreview: preview,
		Ok:          ok,
		ErrMsg:      errMsg,
		CostMs:      time.Since(start).Milliseconds(),
		ClientIp:    clientIP,
		UserAgent:   ua,
	}
	w.DB.Create(log)
}

// RebuildMcpServers 依据最新 MCP 配置重建所有站点的 mcp.Server 并清空连接池缓存。
// 在后台保存 MCP 配置（ExposedTools 变更）后调用，使白名单即时生效。
func RebuildMcpServers() {
	for _, w := range GetWebsites() {
		w.NewMcpServer()
	}
	if mcpPool != nil {
		mcpPool.Reset()
	}
}

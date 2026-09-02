package server

import (
	"context"
	"fmt"
	"log/slog"
	"net/http"
	"sync"

	"github.com/modelcontextprotocol/go-sdk/mcp"
)

// siteEntry 缓存单个站点的 StreamableHTTPHandler。
// Streamable HTTP 的 session 绑定在 handler 实例上，
// 必须复用同一 handler 才能跨请求保持 session。
type siteEntry struct {
	handler *mcp.StreamableHTTPHandler
}

// SiteMcpPool 维护每站点的 StreamableHTTPHandler 缓存。
// 站点首次请求时懒加载，站点删除时调用 Remove 清理。
//
// 注意：mcp.Server 实例由 Website.McpSrv 持有（同 AiSrv 模式），
// pool 仅负责缓存 handler，不负责创建 server。
type SiteMcpPool struct {
	mu      sync.RWMutex
	entries map[uint]*siteEntry // siteID → entry
	logger  *slog.Logger
}

// NewSiteMcpPool 创建实例池。
func NewSiteMcpPool(logger *slog.Logger) *SiteMcpPool {
	if logger == nil {
		logger = slog.Default()
	}
	return &SiteMcpPool{
		entries: make(map[uint]*siteEntry),
		logger:  logger,
	}
}

// GetOrCreate 按站点 ID 获取或懒加载创建 StreamableHTTPHandler。
// srv 参数为该站点已创建的 mcp.Server（来自 Website.McpSrv）。
func (p *SiteMcpPool) GetOrCreate(ctx context.Context, siteID uint, srv *mcp.Server) (*mcp.StreamableHTTPHandler, error) {
	if srv == nil {
		return nil, fmt.Errorf("mcp server is nil for site %d", siteID)
	}

	// 快路径：读锁
	p.mu.RLock()
	if e, ok := p.entries[siteID]; ok {
		p.mu.RUnlock()
		return e.handler, nil
	}
	p.mu.RUnlock()

	// 慢路径：写锁 + 双重检查
	p.mu.Lock()
	defer p.mu.Unlock()
	if e, ok := p.entries[siteID]; ok {
		return e.handler, nil
	}

	handler := mcp.NewStreamableHTTPHandler(
		func(*http.Request) *mcp.Server { return srv },
		&mcp.StreamableHTTPOptions{Stateless: true},
	)

	p.entries[siteID] = &siteEntry{handler: handler}
	p.logger.Info("mcp handler cached for site", "siteId", siteID)
	return handler, nil
}

// Remove 站点删除时从池中移除对应 handler。
func (p *SiteMcpPool) Remove(siteID uint) {
	p.mu.Lock()
	delete(p.entries, siteID)
	p.mu.Unlock()
	p.logger.Info("mcp handler removed for site", "siteId", siteID)
}

// Close 清理所有站点的 handler（服务停止时调用）。
func (p *SiteMcpPool) Close() {
	p.mu.Lock()
	p.entries = make(map[uint]*siteEntry)
	p.mu.Unlock()
	p.logger.Info("all mcp handlers closed")
}

// Count 返回当前池中的 handler 数量。
func (p *SiteMcpPool) Count() int {
	p.mu.RLock()
	defer p.mu.RUnlock()
	return len(p.entries)
}

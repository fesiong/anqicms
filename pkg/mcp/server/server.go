package server

import (
	"context"
	"log/slog"
	"net/http"

	"github.com/modelcontextprotocol/go-sdk/mcp"
)

// Server represents the MCP server for AnQiCMS.
// 每个 Server 实例对应一个站点，包含该站点的全部工具注册。
type Server struct {
	mcpServer *mcp.Server
	logger    *slog.Logger
	ctx       context.Context
}

// ServerConfig holds configuration for MCP server
type ServerConfig struct {
	ServerName    string
	ServerVersion string
	Instructions  string
	Logger        *slog.Logger
}

// DefaultConfig returns default MCP server configuration
func DefaultConfig() *ServerConfig {
	return &ServerConfig{
		ServerName:    "AnQiCMS",
		ServerVersion: "1.0.0",
		Instructions:  "AnQiCMS MCP Server - AI-powered CMS management",
		Logger:        slog.Default(),
	}
}

// New creates a new MCP server instance
func New(cfg *ServerConfig) (*Server, error) {
	if cfg == nil {
		cfg = DefaultConfig()
	}

	opts := &mcp.ServerOptions{
		Instructions: cfg.Instructions,
		Logger:       cfg.Logger,
	}

	mcpServer := mcp.NewServer(&mcp.Implementation{
		Name:    cfg.ServerName,
		Version: cfg.ServerVersion,
	}, opts)

	return &Server{
		mcpServer: mcpServer,
		logger:    cfg.Logger,
		ctx:       context.Background(),
	}, nil
}

// AddTool registers a tool with the MCP server
func (s *Server) AddTool(tool *mcp.Tool, handler mcp.ToolHandler) error {
	s.mcpServer.AddTool(tool, handler)
	s.logger.Info("tool added", "name", tool.Name)
	return nil
}

// AddTools registers multiple tools at once
func (s *Server) AddTools(tools []ToolDef) error {
	for _, td := range tools {
		if err := s.AddTool(td.Tool, td.Handler); err != nil {
			return err
		}
	}
	return nil
}

// Stop gracefully shuts down the server
func (s *Server) Stop(ctx context.Context) error {
	s.logger.Info("shutting down MCP server")
	return nil
}

// ToolDef is a pair of Tool and its handler
type ToolDef struct {
	Tool    *mcp.Tool
	Handler mcp.ToolHandler
}

// GetServer returns the underlying mcp.Server (for advanced usage)
func (s *Server) GetServer() *mcp.Server {
	return s.mcpServer
}

// GetContext returns the server's context
func (s *Server) GetContext() context.Context {
	return s.ctx
}

// StreamableHTTPHandler 暴露本 Server 的 Streamable HTTP 端点。
// 使用 mcp-go v1.4.0 的 NewStreamableHTTPHandler。
func (s *Server) StreamableHTTPHandler() http.Handler {
	return mcp.NewStreamableHTTPHandler(func(*http.Request) *mcp.Server {
		return s.mcpServer
	}, nil)
}

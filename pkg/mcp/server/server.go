package server

import (
	"context"
	"log/slog"
	"net/http"

	"github.com/modelcontextprotocol/go-sdk/mcp"
)

// Server represents the MCP server for AnQiCMS
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
	Port          int
	Host          string
	Logger        *slog.Logger
}

// DefaultConfig returns default MCP server configuration
func DefaultConfig() *ServerConfig {
	return &ServerConfig{
		ServerName:    "AnQiCMS",
		ServerVersion: "1.0.0",
		Instructions:  "AnQiCMS MCP Server - AI-powered CMS management",
		Port:          8081,
		Host:          "0.0.0.0",
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

// Start starts the MCP server HTTP endpoint
func (s *Server) Start() *http.ServeMux {
	mux := http.NewServeMux()
	mux.HandleFunc("/mcp", s.handleMCP)
	mux.HandleFunc("/health", s.handleHealth)
	return mux
}

// handleMCP handles MCP protocol requests
func (s *Server) handleMCP(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	// MCP uses JSON-RPC over HTTP
	w.Header().Set("Content-Type", "application/json")
	// In production, this would parse and route JSON-RPC requests
	// For now, return a basic response
	w.WriteHeader(http.StatusOK)
}

// handleHealth returns health check response
func (s *Server) handleHealth(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)
	w.Write([]byte(`{"status":"ok","service":"anqicms-mcp"}`))
}

// Stop gracefully shuts down the server
func (s *Server) Stop(ctx context.Context) error {
	s.logger.Info("shutting down MCP server")
	// TODO: clean up sessions
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

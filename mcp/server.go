package mcp

import (
	"context"
	"fmt"
	"log/slog"
	"net/http"
	"os"

	mcpserver "github.com/mark3labs/mcp-go/server"
)

const serverName = "openbooks"
const serverVersion = "1.0.0"

// ServerConfig holds MCP server startup options.
type ServerConfig struct {
	// IRC / session options
	IRC Config

	// Transport
	Mock bool   // use fake data, no IRC connection
	Port int    // if > 0, serve HTTP (StreamableHTTP) on this port; otherwise use stdio
	Host string // host to bind to (default "127.0.0.1")
}

// NewMCPHandler builds the MCP StreamableHTTP http.Handler for mounting into
// an existing HTTP server (e.g. the OpenBooks web server at /mcp). The caller
// is responsible for starting the IRC session via Connect and passing it as
// src, or passing a MockSession for testing.
//
// Mount the returned handler at a path, e.g.:
//
//	router.Mount("/mcp", mcp.NewMCPHandler(sess))
func NewMCPHandler(src bookSource) http.Handler {
	s := mcpserver.NewMCPServer(serverName, serverVersion, mcpserver.WithToolCapabilities(false))
	registerTools(s, src)
	return mcpserver.NewStreamableHTTPServer(s)
}

// Start connects to IRC (or creates a mock session) and then starts the MCP
// server. It blocks until ctx is cancelled or the server exits.
func Start(ctx context.Context, cfg ServerConfig) error {
	var src bookSource

	if cfg.Mock {
		cfg.IRC.Log.Info("mock mode enabled — no IRC connection")
		src = NewMockSession(cfg.IRC.DownloadDir)
	} else {
		sess, err := Connect(ctx, cfg.IRC)
		if err != nil {
			return fmt.Errorf("IRC connect failed: %w", err)
		}
		defer sess.Close()
		src = sess
	}

	s := mcpserver.NewMCPServer(serverName, serverVersion, mcpserver.WithToolCapabilities(false))
	registerTools(s, src)

	if cfg.Port > 0 {
		addr := fmt.Sprintf("%s:%d", cfg.Host, cfg.Port)
		cfg.IRC.Log.Info("starting MCP HTTP/StreamableHTTP server", "addr", addr)
		httpServer := mcpserver.NewStreamableHTTPServer(s)
		return httpServer.Start(addr)
	}

	cfg.IRC.Log.Info("starting MCP stdio server")
	return mcpserver.ServeStdio(s)
}

// DefaultLogger returns a slog logger that writes to stderr.
func DefaultLogger() *slog.Logger {
	return slog.New(slog.NewTextHandler(os.Stderr, &slog.HandlerOptions{
		Level: slog.LevelInfo,
	}))
}


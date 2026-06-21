package main

import (
	"context"
	"log"
	"os"
	"os/signal"
	"syscall"

	obsmcp "github.com/evan-buss/openbooks/mcp"
	"github.com/spf13/cobra"
)

var mcpMock bool
var mcpPort int
var mcpHost string
var mcpDir string
var mcpFormats []string

func init() {
	desktopCmd.AddCommand(mcpCmd)

	mcpCmd.Flags().BoolVar(&mcpMock, "mock", false, "Use fake data instead of a real IRC connection (for testing).")
	mcpCmd.Flags().IntVarP(&mcpPort, "port", "p", 0, "Port to serve MCP over HTTP/StreamableHTTP. If 0, uses stdio transport.")
	mcpCmd.Flags().StringVar(&mcpHost, "host", "127.0.0.1", "Host to bind the HTTP/StreamableHTTP server to.")
	mcpCmd.Flags().StringVarP(&mcpDir, "dir", "d", os.TempDir(), "Directory where downloaded books are saved.")
	mcpCmd.Flags().StringSliceVar(&mcpFormats, "formats", []string{"epub"}, "Comma-separated list of accepted file formats.")
}

var mcpCmd = &cobra.Command{
	Use:   "mcp",
	Short: "Run openbooks as an MCP server for AI agents.",
	Long: `Starts an MCP (Model Context Protocol) server exposing tools for:
  - search_books   Search IRC for ebooks
  - download_book  Download a specific book
  - list_servers   List available IRC download servers
  - list_library   List books already on disk

Use --port to expose via HTTP/StreamableHTTP (compatible with Hermes and other MCP clients).
Without --port the server speaks MCP over stdio (compatible with Claude Desktop).
Use --mock to test without a real IRC connection.`,
	RunE: func(cmd *cobra.Command, args []string) error {
		logger := obsmcp.DefaultLogger()

		cfg := obsmcp.ServerConfig{
			Mock: mcpMock,
			Port: mcpPort,
			Host: mcpHost,
			IRC: obsmcp.Config{
				UserName:    globalFlags.UserName,
				UserAgent:   globalFlags.UserAgent,
				Server:      globalFlags.Server,
				EnableTLS:   globalFlags.EnableTLS,
				SearchBot:   globalFlags.SearchBot,
				DownloadDir: mcpDir,
				Formats:     mcpFormats,
				Log:         logger,
			},
		}

		if !mcpMock && cfg.IRC.UserName == "" {
			log.Fatal("--name is required (unless using --mock)")
		}

		ctx, cancel := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
		defer cancel()

		return obsmcp.Start(ctx, cfg)
	},
}

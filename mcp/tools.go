package mcp

import (
	"context"
	"encoding/json"
	"fmt"

	"github.com/evan-buss/openbooks/core"
	"github.com/mark3labs/mcp-go/mcp"
	"github.com/mark3labs/mcp-go/server"
)

// bookSource is the interface satisfied by both Session and MockSession.
type bookSource interface {
	SearchBooks(ctx context.Context, query string) ([]core.BookDetail, []core.ParseError, error)
	DownloadBook(ctx context.Context, downloadString string) (string, error)
	Servers() []string
	ListLibrary() ([]LibraryBook, error)
	isTrustedServer(server string) bool
	Close()
}

// searchResult is what we return to the LLM for each book found.
type searchResult struct {
	Server        string `json:"server"`
	Author        string `json:"author"`
	Title         string `json:"title"`
	Format        string `json:"format"`
	Size          string `json:"size"`
	Trusted       bool   `json:"trusted_server"`
	DownloadString string `json:"download_string"`
}

func registerTools(s *server.MCPServer, src bookSource) {
	s.AddTool(
		mcp.NewTool("search_books",
			mcp.WithDescription(`Search for ebooks on IRC. This call is synchronous and may take up to 60 seconds while waiting for the IRC bot to respond — this is normal, please wait for the result.

Returns a list of results filtered to epub format (zero-size entries excluded). Each result includes server name, trusted_server flag, author, title, format, size, and a download_string. Use these signals to rank results: prefer trusted servers, reasonable file sizes, and the closest title match. Pass the chosen download_string to download_book.`),
			mcp.WithString("query",
				mcp.Required(),
				mcp.Description("The search query, e.g. 'Dune Frank Herbert' or 'Foundation Asimov'"),
			),
		),
		searchBooksHandler(src),
	)

	s.AddTool(
		mcp.NewTool("download_book",
			mcp.WithDescription("Download a specific book using the download_string returned by search_books. Prefer results from trusted servers with a reasonable file size (not suspiciously small)."),
			mcp.WithString("download_string",
				mcp.Required(),
				mcp.Description("The download_string field from a search_books result, e.g. '!ThrawnBot Frank Herbert - Dune.epub'"),
			),
		),
		downloadBookHandler(src),
	)

	s.AddTool(
		mcp.NewTool("list_servers",
			mcp.WithDescription("List the currently available IRC download servers. Trusted/elevated servers are preferred for downloads."),
		),
		listServersHandler(src),
	)

	s.AddTool(
		mcp.NewTool("list_library",
			mcp.WithDescription("List ebooks already downloaded to the local library."),
		),
		listLibraryHandler(src),
	)
}

func searchBooksHandler(src bookSource) server.ToolHandlerFunc {
	return func(ctx context.Context, req mcp.CallToolRequest) (*mcp.CallToolResult, error) {
		query, err := req.RequireString("query")
		if err != nil {
			return mcp.NewToolResultError(err.Error()), nil
		}

		books, parseErrs, err := src.SearchBooks(ctx, query)
		if err != nil {
			return mcp.NewToolResultError(fmt.Sprintf("search failed: %v", err)), nil
		}

		if len(books) == 0 {
			msg := fmt.Sprintf("No results found for %q.", query)
			if len(parseErrs) > 0 {
				msg += fmt.Sprintf(" (%d lines could not be parsed)", len(parseErrs))
			}
			return mcp.NewToolResultText(msg), nil
		}

		results := make([]searchResult, len(books))
		for i, b := range books {
			results[i] = searchResult{
				Server:         b.Server,
				Author:         b.Author,
				Title:          b.Title,
				Format:         b.Format,
				Size:           b.Size,
				Trusted:        src.isTrustedServer(b.Server),
				DownloadString: b.Full,
			}
		}

		data, _ := json.MarshalIndent(results, "", "  ")
		summary := fmt.Sprintf("Found %d result(s)", len(results))
		if len(parseErrs) > 0 {
			summary += fmt.Sprintf(" (%d unparseable lines omitted)", len(parseErrs))
		}
		summary += ":\n" + string(data)

		return mcp.NewToolResultText(summary), nil
	}
}

func downloadBookHandler(src bookSource) server.ToolHandlerFunc {
	return func(ctx context.Context, req mcp.CallToolRequest) (*mcp.CallToolResult, error) {
		dlStr, err := req.RequireString("download_string")
		if err != nil {
			return mcp.NewToolResultError(err.Error()), nil
		}

		path, err := src.DownloadBook(ctx, dlStr)
		if err != nil {
			return mcp.NewToolResultError(fmt.Sprintf("download failed: %v", err)), nil
		}

		return mcp.NewToolResultText(fmt.Sprintf("Downloaded successfully.\nSaved to: %s", path)), nil
	}
}

func listServersHandler(src bookSource) server.ToolHandlerFunc {
	return func(ctx context.Context, _ mcp.CallToolRequest) (*mcp.CallToolResult, error) {
		servers := src.Servers()
		if len(servers) == 0 {
			return mcp.NewToolResultText("No servers available yet. The server list is populated after joining IRC — try again shortly."), nil
		}
		data, _ := json.MarshalIndent(servers, "", "  ")
		return mcp.NewToolResultText(string(data)), nil
	}
}

func listLibraryHandler(src bookSource) server.ToolHandlerFunc {
	return func(ctx context.Context, _ mcp.CallToolRequest) (*mcp.CallToolResult, error) {
		books, err := src.ListLibrary()
		if err != nil {
			return mcp.NewToolResultError(fmt.Sprintf("library read failed: %v", err)), nil
		}
		if len(books) == 0 {
			return mcp.NewToolResultText("Library is empty."), nil
		}
		data, _ := json.MarshalIndent(books, "", "  ")
		return mcp.NewToolResultText(string(data)), nil
	}
}

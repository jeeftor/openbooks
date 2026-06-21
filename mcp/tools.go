package mcp

import (
	"context"
	"encoding/json"
	"fmt"
	"regexp"
	"sort"
	"strings"

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

// searchResponse is the top-level MCP search response. Servers are hoisted to a
// top-level array so each book can reference them by index instead of repeating
// the name. All results are pre-filtered to online/elevated IRC servers only.
type searchResponse struct {
	Servers []string     `json:"servers"`
	Books   []bookResult `json:"books"`
}

// bookResult is a single deduplicated book entry.
// s    = index into searchResponse.Servers
// dl   = download string to pass to download_book
// copies = number of sources collapsed (omitted when only 1 source found)
type bookResult struct {
	ServerIdx int    `json:"s"`
	Author    string `json:"author"`
	Title     string `json:"title"`
	Size      string `json:"size"`
	DL        string `json:"dl"`
	Copies    int    `json:"copies,omitempty"`
}

// reVariants strips parenthetical/bracketed segments: "(retail)", "[epub]", "(v1.2)", etc.
var reVariants = regexp.MustCompile(`\s*[\(\[][^\)\]]*[\)\]]`)

// normalizeAuthor handles "Last, First" vs "First Last" by stripping punctuation,
// splitting into words, sorting alphabetically, then joining. Both forms produce
// the same key.
func normalizeAuthor(s string) string {
	s = strings.ToLower(s)
	var b strings.Builder
	for _, r := range s {
		if (r >= 'a' && r <= 'z') || r == ' ' {
			b.WriteRune(r)
		}
	}
	words := strings.Fields(b.String())
	sort.Strings(words)
	return strings.Join(words, "")
}

// normalizeTitle strips series prefixes, common variant suffixes, and
// non-alphanumeric characters for grouping comparison.
func normalizeTitle(s string) string {
	s = strings.ToLower(s)
	// Remove bracketed/parenthetical segments: "[Series 01]", "(retail)", "(lrf)", etc.
	s = reVariants.ReplaceAllString(s, "")
	// Strip leading " - " left behind after series bracket removal.
	s = strings.TrimPrefix(strings.TrimSpace(s), "- ")
	s = strings.TrimSpace(s)
	// Remove all non-alphanumeric characters.
	var b strings.Builder
	for _, r := range s {
		if (r >= 'a' && r <= 'z') || (r >= '0' && r <= '9') {
			b.WriteRune(r)
		}
	}
	return b.String()
}

// parseSizeBytes converts IRC size strings to a float64 for comparison.
// Bare numbers without a unit (e.g. Firebound's "148.10") are treated as KB.
func parseSizeBytes(s string) float64 {
	s = strings.TrimSpace(s)
	lower := strings.ToLower(s)
	var num float64
	fmt.Sscanf(s, "%f", &num)
	switch {
	case strings.Contains(lower, "gb"):
		return num * 1024 * 1024 * 1024
	case strings.Contains(lower, "mb"):
		return num * 1024 * 1024
	case strings.Contains(lower, "kb"):
		return num * 1024
	default:
		return num * 1024 // bare numbers treated as KB
	}
}

// buildSearchResponse filters to trusted-server epub results, groups by
// normalized author+title, picks the best representative per group, and
// builds the token-efficient response structure.
func buildSearchResponse(books []core.BookDetail, isTrusted func(string) bool) searchResponse {
	// Filter: epub format from trusted servers only.
	filtered := books[:0]
	for _, b := range books {
		if strings.EqualFold(b.Format, "epub") && isTrusted(b.Server) {
			filtered = append(filtered, b)
		}
	}
	books = filtered

	if len(books) == 0 {
		return searchResponse{Servers: []string{}, Books: []bookResult{}}
	}

	// Group by normalized author + title.
	type group struct {
		rep    core.BookDetail
		copies int
	}
	used := make([]bool, len(books))
	groups := []group{}

	for i := range books {
		if used[i] {
			continue
		}
		used[i] = true
		normA := normalizeAuthor(books[i].Author)
		normT := normalizeTitle(books[i].Title)
		g := group{rep: books[i], copies: 1}

		for j := i + 1; j < len(books); j++ {
			if used[j] {
				continue
			}
			if normalizeAuthor(books[j].Author) != normA || normalizeTitle(books[j].Title) != normT {
				continue
			}
			used[j] = true
			g.copies++
			// Prefer larger file size among trusted servers (all entries here are trusted).
			if parseSizeBytes(books[j].Size) > parseSizeBytes(g.rep.Size) {
				g.rep = books[j]
			}
		}
		groups = append(groups, g)
	}

	// Build the server index from representative servers only.
	serverIdx := map[string]int{}
	servers := []string{}
	for _, g := range groups {
		srv := g.rep.Server
		if _, ok := serverIdx[srv]; !ok {
			serverIdx[srv] = len(servers)
			servers = append(servers, srv)
		}
	}

	bookResults := make([]bookResult, len(groups))
	for i, g := range groups {
		copies := 0
		if g.copies > 1 {
			copies = g.copies
		}
		bookResults[i] = bookResult{
			ServerIdx: serverIdx[g.rep.Server],
			Author:    g.rep.Author,
			Title:     g.rep.Title,
			Size:      g.rep.Size,
			DL:        g.rep.Full,
			Copies:    copies,
		}
	}

	return searchResponse{
		Servers: servers,
		Books:   bookResults,
	}
}

func registerTools(s *server.MCPServer, src bookSource) {
	s.AddTool(
		mcp.NewTool("search_books",
			mcp.WithDescription(`Search for ebooks on IRC. Synchronous — may take up to 60 seconds.

Returns only epub results from trusted servers, deduplicated by title. Response fields:
- servers[]: server names used as representatives
- trusted[]: indexes into servers[] that are trusted/elevated (all entries here, since untrusted are filtered out)
- books[]: one entry per unique title with fields: s (index into servers[]), author, title, size, dl (pass to download_book), copies (sources found, omitted if 1)`),
			mcp.WithString("query",
				mcp.Required(),
				mcp.Description("Search query, e.g. 'Dune Frank Herbert' or 'Foundation Asimov'"),
			),
		),
		searchBooksHandler(src),
	)

	s.AddTool(
		mcp.NewTool("download_book",
			mcp.WithDescription("Download a book using the dl string from search_books."),
			mcp.WithString("download_string",
				mcp.Required(),
				mcp.Description("The dl field from a search_books result"),
			),
		),
		downloadBookHandler(src),
	)

	s.AddTool(
		mcp.NewTool("list_servers",
			mcp.WithDescription("List currently available IRC download servers. Trusted/elevated servers are preferred for downloads."),
		),
		listServersHandler(src),
	)

	s.AddTool(
		mcp.NewTool("list_library",
			mcp.WithDescription("List ebooks already downloaded to the local library. Use query to filter by filename substring."),
			mcp.WithString("query",
				mcp.Description("Optional filename substring filter, e.g. 'Cressida' or 'Dune'"),
			),
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

		resp := buildSearchResponse(books, src.isTrustedServer)

		if len(resp.Books) == 0 {
			return mcp.NewToolResultText(fmt.Sprintf("No epub results from trusted servers found for %q.", query)), nil
		}

		data, _ := json.Marshal(resp)
		summary := fmt.Sprintf("Found %d unique title(s) from trusted servers (%d raw results):\n%s",
			len(resp.Books), len(books), string(data))

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
		data, _ := json.Marshal(servers)
		return mcp.NewToolResultText(string(data)), nil
	}
}

func listLibraryHandler(src bookSource) server.ToolHandlerFunc {
	return func(ctx context.Context, req mcp.CallToolRequest) (*mcp.CallToolResult, error) {
		books, err := src.ListLibrary()
		if err != nil {
			return mcp.NewToolResultError(fmt.Sprintf("library read failed: %v", err)), nil
		}

		var query string
		if args, ok := req.Params.Arguments.(map[string]any); ok {
			query, _ = args["query"].(string)
		}
		if query != "" {
			lower := strings.ToLower(query)
			filtered := books[:0]
			for _, b := range books {
				if strings.Contains(strings.ToLower(b.Name), lower) {
					filtered = append(filtered, b)
				}
			}
			books = filtered
		}

		if len(books) == 0 {
			if query != "" {
				return mcp.NewToolResultText(fmt.Sprintf("No books found matching %q.", query)), nil
			}
			return mcp.NewToolResultText("Library is empty."), nil
		}
		data, _ := json.Marshal(books)
		return mcp.NewToolResultText(string(data)), nil
	}
}

package mcp

import (
	"context"
	"encoding/json"
	"fmt"
	"log/slog"
	"regexp"
	"sort"
	"strings"
	"time"

	"github.com/evan-buss/openbooks/core"
	"github.com/evan-buss/openbooks/staging"
	"github.com/mark3labs/mcp-go/mcp"
	"github.com/mark3labs/mcp-go/server"
)

// bookSource is the interface satisfied by both Session and MockSession.
type bookSource interface {
	SearchBooks(ctx context.Context, query string) ([]core.BookDetail, []core.ParseError, error)
	DownloadBook(ctx context.Context, downloadString string) (*staging.StagedBook, error)
	// DownloadStarted returns a channel that is signaled when the DCC file
	// transfer begins. It is non-blocking on the receive side — the caller
	// should select on it with a default or timeout. The channel is buffered
	// (capacity 1) and reused across downloads; drain before each download.
	DownloadStarted() <-chan struct{}
	ConfirmBook(stagedID string, choice staging.Choice) (string, error)
	ListStaged() []*staging.StagedBook
	DiscardStaged(stagedID string) error
	Servers() []string
	ListLibrary() ([]LibraryBook, error)
	isTrustedServer(server string) bool
	// SetLastSearch caches the full deduplicated search response from the most
	// recent search_books call so list_search_results can return it without
	// re-querying IRC.
	SetLastSearch(query string, resp searchResponse)
	// LastSearch returns the cached query and full response from the most
	// recent search_books call. ok is false if no search has been performed.
	LastSearch() (query string, resp searchResponse, ok bool)
	// Logger returns the session's slog logger for tool-layer logging.
	Logger() *slog.Logger
	Close()
}

// searchResponse is the top-level MCP search response. Servers are hoisted to a
// top-level array so each book can reference them by index instead of repeating
// the name. All results are pre-filtered to online/elevated IRC servers only.
//
// When returned from search_books, Books may contain only the top-N ranked
// results; Total holds the full deduplicated count and Truncated indicates
// whether more are available via list_search_results.
type searchResponse struct {
	Servers   []string     `json:"servers"`
	Books     []bookResult `json:"books"`
	Total     int          `json:"total"`
	Truncated bool         `json:"truncated,omitempty"`
}

// topNSearchResults is the maximum number of ranked results search_books
// returns inline. The full set is available via list_search_results.
const topNSearchResults = 10

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

// stagedBookResponse is the agent-facing view of a staged book. It deliberately
// omits the absolute on-disk path and the base64 cover (which would bloat the
// response and is not useful to a text agent). The agent presents `metadata`
// and `options` to the user, then calls confirm_book with `staged_id`.
type stagedBookResponse struct {
	StagedID     string             `json:"staged_id"`
	IRCFilename  string             `json:"irc_filename"`
	Metadata     *core.EPUBMetadata `json:"metadata,omitempty"`
	Options      []staging.Option   `json:"options"`
	ReplaceSpace string             `json:"replace_space,omitempty"`
}

func newStagedBookResponse(b *staging.StagedBook) stagedBookResponse {
	return stagedBookResponse{
		StagedID:     b.ID,
		IRCFilename:  b.IRCFilename,
		Metadata:     b.Metadata,
		Options:      b.Options,
		ReplaceSpace: b.ReplaceSpace,
	}
}

// reVariants strips parenthetical/bracketed segments: "(retail)", "[epub]", "(v1.2)", etc.
var reVariants = regexp.MustCompile(`\s*[\(\[][^\)\]]*[\)\]]`)

// keepRunes returns s filtered to only the runes for which allow returns true.
func keepRunes(s string, allow func(rune) bool) string {
	var b strings.Builder
	for _, r := range s {
		if allow(r) {
			b.WriteRune(r)
		}
	}
	return b.String()
}

// normalizeAuthor handles "Last, First" vs "First Last" by stripping punctuation,
// splitting into words, sorting alphabetically, then joining. Both forms produce
// the same key.
func normalizeAuthor(s string) string {
	s = strings.ToLower(s)
	s = keepRunes(s, func(r rune) bool { return (r >= 'a' && r <= 'z') || r == ' ' })
	words := strings.Fields(s)
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
	return keepRunes(s, func(r rune) bool { return (r >= 'a' && r <= 'z') || (r >= '0' && r <= '9') })
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
		Total:   len(bookResults),
	}
}

// rankBooks sorts book results by relevance to the query, descending. Ties
// preserve the original order (stable sort). Scoring:
//   - All query words appear in title:  +5
//   - Any query word appears in title:  +3 (only if not all)
//   - All query words appear in author: +5
//   - Any query word appears in author: +3 (only if not all)
//   - Each additional copy (source):    +2
//   - File size in MB:                  +1 per MB
func rankBooks(books []bookResult, query string) []bookResult {
	if len(books) <= 1 {
		return books
	}
	queryWords := strings.Fields(strings.ToLower(query))
	scores := make([]float64, len(books))
	for i, b := range books {
		scores[i] = scoreBook(b, queryWords)
	}
	// Stable sort descending by score.
	idx := make([]int, len(books))
	for i := range idx {
		idx[i] = i
	}
	sort.SliceStable(idx, func(a, c int) bool {
		return scores[idx[a]] > scores[idx[c]]
	})
	out := make([]bookResult, len(books))
	for i, j := range idx {
		out[i] = books[j]
	}
	return out
}

func scoreBook(b bookResult, queryWords []string) float64 {
	titleLower := strings.ToLower(b.Title)
	authorLower := strings.ToLower(b.Author)

	var score float64
	allTitle, anyTitle := true, false
	allAuthor, anyAuthor := true, false
	for _, w := range queryWords {
		inTitle := strings.Contains(titleLower, w)
		inAuthor := strings.Contains(authorLower, w)
		if !inTitle {
			allTitle = false
		} else {
			anyTitle = true
		}
		if !inAuthor {
			allAuthor = false
		} else {
			anyAuthor = true
		}
	}
	if len(queryWords) == 0 {
		allTitle, allAuthor = false, false
	}
	if allTitle {
		score += 5
	} else if anyTitle {
		score += 3
	}
	if allAuthor {
		score += 5
	} else if anyAuthor {
		score += 3
	}
	if b.Copies > 1 {
		score += float64(b.Copies-1) * 2
	}
	score += parseSizeBytes(b.Size) / (1024 * 1024) // MB
	return score
}

func registerTools(s *server.MCPServer, src bookSource) {
	s.AddTool(
		mcp.NewTool("search_books",
			mcp.WithDescription(`Search for ebooks on IRC. Synchronous — may take up to 60 seconds.

Returns only epub results from online servers, deduplicated by title, ranked by relevance to the query. Response fields:
- servers[]: server names (representatives for each returned book)
- books[]: top matches, each with fields: s (index into servers[]), author, title, size, dl (pass to download_book), copies (sources found, omitted if 1)
- total: total number of unique titles found
- truncated: true if books[] was truncated (more available via list_search_results)

Present the top results to the user. If none are what they want and truncated is true, call list_search_results to retrieve the full set.`),
			mcp.WithString("query",
				mcp.Required(),
				mcp.Description("Search query, e.g. 'Dune Frank Herbert' or 'Foundation Asimov'"),
			),
		),
		searchBooksHandler(src),
	)

	s.AddTool(
		mcp.NewTool("list_search_results",
			mcp.WithDescription(`Return the full result set from the most recent search_books call.

Use this when search_books returned a truncated list (truncated=true) and the user wants to see all matches. Returns the same servers[]/books[] structure but with every deduplicated result, sorted by relevance.`),
		),
		listSearchResultsHandler(src),
	)

	s.AddTool(
		mcp.NewTool("download_book",
			mcp.WithDescription(`Download a book using the dl string from search_books.

This is the FIRST step of a two-step flow. The book is downloaded, post-processed (cleaned), and its EPUB metadata (author/title/series/series_index) is extracted, but it is NOT yet moved to the final library location. Instead it is held in a staging area and the response carries:
- staged_id: pass this to confirm_book (or discard_staged) afterwards
- irc_filename: the raw filename from IRC
- metadata: extracted Author/Title/Series/SeriesIndex (may be empty/missing)
- options[]: naming choices, each with id, label, preview, isOrganized

The server sends progress notifications during the download (DCC transfer started, post-processing, metadata extraction). These are informational — the tool call blocks until the full flow completes.

You MUST present the metadata to the user and ask whether the author/title/series are correct before saving. Collect any corrections, choose an option id from options[] (or "custom"), then call confirm_book with the staged_id and the (possibly edited) metadata. If the user does not want the book, call discard_staged.`),
			mcp.WithString("download_string",
				mcp.Required(),
				mcp.Description("The dl field from a search_books result"),
			),
		),
		downloadBookHandler(s, src),
	)

	s.AddTool(
		mcp.NewTool("confirm_book",
			mcp.WithDescription(`Save a staged book to the library. This is the SECOND step after download_book.

Pass the staged_id from download_book, the chosen option_id from the options[] list (e.g. "keep", "title", "author-title-flat", "organized", "series"), and the metadata fields the user confirmed or corrected. Set rewrite_metadata=true to also patch the EPUB's internal OPF metadata (dc:title, dc:creator, calibre:series, calibre:series_index) to match.

If option_id is "custom", provide custom_name (a relative path/filename under the download dir). file_name overrides just the leaf filename for the structured options.

Returns the final path relative to the download directory.`),
			mcp.WithString("staged_id",
				mcp.Required(),
				mcp.Description("staged_id returned by download_book"),
			),
			mcp.WithString("option_id",
				mcp.Required(),
				mcp.Description(`One of the option "id" values from download_book's options[], or "custom"`),
			),
			mcp.WithString("author",
				mcp.Description("Confirmed/edited author. Falls back to extracted metadata when empty."),
			),
			mcp.WithString("title",
				mcp.Description("Confirmed/edited title. Falls back to extracted metadata when empty."),
			),
			mcp.WithString("series",
				mcp.Description("Confirmed/edited series name. Omit if the book is not part of a series."),
			),
			mcp.WithString("series_index",
				mcp.Description("Confirmed/edited series index, e.g. '1' or '2.5'. Omit if not part of a series."),
			),
			mcp.WithString("file_name",
				mcp.Description("Optional override for the leaf filename (extension auto-appended if missing)."),
			),
			mcp.WithString("custom_name",
				mcp.Description("Used only when option_id is 'custom': a relative path/filename under the download dir."),
			),
			mcp.WithBoolean("rewrite_metadata",
				mcp.Description("If true, patch the EPUB's internal OPF metadata to match the confirmed fields."),
			),
		),
		confirmBookHandler(src),
	)

	s.AddTool(
		mcp.NewTool("list_staged",
			mcp.WithDescription("List books downloaded via download_book that are still awaiting confirmation. Each entry has a staged_id, irc_filename, metadata, and options — use confirm_book or discard_staged to resolve them."),
		),
		listStagedHandler(src),
	)

	s.AddTool(
		mcp.NewTool("discard_staged",
			mcp.WithDescription("Permanently delete a staged book (the downloaded file and its staging entry) without saving it to the library. Use when the user decides not to keep a book returned by download_book."),
			mcp.WithString("staged_id",
				mcp.Required(),
				mcp.Description("staged_id returned by download_book"),
			),
		),
		discardStagedHandler(src),
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
			toolLogError(src, "search_books", time.Now(), err)
			return mcp.NewToolResultError(err.Error()), nil
		}
		_, start := toolLog(src, "search_books", "query", query)

		books, parseErrs, err := src.SearchBooks(ctx, query)
		if err != nil {
			toolLogError(src, "search_books", start, err, "query", query)
			return mcp.NewToolResultError(fmt.Sprintf("search failed: %v", err)), nil
		}

		if len(books) == 0 {
			msg := fmt.Sprintf("No results found for %q.", query)
			if len(parseErrs) > 0 {
				msg += fmt.Sprintf(" (%d lines could not be parsed)", len(parseErrs))
			}
			toolLogDone(src, "search_books", start, "query", query, "results", 0)
			return mcp.NewToolResultText(msg), nil
		}

		full := buildSearchResponse(books, src.isTrustedServer)

		if len(full.Books) == 0 {
			toolLogDone(src, "search_books", start, "query", query, "results", 0, "raw", len(books))
			return mcp.NewToolResultText(fmt.Sprintf("No epub results from trusted servers found for %q.", query)), nil
		}

		// Rank by relevance and cache the full set.
		full.Books = rankBooks(full.Books, query)
		src.SetLastSearch(query, full)

		// Build the truncated top-N view for the inline response.
		top := full
		if len(top.Books) > topNSearchResults {
			top.Books = append([]bookResult(nil), full.Books[:topNSearchResults]...)
			top.Truncated = true
		}
		top.Total = len(full.Books)

		data, _ := json.Marshal(top)
		summary := fmt.Sprintf("Found %d unique title(s) from trusted servers (%d raw results). Showing top %d by relevance:\n%s",
			top.Total, len(books), len(top.Books), string(data))
		if top.Truncated {
			summary += "\nMore results available — call list_search_results to see all."
		}

		toolLogDone(src, "search_books", start, "query", query, "total", top.Total, "shown", len(top.Books), "truncated", top.Truncated, "raw", len(books))
		return mcp.NewToolResultText(summary), nil
	}
}

func listSearchResultsHandler(src bookSource) server.ToolHandlerFunc {
	return func(ctx context.Context, _ mcp.CallToolRequest) (*mcp.CallToolResult, error) {
		_, start := toolLog(src, "list_search_results")
		query, resp, ok := src.LastSearch()
		if !ok {
			toolLogDone(src, "list_search_results", start, "cached", false)
			return mcp.NewToolResultText("No search has been performed yet. Call search_books first."), nil
		}
		data, _ := json.Marshal(resp)
		summary := fmt.Sprintf("Full results for %q (%d unique titles):\n%s", query, resp.Total, string(data))
		toolLogDone(src, "list_search_results", start, "query", query, "total", resp.Total)
		return mcp.NewToolResultText(summary), nil
	}
}

func downloadBookHandler(s *server.MCPServer, src bookSource) server.ToolHandlerFunc {
	return func(ctx context.Context, req mcp.CallToolRequest) (*mcp.CallToolResult, error) {
		dlStr, err := req.RequireString("download_string")
		if err != nil {
			toolLogError(src, "download_book", time.Now(), err)
			return mcp.NewToolResultError(err.Error()), nil
		}
		_, start := toolLog(src, "download_book", "dl", dlStr)

		// Drain any stale signal from a previous download.
		select {
		case <-src.DownloadStarted():
		default:
		}

		// Spawn a goroutine that waits for the DCC transfer to start, then
		// sends a progress notification to the client. The notification is
		// best-effort — if the client doesn't support it or the channel is
		// blocked, we silently move on.
		go func() {
			select {
			case <-src.DownloadStarted():
				_ = s.SendNotificationToClient(ctx, "notifications/message", map[string]any{
					"level":  "info",
					"logger": "download",
					"data":   "DCC transfer started — downloading book from IRC server...",
				})
			case <-ctx.Done():
			}
		}()

		book, err := src.DownloadBook(ctx, dlStr)
		if err != nil {
			toolLogError(src, "download_book", start, err, "dl", dlStr)
			return mcp.NewToolResultError(fmt.Sprintf("download failed: %v", err)), nil
		}

		// Notify that post-processing and metadata extraction are complete.
		_ = s.SendNotificationToClient(ctx, "notifications/message", map[string]any{
			"level":  "info",
			"logger": "download",
			"data":   "Download complete. Post-processed and metadata extracted — awaiting user confirmation.",
		})

		resp := newStagedBookResponse(book)
		data, _ := json.Marshal(resp)
		summary := fmt.Sprintf(
			"Downloaded and staged. Present the metadata to the user and confirm author/title/series before saving.\n"+
				"Call confirm_book with staged_id=%q (or discard_staged to cancel).\n%s",
			book.ID, string(data),
		)
		toolLogDone(src, "download_book", start, "staged_id", book.ID, "irc_filename", book.IRCFilename)
		return mcp.NewToolResultText(summary), nil
	}
}

func confirmBookHandler(src bookSource) server.ToolHandlerFunc {
	return func(ctx context.Context, req mcp.CallToolRequest) (*mcp.CallToolResult, error) {
		stagedID, err := req.RequireString("staged_id")
		if err != nil {
			toolLogError(src, "confirm_book", time.Now(), err)
			return mcp.NewToolResultError(err.Error()), nil
		}
		optionID, err := req.RequireString("option_id")
		if err != nil {
			toolLogError(src, "confirm_book", time.Now(), err, "staged_id", stagedID)
			return mcp.NewToolResultError(err.Error()), nil
		}
		_, start := toolLog(src, "confirm_book", "staged_id", stagedID, "option", optionID)

		args := toolArgs(req)
		choice := staging.Choice{
			OptionID:        optionID,
			CustomName:      args["custom_name"],
			FileName:        args["file_name"],
			Author:          args["author"],
			Title:           args["title"],
			Series:          args["series"],
			SeriesIndex:     args["series_index"],
			RewriteMetadata: args["rewrite_metadata"] == "true",
		}

		relPath, err := src.ConfirmBook(stagedID, choice)
		if err != nil {
			toolLogError(src, "confirm_book", start, err, "staged_id", stagedID, "option", optionID)
			return mcp.NewToolResultError(fmt.Sprintf("confirm failed: %v", err)), nil
		}

		toolLogDone(src, "confirm_book", start, "staged_id", stagedID, "option", optionID, "path", relPath, "rewrite", choice.RewriteMetadata)
		return mcp.NewToolResultText(fmt.Sprintf("Saved to library: %s", relPath)), nil
	}
}

func listStagedHandler(src bookSource) server.ToolHandlerFunc {
	return func(ctx context.Context, _ mcp.CallToolRequest) (*mcp.CallToolResult, error) {
		_, start := toolLog(src, "list_staged")
		books := src.ListStaged()
		if len(books) == 0 {
			toolLogDone(src, "list_staged", start, "count", 0)
			return mcp.NewToolResultText("No staged books awaiting confirmation."), nil
		}
		out := make([]stagedBookResponse, len(books))
		for i, b := range books {
			out[i] = newStagedBookResponse(b)
		}
		data, _ := json.Marshal(out)
		toolLogDone(src, "list_staged", start, "count", len(books))
		return mcp.NewToolResultText(string(data)), nil
	}
}

func discardStagedHandler(src bookSource) server.ToolHandlerFunc {
	return func(ctx context.Context, req mcp.CallToolRequest) (*mcp.CallToolResult, error) {
		stagedID, err := req.RequireString("staged_id")
		if err != nil {
			toolLogError(src, "discard_staged", time.Now(), err)
			return mcp.NewToolResultError(err.Error()), nil
		}
		_, start := toolLog(src, "discard_staged", "staged_id", stagedID)
		if err := src.DiscardStaged(stagedID); err != nil {
			toolLogError(src, "discard_staged", start, err, "staged_id", stagedID)
			return mcp.NewToolResultError(fmt.Sprintf("discard failed: %v", err)), nil
		}
		toolLogDone(src, "discard_staged", start, "staged_id", stagedID)
		return mcp.NewToolResultText(fmt.Sprintf("Discarded staged book %s.", stagedID)), nil
	}
}

// toolArgs returns the request arguments as a string map, coercing values to
// strings. Booleans come through as "true"/"false".
func toolArgs(req mcp.CallToolRequest) map[string]string {
	out := map[string]string{}
	args, ok := req.Params.Arguments.(map[string]any)
	if !ok {
		return out
	}
	for k, v := range args {
		switch t := v.(type) {
		case string:
			out[k] = t
		case bool:
			if t {
				out[k] = "true"
			} else {
				out[k] = "false"
			}
		default:
			if v != nil {
				out[k] = fmt.Sprintf("%v", v)
			}
		}
	}
	return out
}

func listServersHandler(src bookSource) server.ToolHandlerFunc {
	return func(ctx context.Context, _ mcp.CallToolRequest) (*mcp.CallToolResult, error) {
		_, start := toolLog(src, "list_servers")
		servers := src.Servers()
		if len(servers) == 0 {
			toolLogDone(src, "list_servers", start, "count", 0)
			return mcp.NewToolResultText("No servers available yet. The server list is populated after joining IRC — try again shortly."), nil
		}
		data, _ := json.Marshal(servers)
		toolLogDone(src, "list_servers", start, "count", len(servers))
		return mcp.NewToolResultText(string(data)), nil
	}
}

func listLibraryHandler(src bookSource) server.ToolHandlerFunc {
	return func(ctx context.Context, req mcp.CallToolRequest) (*mcp.CallToolResult, error) {
		var query string
		if args, ok := req.Params.Arguments.(map[string]any); ok {
			query, _ = args["query"].(string)
		}
		_, start := toolLog(src, "list_library", "query", query)

		books, err := src.ListLibrary()
		if err != nil {
			toolLogError(src, "list_library", start, err, "query", query)
			return mcp.NewToolResultError(fmt.Sprintf("library read failed: %v", err)), nil
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
			toolLogDone(src, "list_library", start, "count", 0, "query", query)
			if query != "" {
				return mcp.NewToolResultText(fmt.Sprintf("No books found matching %q.", query)), nil
			}
			return mcp.NewToolResultText("Library is empty."), nil
		}
		data, _ := json.Marshal(books)
		toolLogDone(src, "list_library", start, "count", len(books), "query", query)
		return mcp.NewToolResultText(string(data)), nil
	}
}

// toolLog is a small helper for tool-layer entry/exit logging. It returns a
// start time; callers log the entry line themselves and call toolLogDone (or
// toolLogError) on exit. All lines go to the session's slog logger (stderr).
func toolLog(src bookSource, tool string, args ...any) (string, time.Time) {
	start := time.Now()
	src.Logger().Info("mcp tool call", append([]any{"tool", tool}, args...)...)
	return tool, start
}

// toolLogDone logs a successful tool exit with outcome fields and duration.
func toolLogDone(src bookSource, tool string, start time.Time, args ...any) {
	all := append([]any{"tool", tool, "took", time.Since(start).Round(time.Millisecond)}, args...)
	src.Logger().Info("mcp tool ok", all...)
}

// toolLogError logs a failed tool exit with the error and duration.
func toolLogError(src bookSource, tool string, start time.Time, err error, args ...any) {
	all := append([]any{"tool", tool, "took", time.Since(start).Round(time.Millisecond), "err", err}, args...)
	src.Logger().Error("mcp tool error", all...)
}

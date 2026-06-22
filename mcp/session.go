package mcp

import (
	"context"
	"encoding/base64"
	"fmt"
	"log/slog"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"sync"
	"time"

	"github.com/jeeftor/openbooks/core"
	"github.com/jeeftor/openbooks/irc"
	"github.com/jeeftor/openbooks/staging"
	"github.com/google/uuid"
)

// Session holds a persistent IRC connection and bridges async IRC events
// to synchronous MCP tool calls. Only one search and one download may be
// in-flight at a time.
type Session struct {
	irc            *irc.Conn
	searchBot      string
	downloadDir    string
	formats        []string // file formats to accept, e.g. ["epub"]
	postProcessCmd []string // command + args; file path appended automatically
	replaceSpace   string
	log            *slog.Logger
	activityLog    func(level, msg string) // optional hook into host log buffer
	staged         *staging.StagedBookStore

	mu              sync.Mutex // serialises search AND download calls
	searchDone      chan searchOutcome
	downloadDone    chan downloadOutcome
	downloadStarted chan struct{} // signaled when DCC offer arrives

	serversMu  sync.RWMutex
	serverList []string

	searchCacheMu    sync.RWMutex
	lastSearchQuery  string
	lastSearchResp   searchResponse
	lastSearchValid  bool
}

type searchOutcome struct {
	books  []core.BookDetail
	errors []core.ParseError
	err    error
}

type downloadOutcome struct {
	path string
	err  error
}

// Config holds the parameters needed to start an MCP session.
type Config struct {
	UserName       string
	UserAgent      string
	Server         string
	EnableTLS      bool
	SearchBot      string
	DownloadDir    string
	Formats        []string // filter; nil or empty means accept all
	PostProcessCmd []string // command + args; file path appended automatically
	ReplaceSpace   string   // optional character used to replace spaces in filenames
	Log            *slog.Logger
	// ActivityLog is called for key events (search, download) so callers can
	// route them into a shared log buffer (e.g. the web UI's log panel).
	// Optional — if nil, only slog output is produced.
	ActivityLog func(level, msg string)
}

// Connect creates a new Session and connects to IRC.
func Connect(ctx context.Context, cfg Config) (*Session, error) {
	logger := cfg.Log
	if logger == nil {
		logger = slog.New(slog.NewTextHandler(os.Stderr, nil))
	}

	formats := cfg.Formats
	if len(formats) == 0 {
		formats = []string{"epub"}
	}

	conn := irc.New(cfg.UserName, cfg.UserAgent)
	if err := core.Join(conn, cfg.Server, cfg.EnableTLS); err != nil {
		return nil, fmt.Errorf("irc connect: %w", err)
	}

	stagedStore, err := staging.NewStagedBookStore(cfg.DownloadDir)
	if err != nil {
		return nil, fmt.Errorf("staged store: %w", err)
	}

	sess := &Session{
		irc:            conn,
		searchBot:      cfg.SearchBot,
		downloadDir:    cfg.DownloadDir,
		formats:        formats,
		postProcessCmd: cfg.PostProcessCmd,
		replaceSpace:   cfg.ReplaceSpace,
		log:            logger,
		activityLog:    cfg.ActivityLog,
		staged:         stagedStore,
		searchDone:     make(chan searchOutcome, 1),
		downloadDone:   make(chan downloadOutcome, 1),
		downloadStarted: make(chan struct{}, 1),
	}

	handler := sess.buildHandler()
	go core.StartReader(ctx, conn, handler)

	logger.Info("IRC connected", "server", cfg.Server, "user", cfg.UserName)
	return sess, nil
}

// Close disconnects from IRC.
func (s *Session) Close() {
	s.irc.Disconnect()
}

// Logger returns the session's slog logger.
func (s *Session) Logger() *slog.Logger { return s.log }

// Servers returns the current list of known download servers.
func (s *Session) Servers() []string {
	s.serversMu.RLock()
	defer s.serversMu.RUnlock()
	out := make([]string, len(s.serverList))
	copy(out, s.serverList)
	return out
}

// SetLastSearch caches the full deduplicated response from the most recent
// search_books call so list_search_results can return it without re-querying.
func (s *Session) SetLastSearch(query string, resp searchResponse) {
	s.searchCacheMu.Lock()
	defer s.searchCacheMu.Unlock()
	s.lastSearchQuery = query
	s.lastSearchResp = resp
	s.lastSearchValid = true
}

// LastSearch returns the cached query and full response from the most recent
// search_books call. ok is false if no search has been performed.
func (s *Session) LastSearch() (string, searchResponse, bool) {
	s.searchCacheMu.RLock()
	defer s.searchCacheMu.RUnlock()
	return s.lastSearchQuery, s.lastSearchResp, s.lastSearchValid
}

// SearchBooks sends a search query and waits for results, serialising
// concurrent callers so only one search is in-flight at a time.
func (s *Session) SearchBooks(ctx context.Context, query string) ([]core.BookDetail, []core.ParseError, error) {
	s.mu.Lock()
	defer s.mu.Unlock()

	core.SearchBook(s.irc, s.searchBot, query)
	s.log.Info("search sent", "query", query)
	s.logActivity("info", fmt.Sprintf("🤖 MCP search: %q", query))

	select {
	case outcome := <-s.searchDone:
		if outcome.err != nil {
			s.logActivity("error", fmt.Sprintf("🤖 MCP search failed: %v", outcome.err))
			return nil, nil, outcome.err
		}
		filtered := FilterResults(outcome.books, s.formats)
		s.logActivity("info", fmt.Sprintf("🤖 MCP search results: %d found for %q", len(filtered), query))
		return filtered, outcome.errors, nil
	case <-time.After(90 * time.Second):
		s.logActivity("error", fmt.Sprintf("🤖 MCP search timed out: %q", query))
		return nil, nil, fmt.Errorf("search timed out after 90s")
	case <-ctx.Done():
		return nil, nil, ctx.Err()
	}
}

// DownloadStarted returns a channel signaled when the DCC file transfer begins.
func (s *Session) DownloadStarted() <-chan struct{} {
	return s.downloadStarted
}

// DownloadBook sends a DCC download request, waits for the file to land in the
// staging directory, runs the post-processor (if configured), reads EPUB
// metadata, builds rename options, registers the result as a staged book, and
// returns the staged book descriptor. The caller (AI agent) is expected to
// present the metadata to the user for confirmation and then call ConfirmBook.
//
// The returned *staging.StagedBook exposes absolute StagedPath; MCP tool
// handlers must not leak that to agents — use the staged ID instead.
func (s *Session) DownloadBook(ctx context.Context, downloadString string) (*staging.StagedBook, error) {
	s.mu.Lock()
	defer s.mu.Unlock()

	if err := staging.EnsureStagingDir(s.downloadDir); err != nil {
		return nil, fmt.Errorf("staging dir: %w", err)
	}

	core.DownloadBook(s.irc, downloadString)
	s.log.Info("download sent", "string", downloadString)
	s.logActivity("info", fmt.Sprintf("🤖 MCP download: %s", downloadString))

	var outcome downloadOutcome
	select {
	case outcome = <-s.downloadDone:
	case <-time.After(3 * time.Minute):
		s.logActivity("error", "🤖 MCP download timed out")
		return nil, fmt.Errorf("download timed out after 3m")
	case <-ctx.Done():
		return nil, ctx.Err()
	}

	if outcome.err != nil {
		s.logActivity("error", fmt.Sprintf("🤖 MCP download failed: %v", outcome.err))
		return nil, outcome.err
	}

	extractedPath := outcome.path
	ircFilename := filepath.Base(extractedPath)
	s.logActivity("info", fmt.Sprintf("🤖 MCP download complete: %s", ircFilename))

	// 1. Run post-processor (clean).
	if len(s.postProcessCmd) > 0 {
		if err := runPostProcess(s.postProcessCmd, extractedPath); err != nil {
			s.logActivity("error", fmt.Sprintf("🤖 MCP post-process failed: %v", err))
		} else {
			s.logActivity("info", fmt.Sprintf("🤖 MCP post-process complete: %s", s.postProcessCmd[0]))
		}
	}

	// 2. Read EPUB metadata + cover.
	var meta *core.EPUBMetadata
	var coverBase64, coverMime string
	if strings.EqualFold(filepath.Ext(extractedPath), ".epub") {
		if m, err := core.ReadEPUBMetadata(extractedPath); err == nil {
			meta = m
		}
		if imgBytes, mime, err := core.ExtractCoverImage(extractedPath); err == nil && imgBytes != nil {
			coverBase64 = base64.StdEncoding.EncodeToString(imgBytes)
			coverMime = mime
		}
	}

	// 3. Build rename options and register as a staged book.
	options := staging.BuildOptions(ircFilename, meta, s.replaceSpace)
	staged := &staging.StagedBook{
		ID:           uuid.New().String(),
		StagedPath:   extractedPath,
		IRCFilename:  ircFilename,
		Metadata:     meta,
		Options:      options,
		ReplaceSpace: s.replaceSpace,
		CoverBase64:  coverBase64,
		CoverMime:    coverMime,
		StagedAt:     time.Now(),
	}
	if err := s.staged.Add(staged); err != nil {
		os.Remove(extractedPath)
		return nil, fmt.Errorf("staging failed: %w", err)
	}

	s.logActivity("info", fmt.Sprintf("🤖 MCP staged: %s (awaiting confirmation)", staged.ID))
	return staged, nil
}

// ConfirmBook applies the caller's rename decision to a staged book: resolves
// the final organised path, moves the file out of staging, optionally rewrites
// the EPUB internal metadata, and removes the book from the staged store.
// Returns the final path relative to the download directory.
func (s *Session) ConfirmBook(stagedID string, choice staging.Choice) (string, error) {
	book, ok, err := s.staged.GetAndRemove(stagedID)
	if err != nil {
		return "", fmt.Errorf("staged store error: %w", err)
	}
	if !ok {
		return "", fmt.Errorf("no staged book with id %q", stagedID)
	}

	finalPath := staging.ResolveFinalPath(s.downloadDir, choice, book.IRCFilename, book.Metadata, book.ReplaceSpace)
	if err := staging.MoveFile(book.StagedPath, finalPath); err != nil {
		return "", fmt.Errorf("move failed: %w", err)
	}

	if choice.RewriteMetadata && strings.EqualFold(filepath.Ext(finalPath), ".epub") {
		if err := staging.RewriteEPUBMetadata(finalPath, choice.Title, choice.Author, choice.Series, choice.SeriesIndex, choice.ClearSeries, choice.ClearSeriesIndex); err != nil {
			s.logActivity("error", fmt.Sprintf("🤖 MCP metadata rewrite failed: %v", err))
		}
	}

	rel, _ := filepath.Rel(s.downloadDir, finalPath)
	s.logActivity("info", fmt.Sprintf("🤖 MCP saved: %s", filepath.ToSlash(rel)))
	return filepath.ToSlash(rel), nil
}

// ListStaged returns all books downloaded via MCP that are awaiting confirmation.
func (s *Session) ListStaged() []*staging.StagedBook {
	return s.staged.All()
}

// DiscardStaged deletes a staged book's file and removes it from the registry.
func (s *Session) DiscardStaged(stagedID string) error {
	book, ok := s.staged.Get(stagedID)
	if !ok {
		return fmt.Errorf("no staged book with id %q", stagedID)
	}
	os.Remove(book.StagedPath)
	return s.staged.Remove(stagedID)
}

// runPostProcess executes the configured post-process command with filePath
// appended as the final argument. Errors are returned but non-fatal to the
// caller — the download still succeeds.
func runPostProcess(cmd []string, filePath string) error {
	if len(cmd) == 0 {
		return nil
	}
	args := append(append([]string{}, cmd[1:]...), filePath)
	out, err := exec.Command(cmd[0], args...).CombinedOutput()
	if err != nil {
		return fmt.Errorf("%s: %w (%s)", cmd[0], err, string(out))
	}
	return nil
}

// buildHandler wires IRC events to the session's result channels.
func (s *Session) buildHandler() core.EventHandler {
	handler := core.EventHandler{}

	handler[core.Ping] = func(serverURL string) {
		s.irc.Pong(serverURL)
	}

	handler[core.Version] = func(line string) {
		core.SendVersionInfo(s.irc, line, "OpenBooks MCP")
	}

	handler[core.ServerList] = func(text string) {
		servers := core.ParseServers(text)
		s.serversMu.Lock()
		s.serverList = servers.ElevatedUsers
		s.serversMu.Unlock()
		s.log.Info("server list updated", "count", len(servers.ElevatedUsers))
	}

	handler[core.SearchResult] = func(text string) {
		path, err := core.DownloadExtractDCCString(s.downloadDir, text, nil)
		if err != nil {
			s.searchDone <- searchOutcome{err: fmt.Errorf("search download: %w", err)}
			return
		}
		books, parseErrs, err := core.ParseSearchFile(path)
		os.Remove(path)
		s.searchDone <- searchOutcome{books: books, errors: parseErrs, err: err}
	}

	handler[core.BookResult] = func(text string) {
		// Signal that the DCC transfer has started (non-blocking — the tool
		// handler may or may not be listening).
		select {
		case s.downloadStarted <- struct{}{}:
		default:
		}
		path, err := core.DownloadExtractDCCString(staging.StagingDir(s.downloadDir), text, nil)
		s.downloadDone <- downloadOutcome{path: path, err: err}
	}

	handler[core.NoResults] = func(_ string) {
		s.searchDone <- searchOutcome{} // empty, no error
	}

	handler[core.BadServer] = func(_ string) {
		s.downloadDone <- downloadOutcome{err: fmt.Errorf("server unavailable")}
	}

	return handler
}

// logActivity emits to the activity log callback if set, and mirrors the
// message to stderr via slog so `docker logs` shows the full MCP narrative.
func (s *Session) logActivity(level, msg string) {
	if s.activityLog != nil {
		s.activityLog(level, msg)
	}
	switch level {
	case "error":
		s.log.Error(msg)
	default:
		s.log.Info(msg)
	}
}

// FilterResults removes books that don't match the format list and drops
// entries with a zero or missing file size (corrupt/empty listings).
func FilterResults(books []core.BookDetail, formats []string) []core.BookDetail {
	out := make([]core.BookDetail, 0, len(books))
	for _, b := range books {
		if isZeroSize(b.Size) {
			continue
		}
		if len(formats) > 0 && !formatMatches(b.Format, formats) {
			continue
		}
		out = append(out, b)
	}
	return out
}

// formatMatches returns true if the book format is in the allowed list.
func formatMatches(format string, formats []string) bool {
	for _, f := range formats {
		if strings.EqualFold(format, f) {
			return true
		}
	}
	return false
}

// isZeroSize returns true for entries that have no meaningful file size.
// IRC result sizes are strings like "1.2 MB", "500 KB", "N/A", "0", "".
func isZeroSize(size string) bool {
	size = strings.TrimSpace(size)
	if size == "" || size == "N/A" || size == "0" || size == "0 B" || size == "0.0 B" {
		return true
	}
	// catch "0 KB", "0.0 MB", etc.
	if strings.HasPrefix(size, "0 ") || strings.HasPrefix(size, "0.0 ") {
		return true
	}
	return false
}

// isTrustedServer returns true if the server name appears in the elevated list.
func (s *Session) isTrustedServer(server string) bool {
	s.serversMu.RLock()
	defer s.serversMu.RUnlock()
	for _, sv := range s.serverList {
		if strings.EqualFold(sv, server) {
			return true
		}
	}
	return false
}

// ListLibrary returns all files in the download directory.
func (s *Session) ListLibrary() ([]LibraryBook, error) {
	var books []LibraryBook
	err := filepath.WalkDir(s.downloadDir, func(p string, d os.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if d.IsDir() {
			if strings.HasPrefix(d.Name(), ".") {
				return filepath.SkipDir
			}
			return nil
		}
		name := d.Name()
		if strings.HasPrefix(name, ".") || filepath.Ext(name) == ".temp" {
			return nil
		}
		info, err := d.Info()
		if err != nil {
			return nil
		}
		rel, _ := filepath.Rel(s.downloadDir, p)
		books = append(books, LibraryBook{
			Name:     name,
			Path:     filepath.ToSlash(rel),
			Modified: info.ModTime(),
			SizeKB:   info.Size() / 1024,
		})
		return nil
	})
	return books, err
}

// LibraryBook is a file already on disk.
type LibraryBook struct {
	Name     string    `json:"name"`
	Path     string    `json:"path"`
	Modified time.Time `json:"modified"`
	SizeKB   int64     `json:"size_kb"`
}

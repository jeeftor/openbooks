package mcp

import (
	"context"
	"fmt"
	"log/slog"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"time"

	"github.com/evan-buss/openbooks/core"
	"github.com/evan-buss/openbooks/irc"
)

// Session holds a persistent IRC connection and bridges async IRC events
// to synchronous MCP tool calls. Only one search and one download may be
// in-flight at a time.
type Session struct {
	irc         *irc.Conn
	searchBot   string
	downloadDir string
	formats     []string // file formats to accept, e.g. ["epub"]
	log         *slog.Logger

	mu           sync.Mutex // serialises search AND download calls
	searchDone   chan searchOutcome
	downloadDone chan downloadOutcome

	serversMu  sync.RWMutex
	serverList []string
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
	UserName    string
	UserAgent   string
	Server      string
	EnableTLS   bool
	SearchBot   string
	DownloadDir string
	Formats     []string // filter; nil or empty means accept all
	Log         *slog.Logger
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

	sess := &Session{
		irc:          conn,
		searchBot:    cfg.SearchBot,
		downloadDir:  cfg.DownloadDir,
		formats:      formats,
		log:          logger,
		searchDone:   make(chan searchOutcome, 1),
		downloadDone: make(chan downloadOutcome, 1),
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

// Servers returns the current list of known download servers.
func (s *Session) Servers() []string {
	s.serversMu.RLock()
	defer s.serversMu.RUnlock()
	out := make([]string, len(s.serverList))
	copy(out, s.serverList)
	return out
}

// SearchBooks sends a search query and waits for results, serialising
// concurrent callers so only one search is in-flight at a time.
func (s *Session) SearchBooks(ctx context.Context, query string) ([]core.BookDetail, []core.ParseError, error) {
	s.mu.Lock()
	defer s.mu.Unlock()

	core.SearchBook(s.irc, s.searchBot, query)
	s.log.Info("search sent", "query", query)

	select {
	case outcome := <-s.searchDone:
		if outcome.err != nil {
			return nil, nil, outcome.err
		}
		filtered := filterByFormat(outcome.books, s.formats)
		return filtered, outcome.errors, nil
	case <-time.After(90 * time.Second):
		return nil, nil, fmt.Errorf("search timed out after 90s")
	case <-ctx.Done():
		return nil, nil, ctx.Err()
	}
}

// DownloadBook sends a DCC download request and waits for the file to land on disk.
func (s *Session) DownloadBook(ctx context.Context, downloadString string) (string, error) {
	s.mu.Lock()
	defer s.mu.Unlock()

	core.DownloadBook(s.irc, downloadString)
	s.log.Info("download sent", "string", downloadString)

	select {
	case outcome := <-s.downloadDone:
		return outcome.path, outcome.err
	case <-time.After(3 * time.Minute):
		return "", fmt.Errorf("download timed out after 3m")
	case <-ctx.Done():
		return "", ctx.Err()
	}
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
		path, err := core.DownloadExtractDCCString(s.downloadDir, text, nil)
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

func filterByFormat(books []core.BookDetail, formats []string) []core.BookDetail {
	if len(formats) == 0 {
		return books
	}
	out := make([]core.BookDetail, 0, len(books))
	for _, b := range books {
		for _, f := range formats {
			if strings.EqualFold(b.Format, f) {
				out = append(out, b)
				break
			}
		}
	}
	return out
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

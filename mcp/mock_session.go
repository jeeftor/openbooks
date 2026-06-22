package mcp

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"sync"
	"time"

	"github.com/evan-buss/openbooks/core"
	"github.com/evan-buss/openbooks/staging"
	"github.com/google/uuid"
)

// MockSession implements the same interface as Session but returns fake data.
// Used for testing without a live IRC connection.
type MockSession struct {
	downloadDir string
	replaceSpace string

	mu     sync.Mutex
	staged map[string]*staging.StagedBook

	searchMu       sync.RWMutex
	lastQuery      string
	lastResp       searchResponse
	lastSearchOk   bool

	downloadStarted chan struct{}
}

func NewMockSession(downloadDir string) *MockSession {
	return &MockSession{
		downloadDir:     downloadDir,
		staged:          make(map[string]*staging.StagedBook),
		downloadStarted: make(chan struct{}, 1),
	}
}

func (m *MockSession) DownloadStarted() <-chan struct{} {
	return m.downloadStarted
}

func (m *MockSession) SearchBooks(_ context.Context, query string) ([]core.BookDetail, []core.ParseError, error) {
	// Simulate a brief delay
	time.Sleep(200 * time.Millisecond)
	return []core.BookDetail{
		{
			Server: "ThrawnBot",
			Author: "Frank Herbert",
			Title:  query + " (mock result)",
			Format: "epub",
			Size:   "1.2 MB",
			Full:   fmt.Sprintf("!ThrawnBot Frank Herbert - %s.epub", query),
		},
		{
			Server: "RegularBot",
			Author: "Frank Herbert",
			Title:  query + " Deluxe Edition (mock result)",
			Format: "epub",
			Size:   "2.4 MB",
			Full:   fmt.Sprintf("!RegularBot Frank Herbert - %s Deluxe.epub", query),
		},
		{
			Server: "EpubWorld",
			Author: "Frank Herbert",
			Title:  query + " Abridged (mock result)",
			Format: "epub",
			Size:   "0.3 MB",
			Full:   fmt.Sprintf("!EpubWorld Frank Herbert - %s Abridged.epub", query),
		},
	}, nil, nil
}

func (m *MockSession) DownloadBook(_ context.Context, downloadString string) (*staging.StagedBook, error) {
	// Signal that the "DCC transfer" has started.
	select {
	case m.downloadStarted <- struct{}{}:
	default:
	}
	time.Sleep(300 * time.Millisecond)

	if err := staging.EnsureStagingDir(m.downloadDir); err != nil {
		return nil, err
	}
	// Write a fake epub file into staging so confirm/discard have something to move.
	stagePath := filepath.Join(staging.StagingDir(m.downloadDir), fmt.Sprintf("mock-%s.epub", uuid.New().String()))
	if err := os.WriteFile(stagePath, []byte("mock epub"), 0644); err != nil {
		return nil, err
	}

	ircFilename := filepath.Base(stagePath)
	meta := &core.EPUBMetadata{
		Author: "Frank Herbert",
		Title:  "Dune (mock)",
	}
	options := staging.BuildOptions(ircFilename, meta, m.replaceSpace)

	book := &staging.StagedBook{
		ID:           uuid.New().String(),
		StagedPath:   stagePath,
		IRCFilename:  ircFilename,
		Metadata:     meta,
		Options:      options,
		ReplaceSpace: m.replaceSpace,
		StagedAt:     time.Now(),
	}

	m.mu.Lock()
	m.staged[book.ID] = book
	m.mu.Unlock()
	return book, nil
}

func (m *MockSession) ConfirmBook(stagedID string, choice staging.Choice) (string, error) {
	m.mu.Lock()
	book, ok := m.staged[stagedID]
	if !ok {
		m.mu.Unlock()
		return "", fmt.Errorf("no staged book with id %q", stagedID)
	}
	delete(m.staged, stagedID)
	m.mu.Unlock()

	finalPath := staging.ResolveFinalPath(m.downloadDir, choice, book.IRCFilename, book.Metadata, book.ReplaceSpace)
	if err := staging.MoveFile(book.StagedPath, finalPath); err != nil {
		return "", err
	}
	rel, _ := filepath.Rel(m.downloadDir, finalPath)
	return filepath.ToSlash(rel), nil
}

func (m *MockSession) ListStaged() []*staging.StagedBook {
	m.mu.Lock()
	defer m.mu.Unlock()
	out := make([]*staging.StagedBook, 0, len(m.staged))
	for _, b := range m.staged {
		out = append(out, b)
	}
	return out
}

func (m *MockSession) DiscardStaged(stagedID string) error {
	m.mu.Lock()
	book, ok := m.staged[stagedID]
	if !ok {
		m.mu.Unlock()
		return fmt.Errorf("no staged book with id %q", stagedID)
	}
	delete(m.staged, stagedID)
	m.mu.Unlock()
	os.Remove(book.StagedPath)
	return nil
}

var mockServers = []string{"ThrawnBot", "EpubWorld", "Alexandria", "MirrorBot"}

func (m *MockSession) Servers() []string {
	out := make([]string, len(mockServers))
	copy(out, mockServers)
	return out
}

func (m *MockSession) SetLastSearch(query string, resp searchResponse) {
	m.searchMu.Lock()
	defer m.searchMu.Unlock()
	m.lastQuery = query
	m.lastResp = resp
	m.lastSearchOk = true
}

func (m *MockSession) LastSearch() (string, searchResponse, bool) {
	m.searchMu.RLock()
	defer m.searchMu.RUnlock()
	return m.lastQuery, m.lastResp, m.lastSearchOk
}

func (m *MockSession) ListLibrary() ([]LibraryBook, error) {
	return []LibraryBook{
		{
			Name:     "Dune.epub",
			Path:     "Dune.epub",
			Modified: time.Now().Add(-24 * time.Hour),
			SizeKB:   1200,
		},
		{
			Name:     "Foundation.epub",
			Path:     "Foundation.epub",
			Modified: time.Now().Add(-48 * time.Hour),
			SizeKB:   980,
		},
	}, nil
}

func (m *MockSession) isTrustedServer(server string) bool {
	for _, s := range mockServers {
		if s == server {
			return true
		}
	}
	return false
}

func (m *MockSession) Close() {}

package mcp

import (
	"context"
	"fmt"
	"time"

	"github.com/evan-buss/openbooks/core"
)

// MockSession implements the same interface as Session but returns fake data.
// Used for testing without a live IRC connection.
type MockSession struct {
	downloadDir string
}

func NewMockSession(downloadDir string) *MockSession {
	return &MockSession{downloadDir: downloadDir}
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

func (m *MockSession) DownloadBook(_ context.Context, downloadString string) (string, error) {
	time.Sleep(300 * time.Millisecond)
	return m.downloadDir + "/mock-download.epub", nil
}

func (m *MockSession) Servers() []string {
	return []string{"ThrawnBot", "EpubWorld", "Alexandria", "MirrorBot"}
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
	for _, s := range m.Servers() {
		if s == server {
			return true
		}
	}
	return false
}

func (m *MockSession) Close() {}

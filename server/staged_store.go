package server

import "github.com/evan-buss/openbooks/staging"

// StagedBook is a book that has been downloaded and post-processed but not yet
// renamed and moved to the final library location.
// Alias of staging.StagedBook so the web server and MCP server share one type.
type StagedBook = staging.StagedBook

// StagedBookStore is a thread-safe, file-backed registry of staged books.
// Alias of staging.StagedBookStore so the web server and MCP server share one store.
type StagedBookStore = staging.StagedBookStore

// newStagedBookStore creates the store and loads any existing data from disk.
func newStagedBookStore(downloadDir string) (*StagedBookStore, error) {
	return staging.NewStagedBookStore(downloadDir)
}

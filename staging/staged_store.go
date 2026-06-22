package staging

import (
	"encoding/json"
	"os"
	"path/filepath"
	"sort"
	"sync"
	"time"

	"github.com/jeeftor/openbooks/core"
)

// StagedBook is a book that has been downloaded and post-processed but not yet
// renamed and moved to the final library location.
type StagedBook struct {
	ID           string             `json:"id"`
	StagedPath   string             `json:"stagedPath"`
	IRCFilename  string             `json:"ircFilename"`
	Metadata     *core.EPUBMetadata `json:"metadata,omitempty"`
	Options      []Option           `json:"options"`
	ReplaceSpace string             `json:"replaceSpace"`
	CoverBase64  string             `json:"coverBase64,omitempty"`
	CoverMime    string             `json:"coverMime,omitempty"`
	StagedAt     time.Time          `json:"stagedAt"`
}

// StagedBookStore is a thread-safe, file-backed registry of staged books.
type StagedBookStore struct {
	mu       sync.RWMutex
	books    map[string]*StagedBook
	filePath string
}

// NewStagedBookStore creates the store and loads any existing data from disk.
func NewStagedBookStore(downloadDir string) (*StagedBookStore, error) {
	if err := EnsureStagingDir(downloadDir); err != nil {
		return nil, err
	}
	s := &StagedBookStore{
		books:    make(map[string]*StagedBook),
		filePath: filepath.Join(StagingDir(downloadDir), "staged_books.json"),
	}
	s.load()
	return s, nil
}

// Add inserts a staged book and persists the store.
func (s *StagedBookStore) Add(book *StagedBook) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.books[book.ID] = book
	return s.persist()
}

// Remove deletes a staged book by ID and persists the store.
func (s *StagedBookStore) Remove(id string) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	delete(s.books, id)
	return s.persist()
}

// Get retrieves a staged book by ID.
func (s *StagedBookStore) Get(id string) (*StagedBook, bool) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	b, ok := s.books[id]
	return b, ok
}

// All returns a stable copy of all staged books sorted by StagedAt (oldest first).
func (s *StagedBookStore) All() []*StagedBook {
	s.mu.RLock()
	defer s.mu.RUnlock()
	out := make([]*StagedBook, 0, len(s.books))
	for _, b := range s.books {
		out = append(out, b)
	}
	sort.Slice(out, func(i, j int) bool {
		return out[i].StagedAt.Before(out[j].StagedAt)
	})
	return out
}

// Count returns the number of staged books.
func (s *StagedBookStore) Count() int {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return len(s.books)
}

// load reads staged_books.json from disk, dropping entries whose files no longer exist.
func (s *StagedBookStore) load() {
	data, err := os.ReadFile(s.filePath)
	if err != nil {
		return // file doesn't exist yet — that's fine
	}
	var list []StagedBook
	if err := json.Unmarshal(data, &list); err != nil {
		return // corrupted file — start fresh
	}
	for i := range list {
		b := &list[i]
		if _, err := os.Stat(b.StagedPath); err == nil {
			s.books[b.ID] = b
		}
	}
}

// persist atomically writes the current books to disk (must be called under write lock).
func (s *StagedBookStore) persist() error {
	list := make([]StagedBook, 0, len(s.books))
	for _, b := range s.books {
		list = append(list, *b)
	}
	data, err := json.MarshalIndent(list, "", "  ")
	if err != nil {
		return err
	}
	tmp := s.filePath + ".tmp"
	if err := os.WriteFile(tmp, data, 0644); err != nil {
		return err
	}
	return os.Rename(tmp, s.filePath)
}

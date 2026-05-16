package server

import (
	"encoding/json"
	"os"
	"path/filepath"
	"sync"
	"time"
)

const maxHistoryEntries = 50

// HistoryEntry is one recorded search.
type HistoryEntry struct {
	Query     string    `json:"query"`
	Timestamp int64     `json:"timestamp"` // Unix milliseconds, matches frontend Date.now()
	TimedOut  bool      `json:"timedOut,omitempty"`
}

// SearchHistoryStore is a thread-safe, file-backed list of recent searches.
type SearchHistoryStore struct {
	mu       sync.RWMutex
	entries  []HistoryEntry
	filePath string
}

func newSearchHistoryStore(downloadDir string) (*SearchHistoryStore, error) {
	if err := os.MkdirAll(stagingDir(downloadDir), 0755); err != nil {
		return nil, err
	}
	s := &SearchHistoryStore{
		filePath: filepath.Join(stagingDir(downloadDir), "search_history.json"),
	}
	s.load()
	return s, nil
}

// Add prepends a new entry (de-duplicating by query) and persists.
func (s *SearchHistoryStore) Add(query string) {
	s.mu.Lock()
	defer s.mu.Unlock()

	// Remove any existing entry for the same query so it moves to the top.
	filtered := s.entries[:0]
	for _, e := range s.entries {
		if e.Query != query {
			filtered = append(filtered, e)
		}
	}
	entry := HistoryEntry{
		Query:     query,
		Timestamp: time.Now().UnixMilli(),
	}
	s.entries = append([]HistoryEntry{entry}, filtered...)
	if len(s.entries) > maxHistoryEntries {
		s.entries = s.entries[:maxHistoryEntries]
	}
	s.save()
}

// MarkTimedOut sets timedOut=true on the most recent entry for query.
func (s *SearchHistoryStore) MarkTimedOut(query string) {
	s.mu.Lock()
	defer s.mu.Unlock()
	for i := range s.entries {
		if s.entries[i].Query == query {
			s.entries[i].TimedOut = true
			break
		}
	}
	s.save()
}

// Delete removes the entry with the given timestamp.
func (s *SearchHistoryStore) Delete(timestamp int64) {
	s.mu.Lock()
	defer s.mu.Unlock()
	out := s.entries[:0]
	for _, e := range s.entries {
		if e.Timestamp != timestamp {
			out = append(out, e)
		}
	}
	s.entries = out
	s.save()
}

// Clear removes all entries.
func (s *SearchHistoryStore) Clear() {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.entries = nil
	s.save()
}

// All returns a snapshot of current entries (newest first).
func (s *SearchHistoryStore) All() []HistoryEntry {
	s.mu.RLock()
	defer s.mu.RUnlock()
	cp := make([]HistoryEntry, len(s.entries))
	copy(cp, s.entries)
	return cp
}

func (s *SearchHistoryStore) load() {
	data, err := os.ReadFile(s.filePath)
	if err != nil {
		return
	}
	_ = json.Unmarshal(data, &s.entries)
}

func (s *SearchHistoryStore) save() {
	data, err := json.MarshalIndent(s.entries, "", "  ")
	if err != nil {
		return
	}
	_ = os.WriteFile(s.filePath, data, 0644)
}

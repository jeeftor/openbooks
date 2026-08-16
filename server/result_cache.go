package server

import (
	"strings"
	"sync"
	"time"

	"github.com/jeeftor/openbooks/core"
)

// CachedResult stores a set of search results with provenance metadata.
type CachedResult struct {
	Query     string
	Books     []core.BookDetail
	Errors    []core.ParseError
	Timestamp time.Time
	SessionID string // username of the session that originally searched
}

// ResultCache is a bounded, thread-safe cache for IRC search results.
// When the cache is full the oldest entry is evicted to make room.
type ResultCache struct {
	mu      sync.RWMutex
	entries map[string]*CachedResult
	maxAge  time.Duration
	maxSize int
}

// NewResultCache creates a new ResultCache with the given TTL and capacity.
func NewResultCache(maxAge time.Duration, maxSize int) *ResultCache {
	return &ResultCache{
		entries: make(map[string]*CachedResult),
		maxAge:  maxAge,
		maxSize: maxSize,
	}
}

// normalizeQuery lowercases and trims whitespace for consistent key lookup.
func normalizeQuery(q string) string {
	return strings.ToLower(strings.TrimSpace(q))
}

// Get returns a cached result if it exists and has not expired.
func (rc *ResultCache) Get(query string) (*CachedResult, bool) {
	rc.mu.RLock()
	defer rc.mu.RUnlock()
	entry, ok := rc.entries[normalizeQuery(query)]
	if !ok {
		return nil, false
	}
	if time.Since(entry.Timestamp) > rc.maxAge {
		return nil, false
	}
	return entry, true
}

// Set stores a result in the cache, evicting the oldest entry if at capacity.
func (rc *ResultCache) Set(query string, books []core.BookDetail, errors []core.ParseError, sessionID string) {
	rc.mu.Lock()
	defer rc.mu.Unlock()

	// Evict oldest entry when at capacity.
	if len(rc.entries) >= rc.maxSize {
		var oldestKey string
		var oldestTime time.Time
		for k, v := range rc.entries {
			if oldestKey == "" || v.Timestamp.Before(oldestTime) {
				oldestKey = k
				oldestTime = v.Timestamp
			}
		}
		delete(rc.entries, oldestKey)
	}

	rc.entries[normalizeQuery(query)] = &CachedResult{
		Query:     query,
		Books:     books,
		Errors:    errors,
		Timestamp: time.Now(),
		SessionID: sessionID,
	}
}

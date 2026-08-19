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
// It also tracks in-flight IRC queries so duplicate concurrent searches
// subscribe to the first result rather than firing redundant IRC requests.
type ResultCache struct {
	mu       sync.Mutex
	entries  map[string]*CachedResult
	inFlight map[string][]chan *CachedResult
	maxAge   time.Duration
	maxSize  int
}

// NewResultCache creates a new ResultCache with the given TTL and capacity.
func NewResultCache(maxAge time.Duration, maxSize int) *ResultCache {
	return &ResultCache{
		entries:  make(map[string]*CachedResult),
		inFlight: make(map[string][]chan *CachedResult),
		maxAge:   maxAge,
		maxSize:  maxSize,
	}
}

// normalizeQuery lowercases and collapses all whitespace for consistent key lookup.
// "The  Great  Gatsby" and "the great gatsby" resolve to the same cache entry.
func normalizeQuery(q string) string {
	return strings.ToLower(strings.Join(strings.Fields(q), " "))
}

// Get returns a cached result if it exists and has not expired.
func (rc *ResultCache) Get(query string) (*CachedResult, bool) {
	rc.mu.Lock()
	defer rc.mu.Unlock()
	entry, ok := rc.entries[normalizeQuery(query)]
	if !ok {
		return nil, false
	}
	if time.Since(entry.Timestamp) > rc.maxAge {
		return nil, false
	}
	return entry, true
}

// GetOrSubscribe atomically checks the cache and in-flight state.
//
//   - If a fresh cached result exists: returns (result, nil, true).
//   - If the same query is already in-flight: returns (nil, ch, false) where ch
//     will receive the result (or be closed with nil on failure/cancel).
//   - Otherwise: marks the query as in-flight and returns (nil, nil, false) —
//     the caller must fire the IRC request and later call Resolve or CancelInFlight.
func (rc *ResultCache) GetOrSubscribe(query string) (*CachedResult, chan *CachedResult, bool) {
	rc.mu.Lock()
	defer rc.mu.Unlock()

	key := normalizeQuery(query)

	// Check cache first.
	if entry, ok := rc.entries[key]; ok {
		if time.Since(entry.Timestamp) <= rc.maxAge {
			return entry, nil, true
		}
	}

	// Check in-flight.
	if _, ok := rc.inFlight[key]; ok {
		ch := make(chan *CachedResult, 1)
		rc.inFlight[key] = append(rc.inFlight[key], ch)
		return nil, ch, false
	}

	// New query — mark as in-flight.
	rc.inFlight[key] = []chan *CachedResult{}
	return nil, nil, false
}

// Resolve stores the result in the cache and notifies all subscribers.
// It replaces the old Set method for callers that used GetOrSubscribe.
func (rc *ResultCache) Resolve(query string, books []core.BookDetail, errors []core.ParseError, sessionID string) {
	rc.mu.Lock()
	defer rc.mu.Unlock()

	key := normalizeQuery(query)

	// Evict oldest entry when at capacity, but only if we're adding a new key.
	if _, exists := rc.entries[key]; !exists && len(rc.entries) >= rc.maxSize {
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

	result := &CachedResult{
		Query:     query,
		Books:     books,
		Errors:    errors,
		Timestamp: time.Now(),
		SessionID: sessionID,
	}
	rc.entries[key] = result

	// Notify subscribers and clear in-flight state.
	for _, ch := range rc.inFlight[key] {
		ch <- result
	}
	delete(rc.inFlight, key)
}

// CancelInFlight removes the in-flight marker and closes all subscriber channels.
// Subscribers receive nil (zero value on close), signalling that no result will arrive.
func (rc *ResultCache) CancelInFlight(query string) {
	rc.mu.Lock()
	defer rc.mu.Unlock()

	key := normalizeQuery(query)
	for _, ch := range rc.inFlight[key] {
		close(ch)
	}
	delete(rc.inFlight, key)
}

// Set stores a result in the cache, evicting the oldest entry if at capacity.
// Prefer Resolve when using GetOrSubscribe so subscribers are notified.
func (rc *ResultCache) Set(query string, books []core.BookDetail, errors []core.ParseError, sessionID string) {
	rc.Resolve(query, books, errors, sessionID)
}

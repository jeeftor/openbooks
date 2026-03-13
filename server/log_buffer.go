package server

import (
	"sync"
	"time"
)

type LogEntry struct {
	Time    time.Time `json:"time"`
	Level   string    `json:"level"` // "info", "warn", "error"
	Message string    `json:"message"`
}

type logBuffer struct {
	mu      sync.Mutex
	entries []LogEntry
	max     int
}

func newLogBuffer(max int) *logBuffer {
	return &logBuffer{max: max, entries: make([]LogEntry, 0, max)}
}

func (b *logBuffer) append(level, msg string) {
	b.mu.Lock()
	defer b.mu.Unlock()
	b.entries = append(b.entries, LogEntry{Time: time.Now(), Level: level, Message: msg})
	if len(b.entries) > b.max {
		b.entries = b.entries[len(b.entries)-b.max:]
	}
}

func (b *logBuffer) info(msg string)  { b.append("info", msg) }
func (b *logBuffer) warn(msg string)  { b.append("warn", msg) }
func (b *logBuffer) error(msg string) { b.append("error", msg) }

// all returns entries newest-first.
func (b *logBuffer) all() []LogEntry {
	b.mu.Lock()
	defer b.mu.Unlock()
	result := make([]LogEntry, len(b.entries))
	copy(result, b.entries)
	for i, j := 0, len(result)-1; i < j; i, j = i+1, j-1 {
		result[i], result[j] = result[j], result[i]
	}
	return result
}

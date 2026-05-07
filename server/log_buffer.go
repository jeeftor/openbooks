package server

import (
	"sync"
	"time"
)

type LogEntry struct {
	Time    time.Time `json:"time"`
	Level   string    `json:"level"`            // "info", "warn", "error"
	Message string    `json:"message"`
	Detail  string    `json:"detail,omitempty"` // optional hover detail (e.g. command output)
	Group   string    `json:"group,omitempty"`  // groups related entries (e.g. per-download session)
}

type logBuffer struct {
	mu      sync.Mutex
	entries []LogEntry
	max     int
}

func newLogBuffer(max int) *logBuffer {
	return &logBuffer{max: max, entries: make([]LogEntry, 0, max)}
}

func (b *logBuffer) appendEntry(level, msg, detail, group string) {
	b.mu.Lock()
	defer b.mu.Unlock()
	b.entries = append(b.entries, LogEntry{
		Time: time.Now(), Level: level, Message: msg, Detail: detail, Group: group,
	})
	if len(b.entries) > b.max {
		b.entries = b.entries[len(b.entries)-b.max:]
	}
}

func (b *logBuffer) info(msg string)                { b.appendEntry("info", msg, "", "") }
func (b *logBuffer) infoDetail(msg, detail string)  { b.appendEntry("info", msg, detail, "") }
func (b *logBuffer) warn(msg string)                { b.appendEntry("warn", msg, "", "") }
func (b *logBuffer) warnDetail(msg, detail string)  { b.appendEntry("warn", msg, detail, "") }
func (b *logBuffer) error(msg string)               { b.appendEntry("error", msg, "", "") }
func (b *logBuffer) errorDetail(msg, detail string) { b.appendEntry("error", msg, detail, "") }

// session returns a logSession that tags all entries with the given group ID.
func (b *logBuffer) session(group string) *logSession {
	return &logSession{buf: b, group: group}
}

// bookLogger is implemented by both *logBuffer and *logSession.
// It allows runPostProcess and other helpers to accept either.
type bookLogger interface {
	info(msg string)
	infoDetail(msg, detail string)
	warn(msg string)
	warnDetail(msg, detail string)
	error(msg string)
	errorDetail(msg, detail string)
}

// logSession wraps logBuffer and tags every entry with a group identifier,
// so related log lines (e.g. all entries for one download) can be highlighted together.
type logSession struct {
	buf   *logBuffer
	group string
}

func (s *logSession) info(msg string)                { s.buf.appendEntry("info", msg, "", s.group) }
func (s *logSession) infoDetail(msg, detail string)  { s.buf.appendEntry("info", msg, detail, s.group) }
func (s *logSession) warn(msg string)                { s.buf.appendEntry("warn", msg, "", s.group) }
func (s *logSession) warnDetail(msg, detail string)  { s.buf.appendEntry("warn", msg, detail, s.group) }
func (s *logSession) error(msg string)               { s.buf.appendEntry("error", msg, "", s.group) }
func (s *logSession) errorDetail(msg, detail string) { s.buf.appendEntry("error", msg, detail, s.group) }

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

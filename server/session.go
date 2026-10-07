package server

import (
	"context"
	"fmt"
	"sync"
	"sync/atomic"
	"time"

	"github.com/jeeftor/openbooks/core"
)

// concurrentDownloads is the maximum number of simultaneous IRC DCC transfers.
const concurrentDownloads = 2

// serverListSnapshot holds the IRC server list with freshness timestamp.
type serverListSnapshot struct {
	servers   core.IrcServers
	timestamp time.Time
}

// session represents a long-lived browser session that persists beyond WebSocket
// connections. One session is created per browser UUID (persisted via cookie).
// IRC is now shared via ircHub; sessions are thin WebSocket-routing structs.
type session struct {
	username string

	// ctx/cancel govern the lifetime of the session itself.
	// Used by bookResultHandler to detect session eviction (saveToStaged path).
	ctx    context.Context
	cancel context.CancelFunc

	// renameMu is a mutex (capacity-1 channel, pre-filled) that serialises the
	// rename dialog. Only one RENAME_PROMPT is shown at a time per session.
	renameMu chan struct{}

	// mu protects the clients map and lastSeen.
	mu sync.RWMutex

	// clients holds all attached WebSocket clients for this session.
	// Multiple browser windows/tabs can share the same session via the same cookie.
	clients map[*Client]struct{}

	// query is the most recently dispatched IRC search term (for logging).
	query string

	// serverMu protects serverSnapshot.
	serverMu sync.RWMutex

	// serverSnapshot holds the last received IRC server list with timestamp.
	serverSnapshot serverListSnapshot

	// lastSeen is updated whenever a client attaches or detaches.
	// Used by the session reaper to determine idle TTL.
	lastSeen time.Time

	// inFlightDownloads tracks downloads currently in the hub's pipeline for
	// this session. The reaper will not evict a session with in-flight downloads.
	inFlightDownloads atomic.Int32
}

// newSession creates a new lightweight session.
func newSession(username string) *session {
	ctx, cancel := context.WithCancel(context.Background())

	renameMu := make(chan struct{}, 1)
	renameMu <- struct{}{}

	return &session{
		username: username,
		ctx:      ctx,
		cancel:   cancel,
		renameMu: renameMu,
		clients:  make(map[*Client]struct{}),
		lastSeen: time.Now(),
	}
}

// attachClient adds a WebSocket client to this session.
func (sess *session) attachClient(c *Client) {
	sess.mu.Lock()
	sess.clients[c] = struct{}{}
	sess.lastSeen = time.Now()
	sess.mu.Unlock()
}

// detachClient removes a WebSocket client from this session.
func (sess *session) detachClient(c *Client) {
	sess.mu.Lock()
	delete(sess.clients, c)
	sess.lastSeen = time.Now()
	sess.mu.Unlock()
}

// getClients returns a snapshot of all attached clients, or nil if none.
func (sess *session) getClients() []*Client {
	sess.mu.RLock()
	defer sess.mu.RUnlock()
	if len(sess.clients) == 0 {
		return nil
	}
	result := make([]*Client, 0, len(sess.clients))
	for c := range sess.clients {
		result = append(result, c)
	}
	return result
}

// getAnyClient returns an arbitrary attached client, or nil if none connected.
func (sess *session) getAnyClient() *Client {
	sess.mu.RLock()
	defer sess.mu.RUnlock()
	for c := range sess.clients {
		return c
	}
	return nil
}

// hasInFlightDownload reports whether this session has downloads in progress.
func (sess *session) hasInFlightDownload() bool {
	return sess.inFlightDownloads.Load() > 0
}

// idleSince returns how long the session has been idle (no attached clients).
// Returns 0 if clients are currently attached.
func (sess *session) idleSince() time.Duration {
	sess.mu.RLock()
	defer sess.mu.RUnlock()
	if len(sess.clients) > 0 {
		return 0
	}
	return time.Since(sess.lastSeen)
}

// setServerList updates the session's server list snapshot with the current time.
func (sess *session) setServerList(servers core.IrcServers) {
	sess.serverMu.Lock()
	defer sess.serverMu.Unlock()
	sess.serverSnapshot = serverListSnapshot{
		servers:   servers,
		timestamp: time.Now(),
	}
}

// getServerList returns the session's server list and the timestamp it was last updated.
// If the list is older than maxAge, the third return value is false (stale data).
func (sess *session) getServerList(maxAge time.Duration) (core.IrcServers, time.Time, bool) {
	sess.serverMu.RLock()
	defer sess.serverMu.RUnlock()
	snapshot := sess.serverSnapshot
	fresh := time.Since(snapshot.timestamp) < maxAge
	return snapshot.servers, snapshot.timestamp, fresh
}

func (sess *session) String() string {
	return fmt.Sprintf("%s", sess.username)
}

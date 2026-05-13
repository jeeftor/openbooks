package server

import (
	"context"
	"fmt"
	"strings"
	"sync"
	"time"

	"github.com/evan-buss/openbooks/core"
	"github.com/evan-buss/openbooks/irc"
)

// session represents a long-lived IRC session that persists beyond WebSocket connections.
// One session is created per browser UUID (persisted via cookie). Downloads continue
// in the background even when the browser tab is closed.
type session struct {
	username string

	// IRC connection for this session.
	irc *irc.Conn

	// ctx/cancel govern the lifetime of the session itself (not tied to any browser connection).
	ctx    context.Context
	cancel context.CancelFunc

	// downloadQueue serializes downloads so only one DCC transfer is active at a time.
	downloadQueue chan downloadJob

	// downloadDone is signaled by bookResultHandler when a download finishes.
	downloadDone chan struct{}

	// mu protects the client pointer below.
	mu sync.RWMutex

	// client is the currently attached WebSocket client. Nil when the browser is disconnected.
	client *Client
}

// newSession creates a new IRC session with its own connection and download queue.
func newSession(username, userAgent string) *session {
	ctx, cancel := context.WithCancel(context.Background())
	return &session{
		username:      username,
		irc:           irc.New(username, userAgent),
		ctx:           ctx,
		cancel:        cancel,
		downloadQueue: make(chan downloadJob, 50),
		downloadDone:  make(chan struct{}, 1),
	}
}

// attachClient sets the active WebSocket client for this session.
func (sess *session) attachClient(c *Client) {
	sess.mu.Lock()
	sess.client = c
	sess.mu.Unlock()
}

// detachClient removes the active client reference. Called when the browser disconnects.
// Only removes the client if it matches the one we expect (guards against reconnect races).
func (sess *session) detachClient(c *Client) {
	sess.mu.Lock()
	if sess.client == c {
		sess.client = nil
	}
	sess.mu.Unlock()
}

// getClient returns the currently attached client (may be nil).
func (sess *session) getClient() *Client {
	sess.mu.RLock()
	defer sess.mu.RUnlock()
	return sess.client
}

// signalDone unblocks processDownloadQueue so the next job can start.
func (sess *session) signalDone() {
	select {
	case sess.downloadDone <- struct{}{}:
	default:
	}
}

// processDownloadQueue drains downloadQueue one job at a time, sending each
// request to IRC and waiting for bookResultHandler to signal completion.
// This runs for the lifetime of the session — NOT tied to any browser connection.
func (sess *session) processDownloadQueue(server *server) {
	for {
		select {
		case job, ok := <-sess.downloadQueue:
			if !ok {
				return
			}
			c := sess.getClient()
			pending := len(sess.downloadQueue)
			if pending > 0 {
				server.logBuf.info(fmt.Sprintf("📋 Queued: %s (%d pending)", job.title, pending))
			}
			botName := job.book
			if idx := strings.Index(job.book, " "); idx > 1 {
				botName = job.book[1:idx]
			}
			server.logBuf.info(fmt.Sprintf("📡 Requesting from %s — waiting for IRC bot to send file…", botName))
			safeSend(c, newDownloadWaitingResponse(botName, job.title))
			core.DownloadBook(sess.irc, job.book)
			// Wait for bookResultHandler to signal completion or give up after 5 min.
			select {
			case <-sess.downloadDone:
				// bookResultHandler already sent the clear; re-read client in case reconnect happened.
			case <-time.After(5 * time.Minute):
				safeSend(sess.getClient(), newDownloadWaitingClear())
				server.logBuf.warn(fmt.Sprintf("⏱️  Timed out waiting for %s — bot may be offline or throttling. Skipping.", botName))
			case <-sess.ctx.Done():
				return
			}
		case <-sess.ctx.Done():
			return
		}
	}
}

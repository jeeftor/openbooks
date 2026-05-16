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

// concurrentDownloads is the maximum number of simultaneous IRC DCC transfers per session.
const concurrentDownloads = 2

// slotHandle coordinates the one-time release of a download semaphore slot.
// Both the per-job timeout goroutine and bookResultHandler hold a reference;
// sync.Once ensures exactly one of them actually releases the slot.
type slotHandle struct {
	once sync.Once
	done chan struct{} // closed after release so the timeout goroutine can exit early
	sess *session
}

func newSlotHandle(sess *session) *slotHandle {
	return &slotHandle{done: make(chan struct{}), sess: sess}
}

func (h *slotHandle) release() {
	h.once.Do(func() {
		h.sess.downloadSlots <- struct{}{}
		close(h.done)
	})
}

// searchJob holds a queued search request.
type searchJob struct {
	query string
}

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

	// searchQueue holds pending search requests. Processed one at a time with a
	// cooldown between each to avoid hammering the IRC search bot.
	searchQueue chan searchJob

	// downloadQueue holds pending download requests.
	downloadQueue chan downloadJob

	// downloadSlots is a semaphore (capacity = concurrentDownloads, pre-filled).
	// processDownloadQueue drains one token before starting each IRC request;
	// bookResultHandler returns the token once the file is on disk.
	downloadSlots chan struct{}

	// pendingSlots is a FIFO queue of slotHandles, one per in-flight IRC request.
	// bookResultHandler pops one handle to coordinate slot release with the timeout goroutine.
	pendingSlots chan *slotHandle

	// renameMu is a mutex (capacity-1 channel, pre-filled) that serialises the rename
	// dialog. When two downloads finish close together, only one RENAME_PROMPT is sent
	// at a time so the frontend never receives two overlapping rename dialogs.
	renameMu chan struct{}

	// mu protects the client pointer below.
	mu sync.RWMutex

	// client is the currently attached WebSocket client. Nil when the browser is disconnected.
	client *Client
}

// newSession creates a new IRC session with its own connection and download queue.
func newSession(username, userAgent string) *session {
	ctx, cancel := context.WithCancel(context.Background())

	slots := make(chan struct{}, concurrentDownloads)
	for i := 0; i < concurrentDownloads; i++ {
		slots <- struct{}{}
	}

	renameMu := make(chan struct{}, 1)
	renameMu <- struct{}{}

	return &session{
		username:      username,
		irc:           irc.New(username, userAgent),
		ctx:           ctx,
		cancel:        cancel,
		searchQueue:   make(chan searchJob, 20),
		downloadQueue: make(chan downloadJob, 50),
		downloadSlots: slots,
		pendingSlots:  make(chan *slotHandle, concurrentDownloads),
		renameMu:      renameMu,
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

// processSearchQueue drains searchQueue one at a time, enforcing a cooldown between
// each IRC search request. This replaces the old server-global rate-limit rejection;
// searches are now queued and fired automatically rather than dropped.
func (sess *session) processSearchQueue(srv *server) {
	var lastSearch time.Time
	cooldown := srv.config.SearchTimeout

	for {
		select {
		case job, ok := <-sess.searchQueue:
			if !ok {
				return
			}

			// Wait out any remaining cooldown from the previous search.
			if wait := cooldown - time.Since(lastSearch); wait > 0 {
				pending := len(sess.searchQueue)
				if pending > 0 {
					safeSend(sess.getClient(), newStatusResponse(NOTIFY,
						fmt.Sprintf("Search queued (%d pending) — sending in %.0fs…", pending+1, wait.Seconds())))
				} else {
					safeSend(sess.getClient(), newStatusResponse(NOTIFY,
						fmt.Sprintf("Search queued — sending in %.0fs…", wait.Seconds())))
				}
				select {
				case <-time.After(wait):
				case <-sess.ctx.Done():
					return
				}
			}

			srv.logBuf.info(fmt.Sprintf("CLIENT (%s): 🔍 IRC SEARCH → %q", sess.username, job.query))
			srv.log.Printf("CLIENT (%s): IRC SEARCH → %q\n", sess.username, job.query)
			safeSend(sess.getClient(), newStatusResponse(NOTIFY, fmt.Sprintf("Searching for %q…", job.query)))
			core.SearchBook(sess.irc, srv.config.SearchBot, job.query)
			lastSearch = time.Now()

		case <-sess.ctx.Done():
			return
		}
	}
}

// processDownloadQueue drains downloadQueue up to concurrentDownloads at a time.
// It acquires a semaphore slot before sending each IRC request, then immediately
// moves on to the next job. bookResultHandler releases the slot once the file lands
// on disk — not after the user finishes the rename dialog — so downloads pipeline
// while the user processes previously downloaded books.
func (sess *session) processDownloadQueue(server *server) {
	for {
		select {
		case job, ok := <-sess.downloadQueue:
			if !ok {
				return
			}

			// Acquire a download slot — blocks only when concurrentDownloads are already
			// in-flight waiting for a DCC offer from an IRC bot.
			select {
			case <-sess.downloadSlots:
			case <-sess.ctx.Done():
				return
			}

			pending := len(sess.downloadQueue)
			if pending > 0 {
				server.logBuf.info(fmt.Sprintf("📋 Queued: %s (%d more pending)", job.title, pending))
			}
			botName := job.book
			if idx := strings.Index(job.book, " "); idx > 1 {
				botName = job.book[1:idx]
			}
			server.logBuf.info(fmt.Sprintf("📡 Requesting from %s — waiting for IRC bot to send file…", botName))
			safeSend(sess.getClient(), newDownloadWaitingResponse(botName, job.title))

			// Push a handle into the FIFO before firing the IRC request so
			// bookResultHandler can pop it in order.
			handle := newSlotHandle(sess)
			sess.pendingSlots <- handle

			core.DownloadBook(sess.irc, job.book)

			// Per-job timeout goroutine: if the bot never sends a DCC SEND offer,
			// release the slot so the queue doesn't stall forever.
			go func(h *slotHandle, bot, title string) {
				select {
				case <-time.After(5 * time.Minute):
					safeSend(sess.getClient(), newDownloadWaitingClear())
					server.logBuf.warn(fmt.Sprintf("⏱️  Timed out waiting for %s — bot may be offline or throttling. Skipping.", bot))
					h.release()
				case <-h.done:
					// bookResultHandler already handled this slot — exit cleanly.
				case <-sess.ctx.Done():
					h.release()
				}
			}(handle, botName, job.title)

		case <-sess.ctx.Done():
			return
		}
	}
}

package server

import (
	"context"
	"encoding/base64"
	"fmt"
	"log"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"sync"
	"time"

	"github.com/google/uuid"
	"github.com/jeeftor/openbooks/core"
	"github.com/jeeftor/openbooks/dcc"
	"github.com/jeeftor/openbooks/irc"
	"github.com/jeeftor/openbooks/staging"
	"github.com/jeeftor/openbooks/util"
)

// sharedSearchJob is a search request from a specific session queued to the hub.
type sharedSearchJob struct {
	query   string
	session *session
}

// sharedDownloadJob is a download request from a specific session queued to the hub.
type sharedDownloadJob struct {
	book    string
	title   string
	author  string
	session *session
}

// sharedSlotHandle coordinates the one-time release of a download semaphore slot
// and carries a reference to the session that requested the download for result routing.
type sharedSlotHandle struct {
	once    sync.Once
	done    chan struct{}
	session *session
	hub     *ircHub
}

func newSharedSlotHandle(sess *session, hub *ircHub) *sharedSlotHandle {
	return &sharedSlotHandle{done: make(chan struct{}), session: sess, hub: hub}
}

func (h *sharedSlotHandle) release() {
	h.once.Do(func() {
		h.hub.downloadSlots <- struct{}{}
		close(h.done)
	})
}

const (
	// searchDegradedThreshold is the number of consecutive search timeouts
	// after which the primary search bot is marked down.
	searchDegradedThreshold = 3
	// searchDegradedCooldown is how long searches route to the fallback bot
	// before the primary bot is probed again.
	searchDegradedCooldown = 30 * time.Minute
)

// searchPhase tracks how far an in-flight search has progressed with the bot.
type searchPhase int

const (
	// searchAwaitingResponse: query sent, no bot signal received yet.
	searchAwaitingResponse searchPhase = iota
	// searchAwaitingResult: bot acknowledged the query; awaiting the DCC
	// results file or a terminal notice.
	searchAwaitingResult
)

// inflightSearch tracks the one search currently awaiting a bot response.
type inflightSearch struct {
	session *session
	query   string
	bot     string        // bot nick the current attempt was sent to
	phase   searchPhase   // guarded by hub.currentMu
	probe   bool          // true when this search probes the primary bot after a degraded cooldown
	retried bool          // true when the query was already re-sent to the fallback bot
	done    chan struct{} // closed when the search resolves or is superseded
	rearm   chan struct{} // signalled (cap 1) when phase advances to searchAwaitingResult
}

// ircHub owns the single shared IRC connection for all frontend sessions.
// All search and download requests are routed through this hub.
type ircHub struct {
	irc           *irc.Conn
	ctx           context.Context
	searchQueue   chan sharedSearchJob   // cap 50
	downloadQueue chan sharedDownloadJob // cap 100
	downloadSlots chan struct{}          // semaphore, cap concurrentDownloads
	pendingSlots  chan *sharedSlotHandle // FIFO for DCC result routing

	// current is the search awaiting a bot response. Only processSearchQueue
	// installs a new one (via prepareSearch); all access is under currentMu.
	current   *inflightSearch
	currentMu sync.Mutex

	// Search-bot health state, guarded by currentMu. primaryTimeouts counts
	// consecutive timeouts on searches sent to the primary bot; fallback-bot
	// outcomes neither increment nor reset it.
	primaryTimeouts  int
	searchDegraded   bool
	degradedUntil    time.Time

	// lastElevated is the previously logged elevated-user list; used to
	// dedupe the periodic NAMES refresh so logs only change on diff.
	lastElevated []string
	serverListMu sync.Mutex

	srv *server
	log *log.Logger
}

func newIrcHub(srv *server, ctx context.Context) *ircHub {
	slots := make(chan struct{}, concurrentDownloads)
	for i := 0; i < concurrentDownloads; i++ {
		slots <- struct{}{}
	}
	return &ircHub{
		irc:           irc.New(srv.config.UserName, srv.config.UserAgent),
		ctx:           ctx,
		searchQueue:   make(chan sharedSearchJob, 50),
		downloadQueue: make(chan sharedDownloadJob, 100),
		downloadSlots: slots,
		pendingSlots:  make(chan *sharedSlotHandle, concurrentDownloads),
		srv:           srv,
		log:           log.New(os.Stdout, "IRC HUB: ", log.LstdFlags|log.Lmsgprefix),
	}
}

// isConnected reports whether the hub's IRC connection is live.
func (h *ircHub) isConnected() bool {
	return h.irc.IsConnected()
}

// connectAndRun connects to IRC and runs the reader + queue processors.
// On EOF or error it reconnects automatically. Exits when ctx is cancelled.
func (h *ircHub) connectAndRun() {
	for {
		select {
		case <-h.ctx.Done():
			return
		default:
		}

		if err := core.Join(h.irc, h.srv.config.Server, h.srv.config.EnableTLS); err != nil {
			h.log.Printf("connect failed: %v — retrying in 30s", err)
			h.srv.logBuf.warn(fmt.Sprintf("IRC connect failed: %v — retrying in 30s", err))
			select {
			case <-time.After(30 * time.Second):
			case <-h.ctx.Done():
				return
			}
			continue
		}

		h.log.Printf("connected as %s", h.irc.Username)
		h.srv.logBuf.info(fmt.Sprintf("🔌 IRC connected: %s", h.irc.Username))

		handler := h.buildEventHandler()

		if h.srv.config.Log {
			logger, _, err := util.CreateLogFile(h.irc.Username, h.srv.config.DownloadDir)
			if err != nil {
				h.log.Println(err)
			} else {
				broadcast := handler[core.Message]
				handler[core.Message] = func(text string) {
					logger.Println(text)
					if broadcast != nil {
						broadcast(text)
					}
				}
			}
		}

		readerDone := make(chan struct{})
		go func() {
			defer close(readerDone)
			core.StartReader(h.ctx, h.irc, handler)
		}()
		go h.processSearchQueue()
		go h.processDownloadQueue()
		go h.refreshServerList()

		select {
		case <-readerDone:
			h.log.Printf("reader exited — reconnecting in 15s")
			h.srv.logBuf.warn("IRC connection lost — reconnecting in 15s")
			// Notify the session whose search was in-flight.
			h.clearAndNotifyCurrentSearch()
			// Fresh conn for the next cycle.
			h.irc = irc.New(h.srv.config.UserName, h.srv.config.UserAgent)
			select {
			case <-time.After(15 * time.Second):
			case <-h.ctx.Done():
				return
			}
		case <-h.ctx.Done():
			h.irc.Disconnect()
			return
		}
	}
}

// clearAndNotifyCurrentSearch clears the in-flight search and notifies its
// clients that the connection dropped so they don't wait indefinitely.
func (h *ircHub) clearAndNotifyCurrentSearch() {
	h.currentMu.Lock()
	st := h.current
	h.current = nil
	var query string
	if st != nil {
		query = st.query
		close(st.done)
	}
	h.currentMu.Unlock()

	if st != nil {
		h.srv.resultCache.CancelInFlight(query)
		broadcastToClients(st.session.getClients(), newErrorResponse("IRC connection lost — please retry your search."))
	}
}

// processSearchQueue drains searchQueue one at a time, enforcing a cooldown between
// each IRC search request. Runs as a goroutine per connectAndRun cycle.
func (h *ircHub) processSearchQueue() {
	var lastSearch time.Time
	cooldown := h.srv.config.SearchTimeout

	for {
		select {
		case job, ok := <-h.searchQueue:
			if !ok {
				return
			}
			if wait := cooldown - time.Since(lastSearch); wait > 0 {
				pending := len(h.searchQueue)
				var msg string
				if pending > 0 {
					msg = fmt.Sprintf("Search queued (%d pending) — sending in %.0fs…", pending+1, wait.Seconds())
				} else {
					msg = fmt.Sprintf("Search queued — sending in %.0fs…", wait.Seconds())
				}
				broadcastToClients(job.session.getClients(), newStatusResponse(NOTIFY, msg))
				select {
				case <-time.After(wait):
				case <-h.ctx.Done():
					return
				}
			}

			h.srv.logBuf.info(fmt.Sprintf("CLIENT (%s): 🔍 IRC SEARCH → %q", job.session.username, job.query))
			h.srv.log.Printf("CLIENT (%s): IRC SEARCH → %q\n", job.session.username, job.query)
			broadcastToClients(job.session.getClients(), newStatusResponse(NOTIFY, fmt.Sprintf("Searching for %q…", job.query)))

			st, superseded := h.prepareSearch(job.session, job.query)
			for _, prev := range superseded {
				broadcastToClients(prev.session.getClients(), newStatusResponse(WARNING, "Search superseded by a newer query."))
			}
			if !strings.EqualFold(st.bot, h.srv.config.SearchBot) {
				broadcastToClients(job.session.getClients(), newStatusResponse(NOTIFY,
					fmt.Sprintf("Primary search bot unresponsive — trying %q instead.", st.bot)))
			} else if st.probe {
				broadcastToClients(job.session.getClients(), newStatusResponse(NOTIFY, "Retrying primary search bot…"))
			}

			core.SearchBook(h.irc, st.bot, job.query)
			lastSearch = time.Now()
			go h.watchSearch(st)

		case <-h.ctx.Done():
			return
		}
	}
}

// processDownloadQueue drains downloadQueue up to concurrentDownloads at a time.
// Runs as a goroutine per connectAndRun cycle.
func (h *ircHub) processDownloadQueue() {
	for {
		select {
		case job, ok := <-h.downloadQueue:
			if !ok {
				return
			}

			select {
			case <-h.downloadSlots:
			case <-h.ctx.Done():
				return
			}

			pending := len(h.downloadQueue)
			if pending > 0 {
				h.srv.logBuf.info(fmt.Sprintf("📋 Queued: %s (%d more pending)", job.title, pending))
			}

			botName := job.book
			if idx := strings.Index(job.book, " "); idx > 1 {
				botName = job.book[1:idx]
			}
			h.srv.logBuf.info(fmt.Sprintf("📡 Requesting from %s — waiting for IRC bot to send file…", botName))
			broadcastToClients(job.session.getClients(), newDownloadWaitingResponse(botName, job.title))

			handle := newSharedSlotHandle(job.session, h)
			h.pendingSlots <- handle

			core.DownloadBook(h.irc, job.book)

			go func(handle *sharedSlotHandle, bot, title string, sess *session) {
				select {
				case <-time.After(5 * time.Minute):
					broadcastToClients(sess.getClients(), newDownloadWaitingClear())
					h.srv.logBuf.warn(fmt.Sprintf("⏱️  Timed out waiting for %s — bot may be offline or throttling. Skipping.", bot))
					handle.release()
				case <-handle.done:
				case <-h.ctx.Done():
					handle.release()
				}
			}(handle, botName, job.title, job.session)

		case <-h.ctx.Done():
			return
		}
	}
}

// refreshServerList periodically requests the names list from IRC to keep
// the server list up to date. Runs as a goroutine per connectAndRun cycle.
func (h *ircHub) refreshServerList() {
	ticker := time.NewTicker(30 * time.Second)
	defer ticker.Stop()
	for {
		select {
		case <-ticker.C:
			if h.irc.IsConnected() {
				h.irc.GetUsers("ebooks")
			}
		case <-h.ctx.Done():
			return
		}
	}
}

// buildEventHandler constructs the event handler map for the shared IRC connection.
func (h *ircHub) buildEventHandler() core.EventHandler {
	handler := core.EventHandler{}
	handler[core.SearchResult] = h.searchResultHandler()
	handler[core.BookResult] = h.bookResultHandler()
	handler[core.NoResults] = h.noResultsHandler()
	handler[core.BadServer] = h.badServerHandler()
	handler[core.SearchAccepted] = h.searchAcceptedHandler()
	handler[core.MatchesFound] = h.matchesFoundHandler()
	handler[core.Ping] = func(s string) { h.irc.Pong(s) }
	handler[core.Version] = func(s string) { core.SendVersionInfo(h.irc, s, h.srv.config.UserAgent) }
	handler[core.ServerList] = h.serverListHandler()
	handler[core.Message] = h.ircMessageHandler()
	handler[core.ChannelBanned] = h.channelBroadcastError("You are banned from #ebooks. The app will not retry. Try again later or use a different network.")
	handler[core.ChannelFull] = h.channelBroadcastError("#ebooks is full. Try again later.")
	handler[core.InviteOnly] = h.channelBroadcastError("#ebooks is invite-only. You cannot join.")
	handler[core.BadChannelKey] = h.channelBroadcastError("Bad channel key for #ebooks. The channel may require a password.")
	handler[core.NickInUse] = h.channelBroadcastError("IRC nickname is already in use. Try setting a different --name.")
	return handler
}

// prepareSearch selects which bot should receive the query based on the
// current health state, installs it as the in-flight search, and returns it
// along with any searches it superseded. A still-pending previous search is
// closed out and its result-cache waiters released so they don't hang.
func (h *ircHub) prepareSearch(sess *session, query string) (*inflightSearch, []*inflightSearch) {
	h.currentMu.Lock()
	defer h.currentMu.Unlock()

	primary := h.srv.config.SearchBot
	fallback := strings.TrimSpace(h.srv.config.SearchBotFallback)
	bot := primary
	probe := false
	if h.searchDegraded {
		switch {
		case time.Now().Before(h.degradedUntil) && fallback != "" && !strings.EqualFold(fallback, primary):
			bot = fallback
		case time.Now().Before(h.degradedUntil):
			// Degraded but no usable fallback — keep trying the primary.
		default:
			// Cooldown elapsed — this search probes the primary bot.
			probe = true
		}
	}

	st := &inflightSearch{
		session: sess,
		query:   query,
		bot:     bot,
		probe:   probe,
		done:    make(chan struct{}),
		rearm:   make(chan struct{}, 1),
	}

	var superseded []*inflightSearch
	if prev := h.current; prev != nil {
		close(prev.done)
		h.srv.resultCache.CancelInFlight(prev.query)
		superseded = append(superseded, prev)
	}
	h.current = st
	return st, superseded
}

// watchSearch enforces the two-phase response timeout for an in-flight
// search: SearchResponseTimeout while awaiting any bot signal, then
// SearchResultTimeout once the bot has acknowledged. Phase transitions are
// signalled via st.rearm; resolution closes st.done.
func (h *ircHub) watchSearch(st *inflightSearch) {
	timeout := h.srv.config.SearchResponseTimeout
	for {
		timer := time.NewTimer(timeout)
		select {
		case <-st.done:
			timer.Stop()
			return
		case <-h.ctx.Done():
			timer.Stop()
			return
		case <-st.rearm:
			timer.Stop()
			timeout = h.srv.config.SearchResultTimeout
			continue
		case <-timer.C:
			if h.searchTimedOut(st) {
				// Re-armed onto the fallback bot — keep watching.
				timeout = h.srv.config.SearchResponseTimeout
				continue
			}
			return
		}
	}
}

// searchTimedOut handles a watchdog expiry for st. It returns true when the
// search was re-armed onto the fallback bot (the caller keeps watching); it
// returns false when the search reached a final state — resolved as failed,
// or superseded by a newer search.
//
// A timeout on the primary bot always counts toward degradation, even when
// the fallback retry later rescues the search — otherwise a dead primary
// would never be learned while fallbacks keep succeeding.
func (h *ircHub) searchTimedOut(st *inflightSearch) bool {
	h.currentMu.Lock()
	if h.current != st {
		h.currentMu.Unlock()
		return false // superseded by a newer search
	}

	attemptedBot := st.bot
	attemptedPrimary := strings.EqualFold(attemptedBot, h.srv.config.SearchBot)
	phase := st.phase
	fallback := strings.TrimSpace(h.srv.config.SearchBotFallback)

	enteredDegraded := false
	if attemptedPrimary {
		h.primaryTimeouts++
		if st.probe || h.primaryTimeouts >= searchDegradedThreshold {
			if !h.searchDegraded {
				enteredDegraded = true
			}
			h.searchDegraded = true
			h.degradedUntil = time.Now().Add(searchDegradedCooldown)
		}
	} else if h.searchDegraded {
		// The fallback timed out too — postpone the primary probe.
		h.degradedUntil = time.Now().Add(searchDegradedCooldown)
	}

	// Retry the same query once on the fallback bot when the primary attempt
	// produced no usable result. The in-flight search stays alive — only the
	// bot changes — so result-cache waiters keep waiting.
	canRetry := attemptedPrimary && !st.retried &&
		fallback != "" && !strings.EqualFold(fallback, h.srv.config.SearchBot)
	if canRetry {
		st.bot = fallback
		st.phase = searchAwaitingResponse
		st.retried = true
		h.currentMu.Unlock()

		// Dispatch the same query to the fallback bot. SendMessage is a
		// no-op when the connection is down; the watchdog will simply time
		// out again on the fallback attempt.
		core.SearchBook(h.irc, fallback, st.query)
		h.srv.logBuf.info(fmt.Sprintf("CLIENT (%s): 🔁 IRC SEARCH retry → %q via %s", st.session.username, st.query, fallback))

		waited := "a response"
		if phase == searchAwaitingResult {
			waited = "the results file"
		}
		msg := fmt.Sprintf("IRC search timed out waiting for %s from %q — retrying with %q (query: %q)", waited, attemptedBot, fallback, st.query)
		h.srv.log.Printf("IRC HUB: %s\n", msg)
		h.srv.logBuf.warn(msg)
		broadcastToClients(st.session.getClients(), newStatusResponse(NOTIFY,
			fmt.Sprintf("%s didn't respond — trying %s…", attemptedBot, fallback)))
		if enteredDegraded {
			h.announceDegraded()
		}
		return true
	}

	h.current = nil
	h.currentMu.Unlock()

	waited := "a response"
	if phase == searchAwaitingResult {
		waited = "the results file"
	}
	msg := fmt.Sprintf("IRC search timed out waiting for %s from %q (query: %q)", waited, attemptedBot, st.query)
	h.srv.log.Printf("IRC HUB: %s\n", msg)
	h.srv.logBuf.warn(msg)
	broadcastToClients(st.session.getClients(), newErrorResponse(
		fmt.Sprintf("Search bot %q isn't responding — it may be down. Try again later.", attemptedBot)))
	h.srv.resultCache.CancelInFlight(st.query)

	if enteredDegraded {
		h.announceDegraded()
	}
	return false
}

// recordSearchOutcome updates bot-health state after a search reaches a
// terminal state. Only a resolution on the primary bot clears the failure
// streak (and can trigger recovery); fallback resolutions leave the primary's
// presumed-down state untouched until a probe proves otherwise.
func (h *ircHub) recordSearchOutcome(st *inflightSearch) (recovered bool) {
	h.currentMu.Lock()
	defer h.currentMu.Unlock()
	if strings.EqualFold(st.bot, h.srv.config.SearchBot) {
		h.primaryTimeouts = 0
		if h.searchDegraded {
			h.searchDegraded = false
			h.degradedUntil = time.Time{}
			recovered = true
		}
	}
	return recovered
}

// searchProgress advances the in-flight search to the result-wait phase when
// the bot acknowledges the query ("accepted" / "matches found" notices).
func (h *ircHub) searchProgress() {
	h.currentMu.Lock()
	defer h.currentMu.Unlock()
	st := h.current
	if st == nil || st.phase != searchAwaitingResponse {
		return
	}
	st.phase = searchAwaitingResult
	select {
	case st.rearm <- struct{}{}:
	default:
	}
}

// peekCurrentSearch returns the in-flight search without clearing it.
func (h *ircHub) peekCurrentSearch() *inflightSearch {
	h.currentMu.Lock()
	defer h.currentMu.Unlock()
	return h.current
}

// popCurrentSearchForFile clears the in-flight search — stopping its watchdog —
// only when the results filename plausibly belongs to its query. stale=true
// means the filename belongs to a different (usually timed-out) query: the
// in-flight search is left intact so the real response can still resolve it.
func (h *ircHub) popCurrentSearchForFile(filename string) (clients []*Client, st *inflightSearch, stale bool) {
	h.currentMu.Lock()
	defer h.currentMu.Unlock()
	st = h.current
	if st == nil {
		return nil, nil, false
	}
	if filename != "" && !core.QueryMatchesResultsFilename(st.query, filename) {
		return nil, nil, true
	}
	h.current = nil
	close(st.done)
	clients = st.session.getClients()
	return clients, st, false
}

// popCurrentSearch clears the in-flight search state — stopping its watchdog —
// and returns its clients along with the search record.
func (h *ircHub) popCurrentSearch() (clients []*Client, st *inflightSearch) {
	h.currentMu.Lock()
	defer h.currentMu.Unlock()
	st = h.current
	h.current = nil
	if st != nil {
		close(st.done)
		clients = st.session.getClients()
	}
	return
}

// announceDegraded logs and broadcasts the one-time warning emitted when the
// primary search bot is marked down.
func (h *ircHub) announceDegraded() {
	primary := h.srv.config.SearchBot
	fallback := strings.TrimSpace(h.srv.config.SearchBotFallback)
	msg := fmt.Sprintf("Search bot %q has not responded to the last %d searches — it may be down.", primary, searchDegradedThreshold)
	if fallback != "" && !strings.EqualFold(fallback, primary) {
		msg += fmt.Sprintf(" Routing searches to %q for %s.", fallback, searchDegradedCooldown)
	}
	h.srv.log.Printf("IRC HUB: %s\n", msg)
	h.srv.logBuf.warn(msg)
	h.broadcastAll(newStatusResponse(WARNING, msg))
}

// announceRecovery logs and broadcasts the notification emitted when the
// primary search bot starts responding again.
func (h *ircHub) announceRecovery() {
	msg := fmt.Sprintf("Primary search bot %q is responding again.", h.srv.config.SearchBot)
	h.srv.log.Printf("IRC HUB: %s\n", msg)
	h.srv.logBuf.info(msg)
	h.broadcastAll(newStatusResponse(NOTIFY, msg))
}

// broadcastAll sends a message to every connected client of every session.
func (h *ircHub) broadcastAll(msg interface{}) {
	h.srv.sessionsMu.RLock()
	defer h.srv.sessionsMu.RUnlock()
	for _, sess := range h.srv.sessions {
		broadcastToClients(sess.getClients(), msg)
	}
}

func (h *ircHub) searchResultHandler() core.HandlerFunc {
	return func(text string) {
		var dccFilename string
		if d, parseErr := dcc.ParseString(text); parseErr == nil {
			dccFilename = d.Filename
			h.srv.logBuf.info(fmt.Sprintf("DCC SEND: %s -> %s:%s (%d bytes)", d.Filename, d.IP, d.Port, d.Size))
		} else {
			h.srv.logBuf.warn(fmt.Sprintf("Search DCC string unreadable (%v) — attempting download anyway", parseErr))
		}

		// Stale-result guard + pop, atomically: a results file belonging to a
		// different (usually timed-out) query must not be attributed to the
		// in-flight search, and the check must not race a newer dispatch.
		clients, st, stale := h.popCurrentSearchForFile(dccFilename)
		if stale {
			if cur := h.peekCurrentSearch(); cur != nil {
				h.srv.logBuf.warn(fmt.Sprintf("Ignoring stale search result %q — in-flight search is %q", dccFilename, cur.query))
			}
			return
		}
		if st == nil {
			h.srv.logBuf.info("Ignoring late search result — no search in flight")
			return
		}
		if h.recordSearchOutcome(st) {
			h.announceRecovery()
		}
		query := st.query

		extractedPath, err := core.DownloadExtractDCCString(h.srv.config.DownloadDir, text, nil)
		if err != nil {
			h.srv.logBuf.error(fmt.Sprintf("Search download failed: %v", err))
			broadcastToClients(clients, newErrorResponse("Error when downloading search results."))
			h.srv.resultCache.CancelInFlight(query)
			return
		}

		bookResults, parseErrors, err := core.ParseSearchFile(extractedPath)
		if err != nil {
			broadcastToClients(clients, newErrorResponse("Error when parsing search results."))
			h.srv.resultCache.CancelInFlight(query)
			return
		}
		rawResults, _ := os.ReadFile(extractedPath)

		if len(bookResults) == 0 && len(parseErrors) == 0 {
			h.srv.logBuf.info("IRC: no results returned for query")
			broadcastToClients(clients, newErrorResponse("No results found for the query."))
			h.srv.resultCache.CancelInFlight(query)
			return
		}

		h.srv.logBuf.info(fmt.Sprintf("🔍 Search results: %d found, %d unparseable", len(bookResults), len(parseErrors)))
		broadcastToClients(clients, newSearchResponse(bookResults, parseErrors, string(rawResults)))
		os.Remove(extractedPath)

		if query != "" {
			h.srv.resultCache.Resolve(query, bookResults, parseErrors, h.irc.Username)
			h.srv.searchHistory.Add(query)
		}

		st.session.query = query
	}
}

func (h *ircHub) bookResultHandler() core.HandlerFunc {
	return func(text string) {
		var handle *sharedSlotHandle
		select {
		case handle = <-h.pendingSlots:
		default:
		}

		var sess *session
		if handle != nil {
			sess = handle.session
		}

		dir := h.srv.config.DownloadDir

		if err := staging.EnsureStagingDir(dir); err != nil {
			h.srv.logBuf.error("Failed to create staging directory.")
			if sess != nil {
				broadcastToClients(sess.getClients(), newDownloadFailedResponse("Failed to prepare download directory."))
			}
			if handle != nil {
				handle.release()
			}
			return
		}
		stage := staging.StagingDir(dir)

		if sess != nil {
			broadcastToClients(sess.getClients(), newDownloadWaitingClear())
			broadcastToClients(sess.getClients(), newDownloadStartedResponse())
		}

		var ircFilenamePreview string
		if d, err := dcc.ParseString(text); err == nil {
			ircFilenamePreview = d.Filename
		}
		group := ircFilenamePreview
		if group == "" {
			group = fmt.Sprintf("dl-%d", time.Now().UnixMilli())
		}
		sess_lb := h.srv.logBuf.session(group)
		sess_lb.info(fmt.Sprintf("⬇️  Downloading: %s", ircFilenamePreview))

		extractedPath, err := core.DownloadExtractDCCString(stage, text, nil)
		if err != nil {
			sess_lb.error(fmt.Sprintf("Download failed: %v", err))
			if sess != nil {
				broadcastToClients(sess.getClients(), newDownloadFailedResponse("Error when downloading book."))
			}
			if handle != nil {
				handle.release()
			}
			return
		}

		if handle != nil {
			handle.release()
		}

		size := fileSizeMB(extractedPath)
		ircFilename := filepath.Base(extractedPath)
		sess_lb.infoDetail(
			fmt.Sprintf("📥 Downloaded: %s (%s)", ircFilename, size),
			fmt.Sprintf("File: %s\nSize: %s\nStaged at: %s", ircFilename, size, extractedPath),
		)

		var stagedOriginalPath string
		if h.srv.config.DevMode {
			stagedOriginalPath = staging.OriginalCopyPath(extractedPath)
			if err := staging.CopyFile(extractedPath, stagedOriginalPath); err != nil {
				sess_lb.warn(fmt.Sprintf("Could not preserve original download: %v", err))
				stagedOriginalPath = ""
			}
		}

		config := h.srv.config
		if len(config.PostProcessCmd) > 0 {
			if sess != nil {
				broadcastToClients(sess.getClients(), newPostProcessStartedResponse())
			}
		}
		runPostProcess(config.PostProcessCmd, extractedPath, sess_lb)

		var meta *core.EPUBMetadata
		var coverBase64, coverMime string
		if strings.EqualFold(filepath.Ext(extractedPath), ".epub") {
			if m, err := core.ReadEPUBMetadata(extractedPath); err == nil {
				meta = m
			}
			if imgBytes, mime, err := core.ExtractCoverImage(extractedPath); err == nil && imgBytes != nil {
				coverBase64 = base64.StdEncoding.EncodeToString(imgBytes)
				coverMime = mime
			}
		}

		options := staging.BuildOptions(ircFilename, meta, config.ReplaceSpace)

		saveToStaged := func() {
			staged := &StagedBook{
				ID:           uuid.New().String(),
				StagedPath:   extractedPath,
				IRCFilename:  ircFilename,
				Metadata:     meta,
				Options:      options,
				ReplaceSpace: config.ReplaceSpace,
				CoverBase64:  coverBase64,
				CoverMime:    coverMime,
				StagedAt:     time.Now(),
			}
			if err := h.srv.stagedBooks.Add(staged); err != nil {
				os.Remove(extractedPath)
			}
			if stagedOriginalPath != "" {
				os.Remove(stagedOriginalPath)
			}
			h.srv.broadcastStagedCount()
		}

		if sess == nil {
			saveToStaged()
			return
		}

		select {
		case <-sess.renameMu:
			defer func() { sess.renameMu <- struct{}{} }()
		case <-sess.ctx.Done():
			saveToStaged()
			return
		}

		if config.AutoRename {
			choice := autoRenameChoice(config.AutoRenameOption, options, meta)
			finalPath := staging.ResolveFinalPath(dir, choice, ircFilename, meta, config.ReplaceSpace)
			if staging.FileExists(finalPath) {
				finalPath = staging.UniquePath(finalPath)
				rel, _ := filepath.Rel(dir, finalPath)
				sess_lb.warn(fmt.Sprintf("⚠️  Auto-rename conflict — using: %s", filepath.ToSlash(rel)))
			}
			sess_lb.info(fmt.Sprintf("🤖 Auto-renamed [%s]: %s", choice.OptionID, ircFilename))
			finalizeRename(choice, options, meta, ircFilename, extractedPath, stagedOriginalPath, finalPath, dir, config, sess_lb, sess, h.srv.seriesRegistry)
			return
		}

		currentClient := sess.getAnyClient()
		if currentClient == nil {
			saveToStaged()
			return
		}

		safeSend(currentClient, RenamePromptResponse{
			StatusResponse: StatusResponse{
				MessageType:      RENAME_PROMPT,
				NotificationType: NOTIFY,
				Title:            "Book downloaded — how would you like to save it?",
			},
			IRCFilename:  ircFilename,
			Metadata:     meta,
			Options:      options,
			ReplaceSpace: config.ReplaceSpace,
			CoverBase64:  coverBase64,
			CoverMime:    coverMime,
		})

		var choice RenameChoice
		select {
		case choice = <-currentClient.renameConfirm:
		case <-time.After(30 * time.Minute):
			sess_lb.warn(fmt.Sprintf("Rename timed out — keeping IRC filename: %s", ircFilename))
			choice = RenameChoice{OptionID: "keep"}
		case <-currentClient.ctx.Done():
			saveToStaged()
			return
		}

		if choice.OptionID == "queue_later" {
			saveToStaged()
			return
		}

		for {
			finalPath := staging.ResolveFinalPath(dir, choice, ircFilename, meta, config.ReplaceSpace)
			if !staging.FileExists(finalPath) || choice.Force {
				finalizeRename(choice, options, meta, ircFilename, extractedPath, stagedOriginalPath, finalPath, dir, config, sess_lb, sess, h.srv.seriesRegistry)
				return
			}

			rel, _ := filepath.Rel(dir, finalPath)
			sess_lb.warn(fmt.Sprintf("⚠️  Rename conflict: %s already exists", filepath.ToSlash(rel)))
			safeSend(currentClient, newFileConflictResponse(
				ircFilename, meta, options, config.ReplaceSpace,
				coverBase64, coverMime, filepath.ToSlash(rel), "",
			))

			select {
			case choice = <-currentClient.renameConfirm:
			case <-time.After(30 * time.Minute):
				sess_lb.warn(fmt.Sprintf("Rename conflict timed out — keeping staged: %s", ircFilename))
				saveToStaged()
				return
			case <-currentClient.ctx.Done():
				saveToStaged()
				return
			}

			if choice.OptionID == "queue_later" {
				saveToStaged()
				return
			}
		}
	}
}

func (h *ircHub) noResultsHandler() core.HandlerFunc {
	return func(_ string) {
		clients, st := h.popCurrentSearch()
		if st == nil {
			h.srv.logBuf.info("Ignoring late 'no results' notice — no search in flight")
			return
		}
		if h.recordSearchOutcome(st) {
			h.announceRecovery()
		}
		h.srv.logBuf.info("IRC: no results returned for query")
		broadcastToClients(clients, newErrorResponse("No results found for the query."))
		h.srv.resultCache.CancelInFlight(st.query)
	}
}

func (h *ircHub) badServerHandler() core.HandlerFunc {
	return func(_ string) {
		clients, st := h.popCurrentSearch()
		if st == nil {
			h.srv.logBuf.info("Ignoring late 'server unavailable' notice — no search in flight")
			return
		}
		if h.recordSearchOutcome(st) {
			h.announceRecovery()
		}
		h.srv.logBuf.warn("IRC: server unavailable, try another")
		broadcastToClients(clients, newErrorResponse("Server is not available. Try another one."))
		h.srv.resultCache.CancelInFlight(st.query)
	}
}

func (h *ircHub) searchAcceptedHandler() core.HandlerFunc {
	return func(_ string) {
		h.searchProgress()
		h.srv.logBuf.info("IRC: search accepted by bot")
		if st := h.peekCurrentSearch(); st != nil {
			broadcastToClients(st.session.getClients(), newStatusResponse(NOTIFY, "Search accepted into the queue."))
		}
	}
}

func (h *ircHub) matchesFoundHandler() core.HandlerFunc {
	return func(num string) {
		h.searchProgress()
		h.srv.logBuf.info(fmt.Sprintf("IRC: %s matches found", num))
		if st := h.peekCurrentSearch(); st != nil {
			broadcastToClients(st.session.getClients(), newStatusResponse(NOTIFY, fmt.Sprintf("Found %s results for your query.", num)))
		}
	}
}

func (h *ircHub) serverListHandler() core.HandlerFunc {
	return func(text string) {
		servers := core.ParseServers(text)

		h.serverListMu.Lock()
		changed := !slices.Equal(h.lastElevated, servers.ElevatedUsers)
		h.lastElevated = servers.ElevatedUsers
		h.serverListMu.Unlock()

		if changed {
			h.srv.log.Printf("IRC hub: #ebooks — elevated: %v\n", servers.ElevatedUsers)
			h.srv.logBuf.info(fmt.Sprintf("📋 Server list updated: %d elevated, %d regular", len(servers.ElevatedUsers), len(servers.RegularUsers)))
		}

		h.srv.sessionsMu.RLock()
		defer h.srv.sessionsMu.RUnlock()
		for _, sess := range h.srv.sessions {
			sess.setServerList(servers)
			broadcastToClients(sess.getClients(), newServerListResponse(servers))
		}
	}
}

func (h *ircHub) ircMessageHandler() core.HandlerFunc {
	return func(text string) {
		h.srv.sessionsMu.RLock()
		defer h.srv.sessionsMu.RUnlock()
		for _, sess := range h.srv.sessions {
			for _, c := range sess.getClients() {
				if c.ircSubscribed.Load() {
					safeSend(c, newIrcMessageResponse(text))
				}
			}
		}
	}
}

func (h *ircHub) channelBroadcastError(message string) core.HandlerFunc {
	return func(_ string) {
		h.srv.logBuf.error(fmt.Sprintf("IRC: channel error — %s", message))
		h.srv.sessionsMu.RLock()
		defer h.srv.sessionsMu.RUnlock()
		for _, sess := range h.srv.sessions {
			broadcastToClients(sess.getClients(), newErrorResponse(message))
		}
	}
}

package server

import (
	"context"
	"encoding/base64"
	"fmt"
	"log"
	"os"
	"path/filepath"
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

// ircHub owns the single shared IRC connection for all frontend sessions.
// All search and download requests are routed through this hub.
type ircHub struct {
	irc           *irc.Conn
	ctx           context.Context
	searchQueue   chan sharedSearchJob    // cap 50
	downloadQueue chan sharedDownloadJob  // cap 100
	downloadSlots chan struct{}           // semaphore, cap concurrentDownloads
	pendingSlots  chan *sharedSlotHandle  // FIFO for DCC result routing

	// currentSearch is the session waiting for the in-flight search result.
	// Only processSearchQueue writes it; read under currentMu.
	currentSearch *session
	currentQuery  string
	currentMu     sync.Mutex

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

// clearAndNotifyCurrentSearch clears currentSearch and notifies its clients that
// the connection dropped so they don't wait indefinitely.
func (h *ircHub) clearAndNotifyCurrentSearch() {
	h.currentMu.Lock()
	sess := h.currentSearch
	query := h.currentQuery
	h.currentSearch = nil
	h.currentQuery = ""
	h.currentMu.Unlock()

	if sess != nil {
		h.srv.resultCache.CancelInFlight(query)
		broadcastToClients(sess.getClients(), newErrorResponse("IRC connection lost — please retry your search."))
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

			h.currentMu.Lock()
			h.currentSearch = job.session
			h.currentQuery = job.query
			h.currentMu.Unlock()

			core.SearchBook(h.irc, h.srv.config.SearchBot, job.query)
			lastSearch = time.Now()

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

// currentSearchClients returns the clients belonging to the session currently
// waiting for a search result, and clears the current search state.
func (h *ircHub) popCurrentSearch() (clients []*Client, sess *session, query string) {
	h.currentMu.Lock()
	defer h.currentMu.Unlock()
	sess = h.currentSearch
	query = h.currentQuery
	h.currentSearch = nil
	h.currentQuery = ""
	if sess != nil {
		clients = sess.getClients()
	}
	return
}

func (h *ircHub) searchResultHandler() core.HandlerFunc {
	return func(text string) {
		clients, sess, query := h.popCurrentSearch()

		if d, parseErr := dcc.ParseString(text); parseErr == nil {
			h.srv.logBuf.info(fmt.Sprintf("DCC SEND: %s -> %s:%s (%d bytes)", d.Filename, d.IP, d.Port, d.Size))
		} else {
			h.srv.logBuf.warn(fmt.Sprintf("Search DCC string unreadable (%v) — attempting download anyway", parseErr))
		}

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

		if sess != nil {
			sess.query = query
		}
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
		clients, _, query := h.popCurrentSearch()
		h.srv.logBuf.info("IRC: no results returned for query")
		broadcastToClients(clients, newErrorResponse("No results found for the query."))
		h.srv.resultCache.CancelInFlight(query)
	}
}

func (h *ircHub) badServerHandler() core.HandlerFunc {
	return func(_ string) {
		clients, _, query := h.popCurrentSearch()
		h.srv.logBuf.warn("IRC: server unavailable, try another")
		broadcastToClients(clients, newErrorResponse("Server is not available. Try another one."))
		h.srv.resultCache.CancelInFlight(query)
	}
}

func (h *ircHub) searchAcceptedHandler() core.HandlerFunc {
	return func(_ string) {
		h.currentMu.Lock()
		sess := h.currentSearch
		h.currentMu.Unlock()
		h.srv.logBuf.info("IRC: search accepted by bot")
		if sess != nil {
			broadcastToClients(sess.getClients(), newStatusResponse(NOTIFY, "Search accepted into the queue."))
		}
	}
}

func (h *ircHub) matchesFoundHandler() core.HandlerFunc {
	return func(num string) {
		h.currentMu.Lock()
		sess := h.currentSearch
		h.currentMu.Unlock()
		h.srv.logBuf.info(fmt.Sprintf("IRC: %s matches found", num))
		if sess != nil {
			broadcastToClients(sess.getClients(), newStatusResponse(NOTIFY, fmt.Sprintf("Found %s results for your query.", num)))
		}
	}
}

func (h *ircHub) serverListHandler() core.HandlerFunc {
	return func(text string) {
		servers := core.ParseServers(text)
		h.srv.log.Printf("IRC hub: #ebooks — elevated: %v | regular: %v\n", servers.ElevatedUsers, servers.RegularUsers)
		h.srv.logBuf.info(fmt.Sprintf("📋 Server list updated: %d elevated, %d regular", len(servers.ElevatedUsers), len(servers.RegularUsers)))

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

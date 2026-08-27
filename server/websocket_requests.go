package server

import (
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"time"

	"github.com/jeeftor/openbooks/core"
	"github.com/jeeftor/openbooks/staging"
	"github.com/jeeftor/openbooks/util"
)

// errQueueLater is a sentinel returned by processStagedBookChoice when the user
// chose to defer the book ("queue_later"). The caller should return/continue
// without broadcasting — no file was moved.
var errQueueLater = errors.New("queued for later")

// errFileConflict is a sentinel returned by processStagedBookChoice when the
// destination path already exists and the user didn't force an overwrite. A
// FILE_CONFLICT response has already been sent to the client; the caller
// should wait for the next renameConfirm and retry.
var errFileConflict = errors.New("file conflict")

// RequestHandler defines a generic handle() method that is called when a specific request type is made
type RequestHandler interface {
	handle(c *Client)
}

// messageRouter is used to parse the incoming request and respond appropriately
func (server *server) routeMessage(message Request, c *Client) {
	var obj interface{}

	// Messages with no payload (CONNECT, PROCESS_STAGED_BOOKS) skip unmarshalling.
	switch message.MessageType {
	case CONNECT:
		c.startIrcConnection(server)
		return
	case PROCESS_STAGED_BOOKS:
		go c.handleProcessStagedBooks(server)
		return
	case GET_STAGED_LIST:
		c.handleGetStagedList(server)
		return
	case HISTORY_CLEAR:
		server.searchHistory.Clear()
		return
	}

	switch message.MessageType {
	case SEARCH:
		obj = new(SearchRequest)
	case DOWNLOAD:
		obj = new(DownloadRequest)
	case RENAME_CONFIRM:
		obj = new(RenameConfirmRequest)
	case STAGED_QUEUE_LATER:
		obj = new(StageQueueLaterRequest)
	case DELETE_STAGED:
		obj = new(DeleteStagedRequest)
	case PROCESS_ONE_STAGED:
		obj = new(ProcessOneStagedRequest)
	case HISTORY_DELETE:
		obj = new(HistoryDeleteRequest)
	default:
		server.log.Println("Unknown request type received.")
		return
	}

	if err := json.Unmarshal(message.Payload, &obj); err != nil {
		server.log.Printf("Invalid request payload. %s.\n", err.Error())
		c.send <- StatusResponse{
			MessageType:      STATUS,
			NotificationType: DANGER,
			Title:            "Unknown request payload.",
		}
		return
	}

	switch message.MessageType {
	case SEARCH:
		c.sendSearchRequest(obj.(*SearchRequest), server)
	case DOWNLOAD:
		c.sendDownloadRequest(obj.(*DownloadRequest), server)
	case RENAME_CONFIRM:
		c.handleRenameConfirm(obj.(*RenameConfirmRequest), server)
	case STAGED_QUEUE_LATER:
		c.handleStageQueueLater(obj.(*StageQueueLaterRequest))
	case DELETE_STAGED:
		c.handleDeleteStaged(obj.(*DeleteStagedRequest), server)
	case PROCESS_ONE_STAGED:
		go c.handleProcessOneStaged(obj.(*ProcessOneStagedRequest), server)
	case HISTORY_DELETE:
		server.searchHistory.Delete(obj.(*HistoryDeleteRequest).Timestamp)
	}
}

// startIrcConnection handles the CONNECT message. For new sessions it connects to IRC;
// for reconnecting sessions the IRC is already running so we just send the welcome response.
func (c *Client) startIrcConnection(server *server) {
	defer func() {
		if r := recover(); r != nil {
			c.log.Printf("Recovered from panic in startIrcConnection: %v", r)
		}
	}()

	sess := server.getSession(c.uuid)
	if sess == nil {
		safeSend(c, newErrorResponse("Session not found."))
		return
	}

	if !sess.irc.IsConnected() {
		// First connection for this session — connect to IRC.
		if err := core.Join(sess.irc, server.config.Server, server.config.EnableTLS); err != nil {
			c.log.Println(err)
			server.logBuf.error(fmt.Sprintf("IRC connect failed: %v", err))
			safeSend(c, newErrorResponse("Unable to connect to IRC server."))
			// Still notify about any staged books — they're available regardless of IRC.
			if count := server.stagedBooks.Count(); count > 0 {
				safeSend(c, newStagedBooksNotifyResponse(count))
			}
			return
		}

		server.logBuf.info(fmt.Sprintf("🔌 IRC connected: %s", sess.username))
		handler := server.NewIrcEventHandler(sess)

		if server.config.Log {
			logger, _, err := util.CreateLogFile(sess.username, server.config.DownloadDir)
			if err != nil {
				server.log.Println(err)
			}
			handler[core.Message] = func(text string) { logger.Println(text) }
		}

		go core.StartReader(sess.ctx, sess.irc, handler)
		go sess.processSearchQueue(server)
		go sess.processDownloadQueue(server)
		go sess.refreshServerList()
	}
	// else: reconnecting — IRC and both queues are already running.

	safeSend(c, ConnectionResponse{
		StatusResponse: StatusResponse{
			MessageType:      CONNECT,
			NotificationType: SUCCESS,
			Title:            "Welcome, connection established.",
			Detail:           fmt.Sprintf("IRC username %s", sess.username),
		},
		Name: sess.username,
	})

	// Notify about staged books waiting to be processed.
	if count := server.stagedBooks.Count(); count > 0 {
		safeSend(c, newStagedBooksNotifyResponse(count))
	}

	// Send series autocomplete data.
	safeSend(c, newSeriesAutocompleteResponse(server.seriesRegistry.All()))

	// Send server-side search history.
	safeSend(c, HistoryListResponse{
		StatusResponse: StatusResponse{MessageType: HISTORY_LIST, NotificationType: NOTIFY},
		Entries:        server.searchHistory.All(),
	})
}

// safeSend attempts to send on the client channel, recovering from panic if channel is closed.
// A nil client is silently ignored (used when no browser is connected).
func safeSend(c *Client, msg interface{}) {
	if c == nil {
		return
	}
	defer func() {
		if r := recover(); r != nil {
			c.log.Printf("Channel closed, message not sent: %v", r)
		}
	}()
	select {
	case c.send <- msg:
		// sent successfully
	case <-c.ctx.Done():
		// context cancelled, client is being shut down
	}
}

// broadcastToClients sends a message to all clients in the provided slice.
// The clients slice should come from sess.getClients().
func broadcastToClients(clients []*Client, msg interface{}) {
	if clients == nil {
		return
	}
	for _, c := range clients {
		safeSend(c, msg)
	}
}

// sendSearchRequest checks the server-side result cache and either returns cached
// results immediately, subscribes to an in-flight IRC query, or enqueues a new search.
func (c *Client) sendSearchRequest(s *SearchRequest, server *server) {
	sess := server.getSession(c.uuid)
	if sess == nil {
		return
	}

	cached, ch, fromCache := server.resultCache.GetOrSubscribe(s.Query)
	if fromCache {
		// Serve fresh cached result immediately.
		cachedAt := cached.Timestamp
		resp := newSearchResponse(cached.Books, cached.Errors, "")
		resp.CachedAt = &cachedAt
		c.send <- resp
		server.searchHistory.Add(s.Query)
		server.logBuf.info(fmt.Sprintf("📋 Cache hit for %q (age: %.0fm)", s.Query, time.Since(cachedAt).Minutes()))
		return
	}

	if ch != nil {
		// Same query already in-flight — wait for the result rather than firing duplicate IRC.
		server.logBuf.info(fmt.Sprintf("🔁 Deduped search for %q — waiting for in-flight result", s.Query))
		c.send <- newStatusResponse(NOTIFY, "Search in progress — waiting for results.")
		go func() {
			select {
			case result := <-ch:
				if result != nil {
					cachedAt := result.Timestamp
					resp := newSearchResponse(result.Books, result.Errors, "")
					resp.CachedAt = &cachedAt
					safeSend(c, resp)
				}
			case <-c.ctx.Done():
			}
		}()
		return
	}

	// New query — enqueue to IRC.
	pending := len(sess.searchQueue)
	if pending > 0 {
		c.log.Printf("Search queued (position %d): %q\n", pending+1, s.Query)
		c.send <- newStatusResponse(NOTIFY, fmt.Sprintf("Search queued (position %d).", pending+1))
	} else {
		c.log.Printf("Search queued: %q\n", s.Query)
		c.send <- newStatusResponse(NOTIFY, "Search queued.")
	}

	select {
	case sess.searchQueue <- searchJob{query: s.Query}:
	default:
		// Queue is full — cancel the in-flight mark so subscribers aren't left waiting.
		server.resultCache.CancelInFlight(s.Query)
		c.send <- newStatusResponse(WARNING, "Search queue is full. Please wait before searching again.")
	}
}

// handleRenameConfirm forwards the user's rename decision to the waiting bookResultHandler,
// or processes a staged book if StagedID is set.
func (c *Client) handleRenameConfirm(req *RenameConfirmRequest, server *server) {
	if req.StagedID != "" {
		c.handleStagedRenameConfirm(req, server)
		return
	}

	choice := RenameChoice{
		OptionID:        req.OptionID,
		CustomName:      req.CustomName,
		FileName:        req.FileName,
		RewriteMetadata: req.RewriteMetadata,
		Author:          req.Author,
		Title:           req.Title,
		Series:          req.Series,
		SeriesIndex:     req.SeriesIndex,
		Force:           req.Force,
	}
	select {
	case c.renameConfirm <- choice:
	default:
		c.log.Println("handleRenameConfirm: no pending rename awaiting confirmation")
	}
}

// handleGetStagedList sends the full list of staged books to the client.
func (c *Client) handleGetStagedList(server *server) {
	all := server.stagedBooks.All()
	summaries := make([]StagedBookSummary, len(all))
	for i, b := range all {
		summaries[i] = StagedBookSummary{
			ID:          b.ID,
			IRCFilename: b.IRCFilename,
			Metadata:    b.Metadata,
			CoverBase64: b.CoverBase64,
			CoverMime:   b.CoverMime,
			StagedAt:    b.StagedAt.Format("2006-01-02T15:04:05Z"),
		}
	}
	safeSend(c, StagedBooksListResponse{
		StatusResponse: StatusResponse{
			MessageType:      STAGED_BOOKS_LIST,
			NotificationType: NOTIFY,
		},
		Books: summaries,
	})
}

// processStagedBookChoice moves a staged book to its final path based on the
// user's rename choice, optionally rewrites EPUB metadata, and removes the
// book from the staged store. Returns nil on success, an error if the move
// fails. If choice.OptionID is "queue_later", sends a status message and
// returns errQueueLater so the caller can return/continue without broadcasting.
// If the destination already exists and choice.Force is false, sends a
// FILE_CONFLICT response and returns errFileConflict so channel-based callers
// can wait for the user's next decision.
func (c *Client) processStagedBookChoice(server *server, staged *StagedBook, choice RenameChoice) error {
	if choice.OptionID == "queue_later" {
		safeSend(c, newStatusResponse(NOTIFY, "Book saved for later."))
		return errQueueLater
	}

	finalPath := staging.ResolveFinalPath(server.config.DownloadDir, choice, staged.IRCFilename, staged.Metadata, staged.ReplaceSpace)

	// Conflict check: if the destination exists and the user didn't force
	// overwrite, send FILE_CONFLICT and let the caller wait for a new choice.
	if !choice.Force && staging.FileExists(finalPath) {
		rel, _ := filepath.Rel(server.config.DownloadDir, finalPath)
		safeSend(c, newFileConflictResponse(
			staged.IRCFilename, staged.Metadata, staged.Options, staged.ReplaceSpace,
			staged.CoverBase64, staged.CoverMime, filepath.ToSlash(rel), staged.ID,
		))
		return errFileConflict
	}

	if err := staging.MoveFile(staged.StagedPath, finalPath); err != nil {
		safeSend(c, newErrorResponse(fmt.Sprintf("Move failed: %v", err)))
		return err
	}

	if choice.RewriteMetadata {
		if err := staging.RewriteEPUBMetadata(finalPath, choice.Title, choice.Author, choice.Series, choice.SeriesIndex, choice.ClearSeries, choice.ClearSeriesIndex); err != nil {
			c.log.Printf("RewriteEPUBMetadata: %v", err)
		}
	}

	if choice.Series != "" {
		server.seriesRegistry.AddIfNew(choice.Series)
	}

	server.stagedBooks.Remove(staged.ID)
	safeSend(c, newDownloadResponse(finalPath, server.config.DownloadDir))
	return nil
}

// handleProcessOneStaged sends a STAGED_BOOK_RESUME for a single specific book,
// then waits for the user's rename decision and processes it.
func (c *Client) handleProcessOneStaged(req *ProcessOneStagedRequest, server *server) {
	staged, ok := server.stagedBooks.Get(req.StagedID)
	if !ok {
		safeSend(c, newErrorResponse("Staged book not found."))
		return
	}
	safeSend(c, StagedBookResumeResponse{
		StatusResponse: StatusResponse{
			MessageType:      STAGED_BOOK_RESUME,
			NotificationType: NOTIFY,
			Title:            "How would you like to save this book?",
		},
		StagedID:      staged.ID,
		IRCFilename:   staged.IRCFilename,
		Metadata:      staged.Metadata,
		Options:       staged.Options,
		ReplaceSpace:  staged.ReplaceSpace,
		CoverBase64:   staged.CoverBase64,
		CoverMime:     staged.CoverMime,
		StagedAt:      staged.StagedAt,
		QueuePosition: 1,
		TotalQueued:   1,
	})

	var choice RenameChoice
	select {
	case choice = <-c.renameConfirm:
	case <-time.After(30 * time.Minute):
		return
	case <-c.ctx.Done():
		return
	}

	for {
		err := c.processStagedBookChoice(server, staged, choice)
		if err == nil {
			server.broadcastStagedCount()
			return
		}
		if err == errQueueLater {
			return
		}
		if err == errFileConflict {
			// FILE_CONFLICT already sent — wait for the user's next decision.
			select {
			case choice = <-c.renameConfirm:
			case <-time.After(30 * time.Minute):
				return
			case <-c.ctx.Done():
				return
			}
			continue
		}
		// Other error (move failure etc.) — already reported to client.
		return
	}
}

// handleDeleteStaged permanently deletes a staged file from disk and removes it from the registry.
func (c *Client) handleDeleteStaged(req *DeleteStagedRequest, server *server) {
	staged, ok := server.stagedBooks.Get(req.StagedID)
	if !ok {
		safeSend(c, newErrorResponse("Staged book not found."))
		return
	}
	if err := os.Remove(staged.StagedPath); err != nil && !os.IsNotExist(err) {
		safeSend(c, newErrorResponse(fmt.Sprintf("Delete failed: %v", err)))
		return
	}
	server.stagedBooks.Remove(req.StagedID)
	safeSend(c, newStatusResponse(SUCCESS, fmt.Sprintf("Deleted %q.", staged.IRCFilename)))
	server.broadcastStagedCount()
}

// handleStageQueueLater re-queues the current staged book (or live rename) for later.
func (c *Client) handleStageQueueLater(req *StageQueueLaterRequest) {
	if req.StagedID != "" {
		// Already staged; nothing to do server-side — client just dismissed the modal.
		safeSend(c, newStatusResponse(NOTIFY, "Book saved for later."))
		return
	}
	// For a live rename prompt, send "queue_later" through the rename confirm channel
	// so bookResultHandler saves it to the staged store.
	select {
	case c.renameConfirm <- RenameChoice{OptionID: "queue_later"}:
	default:
		c.log.Println("handleStageQueueLater: no pending rename to defer")
	}
}

// handleProcessStagedBooks iterates all staged books and sends each as a STAGED_BOOK_RESUME.
// It blocks on renameConfirm for each book. Run in a separate goroutine.
func (c *Client) handleProcessStagedBooks(server *server) {
	all := server.stagedBooks.All()
	if len(all) == 0 {
		safeSend(c, newStatusResponse(NOTIFY, "No staged books to process."))
		return
	}
	total := len(all)
	for i, staged := range all {
		safeSend(c, StagedBookResumeResponse{
			StatusResponse: StatusResponse{
				MessageType:      STAGED_BOOK_RESUME,
				NotificationType: NOTIFY,
				Title:            fmt.Sprintf("Staged book %d of %d — how would you like to save it?", i+1, total),
			},
			StagedID:      staged.ID,
			IRCFilename:   staged.IRCFilename,
			Metadata:      staged.Metadata,
			Options:       staged.Options,
			ReplaceSpace:  staged.ReplaceSpace,
			CoverBase64:   staged.CoverBase64,
			CoverMime:     staged.CoverMime,
			StagedAt:      staged.StagedAt,
			QueuePosition: i + 1,
			TotalQueued:   total,
		})

		var choice RenameChoice
		select {
		case choice = <-c.renameConfirm:
		case <-time.After(30 * time.Minute):
			// Timed out — leave remaining books in staging, stop processing.
			return
		case <-c.ctx.Done():
			return
		}

		for {
			err := c.processStagedBookChoice(server, staged, choice)
			if err == nil {
				break // success — move to next staged book
			}
			if err == errQueueLater {
				break // deferred — move to next staged book
			}
			if err == errFileConflict {
				// FILE_CONFLICT already sent — wait for the user's next decision.
				select {
				case choice = <-c.renameConfirm:
				case <-time.After(30 * time.Minute):
					return
				case <-c.ctx.Done():
					return
				}
				continue
			}
			// Other error (move failure etc.) — already reported, move on.
			break
		}
	}

	server.broadcastStagedCount()
}

// handleStagedRenameConfirm processes a rename confirm for a specific staged book (by ID).
// This path is used when the frontend sends RENAME_CONFIRM with a non-empty stagedId.
func (c *Client) handleStagedRenameConfirm(req *RenameConfirmRequest, server *server) {
	staged, ok := server.stagedBooks.Get(req.StagedID)
	if !ok {
		safeSend(c, newErrorResponse("Staged book not found."))
		return
	}

	choice := RenameChoice{
		OptionID:        req.OptionID,
		CustomName:      req.CustomName,
		FileName:        req.FileName,
		RewriteMetadata: req.RewriteMetadata,
		Author:          req.Author,
		Title:           req.Title,
		Series:          req.Series,
		SeriesIndex:     req.SeriesIndex,
		Force:           req.Force,
	}

	if err := c.processStagedBookChoice(server, staged, choice); err == errQueueLater {
		return
	} else if err == errFileConflict {
		// FILE_CONFLICT response already sent — the frontend will re-show the
		// modal and the user will send a new RENAME_CONFIRM.
		return
	} else if err != nil {
		return
	}
	server.broadcastStagedCount()
}

// sendDownloadRequest queues a book download in the session's download queue.
func (c *Client) sendDownloadRequest(d *DownloadRequest, server *server) {
	sess := server.getSession(c.uuid)
	if sess == nil {
		return
	}

	title := d.Title
	if title == "" {
		title = d.Book
	}
	pending := len(sess.downloadQueue)
	if pending > 0 {
		server.logBuf.info(fmt.Sprintf("Queued: %s (position %d)", title, pending+1))
		c.send <- newStatusResponse(NOTIFY, fmt.Sprintf("Download queued (position %d).", pending+1))
	} else {
		if d.Title != "" && d.Author != "" {
			server.logBuf.info(fmt.Sprintf("📚 Download: %s by %s", d.Title, d.Author))
		} else {
			server.logBuf.info(fmt.Sprintf("📚 Download: %s", d.Book))
		}
		c.send <- newStatusResponse(NOTIFY, "Download request received.")
	}
	sess.downloadQueue <- downloadJob{book: d.Book, title: title, author: d.Author}
}

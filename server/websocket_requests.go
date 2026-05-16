package server

import (
	"encoding/json"
	"fmt"
	"os"
	"strings"
	"time"

	"github.com/evan-buss/openbooks/core"
	"github.com/evan-buss/openbooks/util"
)

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

// sendSearchRequest enqueues a search query in the session's search queue.
// The queue is drained by processSearchQueue with a cooldown between each request.
func (c *Client) sendSearchRequest(s *SearchRequest, server *server) {
	sess := server.getSession(c.uuid)
	if sess == nil {
		return
	}

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
		c.send <- newStatusResponse(WARNING, "Search queue is full. Please wait before searching again.")
	}
}

// sanitizePathComponent trims whitespace, replaces path separators with dashes,
// and optionally replaces spaces with the given character.
func sanitizePathComponent(s, replaceSpace string) string {
	s = strings.TrimSpace(s)
	s = strings.ReplaceAll(s, "/", "-")
	s = strings.ReplaceAll(s, "\\", "-")
	if replaceSpace != "" {
		s = strings.ReplaceAll(s, " ", replaceSpace)
	}
	return s
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
	}
	select {
	case c.renameConfirm <- choice:
	default:
		c.log.Println("handleRenameConfirm: no pending rename awaiting confirmation")
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

		if choice.OptionID == "queue_later" {
			// User deferred this one — move on to the next.
			safeSend(c, newStatusResponse(NOTIFY, "Book saved for later."))
			continue
		}

		// Move the staged book to its final path.
		finalPath := resolveFinalPath(server.config.DownloadDir, choice, staged.IRCFilename, staged.Metadata, staged.ReplaceSpace)
		if err := moveFile(staged.StagedPath, finalPath); err != nil {
			safeSend(c, newErrorResponse(fmt.Sprintf("Move failed: %v", err)))
			continue
		}

		if choice.RewriteMetadata {
			if err := RewriteEPUBMetadata(finalPath, choice.Title, choice.Author, choice.Series, choice.SeriesIndex); err != nil {
				c.log.Printf("RewriteEPUBMetadata: %v", err)
			}
		}

		if choice.Series != "" {
			server.seriesRegistry.AddIfNew(choice.Series)
		}

		server.stagedBooks.Remove(staged.ID)
		safeSend(c, newDownloadResponse(finalPath, server.config.DownloadDir))
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
	}

	if choice.OptionID == "queue_later" {
		safeSend(c, newStatusResponse(NOTIFY, "Book saved for later."))
		return
	}

	finalPath := resolveFinalPath(server.config.DownloadDir, choice, staged.IRCFilename, staged.Metadata, staged.ReplaceSpace)
	if err := moveFile(staged.StagedPath, finalPath); err != nil {
		safeSend(c, newErrorResponse(fmt.Sprintf("Move failed: %v", err)))
		return
	}

	if choice.RewriteMetadata {
		if err := RewriteEPUBMetadata(finalPath, choice.Title, choice.Author, choice.Series, choice.SeriesIndex); err != nil {
			c.log.Printf("RewriteEPUBMetadata: %v", err)
		}
	}

	if choice.Series != "" {
		server.seriesRegistry.AddIfNew(choice.Series)
	}

	server.stagedBooks.Remove(req.StagedID)
	safeSend(c, newDownloadResponse(finalPath, server.config.DownloadDir))
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

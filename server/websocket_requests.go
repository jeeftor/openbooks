package server

import (
	"encoding/json"
	"fmt"
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

	switch message.MessageType {
	case SEARCH:
		obj = new(SearchRequest)
	case DOWNLOAD:
		obj = new(DownloadRequest)
	case RENAME_CONFIRM:
		obj = new(RenameConfirmRequest)
	}

	err := json.Unmarshal(message.Payload, &obj)
	if err != nil {
		server.log.Printf("Invalid request payload. %s.\n", err.Error())
		c.send <- StatusResponse{
			MessageType:      STATUS,
			NotificationType: DANGER,
			Title:            "Unknown request payload.",
		}
	}

	switch message.MessageType {
	case CONNECT:
		c.startIrcConnection(server)
	case SEARCH:
		c.sendSearchRequest(obj.(*SearchRequest), server)
	case DOWNLOAD:
		c.sendDownloadRequest(obj.(*DownloadRequest), server)
	case RENAME_CONFIRM:
		c.handleRenameConfirm(obj.(*RenameConfirmRequest))
	default:
		server.log.Println("Unknown request type received.")
	}
}

// handle ConnectionRequests and either connect to the server or do nothing
func (c *Client) startIrcConnection(server *server) {
	// Protect against send on closed channel if this client is being replaced
	defer func() {
		if r := recover(); r != nil {
			c.log.Printf("Recovered from panic in startIrcConnection: %v", r)
		}
	}()

	err := core.Join(c.irc, server.config.Server, server.config.EnableTLS)
	if err != nil {
		c.log.Println(err)
		server.logBuf.error(fmt.Sprintf("IRC connect failed: %v", err))
		safeSend(c, newErrorResponse("Unable to connect to IRC server."))
		return
	}

	server.logBuf.info(fmt.Sprintf("🔌 IRC connected: %s", c.irc.Username))
	handler := server.NewIrcEventHandler(c)

	if server.config.Log {
		logger, _, err := util.CreateLogFile(c.irc.Username, server.config.DownloadDir)
		if err != nil {
			server.log.Println(err)
		}
		handler[core.Message] = func(text string) { logger.Println(text) }
	}

	go core.StartReader(c.ctx, c.irc, handler)
	go c.processDownloadQueue(server)

	safeSend(c, ConnectionResponse{
		StatusResponse: StatusResponse{
			MessageType:      CONNECT,
			NotificationType: SUCCESS,
			Title:            "Welcome, connection established.",
			Detail:           fmt.Sprintf("IRC username %s", c.irc.Username),
		},
		Name: c.irc.Username,
	})
}

// safeSend attempts to send on the client channel, recovering from panic if channel is closed
func safeSend(c *Client, msg interface{}) {
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

// handle SearchRequests and send the query to the book server
func (c *Client) sendSearchRequest(s *SearchRequest, server *server) {
	server.lastSearchMutex.Lock()
	defer server.lastSearchMutex.Unlock()

	nextAvailableSearch := server.lastSearch.Add(server.config.SearchTimeout)

	if time.Now().Before(nextAvailableSearch) {
		remainingSeconds := time.Until(nextAvailableSearch).Seconds()
		c.send <- newRateLimitResponse(remainingSeconds)

		return
	}

	c.log.Printf("Searching for: %q\n", s.Query)
	server.logBuf.info(fmt.Sprintf("🔍 Search: %q", s.Query))
	core.SearchBook(c.irc, server.config.SearchBot, s.Query)
	server.lastSearch = time.Now()

	c.send <- newStatusResponse(NOTIFY, "Search request sent.")
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

// handleRenameConfirm forwards the user's rename decision to the waiting bookResultHandler.
func (c *Client) handleRenameConfirm(req *RenameConfirmRequest) {
	choice := RenameChoice{
		OptionID:        req.OptionID,
		CustomName:      req.CustomName,
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

// handle DownloadRequests by sending the request to the book server
func (c *Client) sendDownloadRequest(d *DownloadRequest, server *server) {
	title := d.Title
	if title == "" {
		title = d.Book
	}
	pending := len(c.downloadQueue)
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
	c.downloadQueue <- downloadJob{book: d.Book, title: title, author: d.Author}
}

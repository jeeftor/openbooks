package server

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"strings"
	"time"

	"github.com/evan-buss/openbooks/core"
	"github.com/evan-buss/openbooks/irc"
	"github.com/google/uuid"

	"github.com/gorilla/websocket"
)

const (
	// Time allowed to write a message to the peer.
	writeWait = 10 * time.Second

	// Time allowed to read the next pong message from the peer.
	pongWait = 60 * time.Second

	// Send pings to peer with this period. Must be less than pongWait.
	pingPeriod = (pongWait * 9) / 10

	// Maximum message size allowed from peer.
	maxMessageSize = 4096
)

var upgrader = websocket.Upgrader{
	ReadBufferSize:  1024,
	WriteBufferSize: 1024,
}

// downloadJob holds a queued download request.
type downloadJob struct {
	book   string
	title  string
	author string
}

// Client is a middleman between the websocket connection and the hub.
type Client struct {
	// Unique ID for the client
	uuid uuid.UUID

	// The websocket connection.
	conn *websocket.Conn

	// Message to send to the client ws connection
	send chan interface{}

	// Individual IRC connection per connected client.
	irc *irc.Conn

	log *log.Logger

	// Context is used to signal when this client should close.
	ctx context.Context

	// downloadQueue serializes downloads so only one DCC transfer is active at a time.
	downloadQueue chan downloadJob

	// downloadDone is signaled by bookResultHandler when a download finishes.
	downloadDone chan struct{}

	// renameConfirm receives the user's rename decision from the WebSocket handler.
	renameConfirm chan RenameChoice
}

// processDownloadQueue drains downloadQueue one job at a time, sending each
// request to IRC and waiting for bookResultHandler to signal completion before
// proceeding to the next. This prevents flooding IRC with concurrent DCC requests.
func (c *Client) processDownloadQueue(server *server) {
	for {
		select {
		case job, ok := <-c.downloadQueue:
			if !ok {
				return
			}
			pending := len(c.downloadQueue)
			if pending > 0 {
				server.logBuf.info(fmt.Sprintf("📋 Queued: %s (%d pending)", job.title, pending))
			}
			// Extract bot name from the book string (first word after "!")
			botName := job.book
			if idx := strings.Index(job.book, " "); idx > 1 {
				botName = job.book[1:idx] // strip leading "!"
			}
			server.logBuf.info(fmt.Sprintf("📡 Requesting from %s — waiting for IRC bot to send file…", botName))
			c.send <- newDownloadWaitingResponse(botName, job.title)
			core.DownloadBook(c.irc, job.book)
			// Wait for bookResultHandler to signal completion or give up after 5 min.
			select {
			case <-c.downloadDone:
				// bookResultHandler already sent the clear after download finished.
			case <-time.After(5 * time.Minute):
				c.send <- newDownloadWaitingClear()
				server.logBuf.warn(fmt.Sprintf("⏱️  Timed out waiting for %s — bot may be offline or throttling. Skipping.", botName))
			case <-c.ctx.Done():
				c.send <- newDownloadWaitingClear()
				return
			}
		case <-c.ctx.Done():
			return
		}
	}
}

// readPump pumps messages from the websocket connection to the hub.
//
// The application runs readPump in a per-connection goroutine. The application
// ensures that there is at most one reader on a connection by executing all
// reads from this goroutine.
func (server *server) readPump(c *Client) {
	defer func() {
		server.logBuf.info(fmt.Sprintf("IRC disconnected: %s", c.irc.Username))
		c.irc.Disconnect()
		c.conn.Close()
		server.unregister <- c
	}()
	c.conn.SetReadLimit(maxMessageSize)
	c.conn.SetReadDeadline(time.Now().Add(pongWait))
	c.conn.SetPongHandler(func(string) error { c.conn.SetReadDeadline(time.Now().Add(pongWait)); return nil })
	for {
		select {
		case <-c.ctx.Done():
			return
		default:
			var request Request
			err := c.conn.ReadJSON(&request)

			if err != nil {
				c.log.Printf("Connection Closed: %v", err)
				return
			}

			// Log message type and payload for debugging
			payloadJSON, _ := json.Marshal(request.Payload)
			c.log.Printf("%s Message Received: %s\n", request.MessageType, string(payloadJSON))

			server.routeMessage(request, c)
		}
	}
}

// writePump pumps messages from the hub to the websocket connection.
//
// A goroutine running writePump is started for each connection. The
// application ensures that there is at most one writer to a connection by
// executing all writes from this goroutine.
func (server *server) writePump(c *Client) {
	ticker := time.NewTicker(pingPeriod)
	defer func() {
		ticker.Stop()
	}()

	for {
		select {
		case message, ok := <-c.send:
			c.conn.SetWriteDeadline(time.Now().Add(writeWait))
			if !ok {
				// The hub closed the channel.
				c.conn.WriteMessage(websocket.CloseMessage, []byte{})
				return
			}

			err := c.conn.WriteJSON(message)
			if err != nil {
				c.log.Printf("Error writing JSON to websocket: %s\n", err)
				return
			}
		case <-c.ctx.Done():
			return
		case <-ticker.C:
			c.conn.SetWriteDeadline(time.Now().Add(writeWait))
			if err := c.conn.WriteMessage(websocket.PingMessage, nil); err != nil {
				return
			}
		}
	}
}

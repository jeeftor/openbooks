package server

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"time"

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
	// Unique ID for the client (matches the browser cookie, used to look up the IRC session).
	uuid uuid.UUID

	// IRC username for this client's session — used for logging and stats.
	username string

	// The websocket connection.
	conn *websocket.Conn

	// Message to send to the client ws connection
	send chan interface{}

	log *log.Logger

	// Context is used to signal when this client should close.
	ctx context.Context

	// renameConfirm receives the user's rename decision from the WebSocket handler.
	renameConfirm chan RenameChoice
}


// readPump pumps messages from the websocket connection to the hub.
//
// The application runs readPump in a per-connection goroutine. The application
// ensures that there is at most one reader on a connection by executing all
// reads from this goroutine.
func (server *server) readPump(c *Client) {
	defer func() {
		server.logBuf.info(fmt.Sprintf("🔌 Browser disconnected: %s", c.username))
		// Detach this client from its session. The IRC connection and download queue
		// continue running so background downloads can complete.
		if sess := server.getSession(c.uuid); sess != nil {
			sess.detachClient(c)
		}
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

package irc

import (
	"crypto/tls"
	"log"
	"net"
	"sync/atomic"
)

// Conn represents an IRC connection to a server
type Conn struct {
	net.Conn
	channel      string
	Username     string
	realname     string
	disconnected atomic.Bool
}

// New creates a new IRC connection to the server using the supplied username and realname
func New(username, realname string) *Conn {
	irc := &Conn{
		channel:  "",
		Username: username,
		realname: realname,
	}

	return irc
}

// Connect connects to the given server at port 6667
func (i *Conn) Connect(address string, enableTLS bool) error {
	var conn net.Conn
	var err error
	if enableTLS {
		conn, err = tls.Dial("tcp", address, &tls.Config{InsecureSkipVerify: true})
	} else {
		conn, err = net.Dial("tcp", address)
	}

	if err != nil {
		return err
	}

	i.Conn = conn
	i.disconnected.Store(false)

	user := "USER " + i.Username + " 0 * :" + i.Username + "\r\n"
	nick := "NICK " + i.Username + "\r\n"

	i.write([]byte(user))
	i.write([]byte(nick))
	return nil
}

// write sends bytes to the IRC server and marks the connection dead on error.
func (i *Conn) write(data []byte) {
	if _, err := i.Conn.Write(data); err != nil {
		log.Printf("[IRC] write error: %v", err)
		i.disconnected.Store(true)
	}
}

// Disconnect closes connection to the IRC server
func (i *Conn) Disconnect() {
	i.disconnected.Store(true)
	if i.Conn == nil {
		return
	}
	i.Conn.Write([]byte("QUIT :Goodbye\r\n")) //nolint:errcheck — closing anyway
	i.Conn.Close()
}

// SendMessage sends the given message string to the connected IRC server
func (i *Conn) SendMessage(message string) {
	if !i.IsConnected() {
		return
	}
	i.write([]byte("PRIVMSG #" + i.channel + " :" + message + "\r\n"))
}

// SendNotice sends a notice message to the specified user
func (i *Conn) SendNotice(user string, message string) {
	if !i.IsConnected() {
		return
	}
	i.write([]byte("NOTICE " + user + " :" + message + "\r\n"))
}

// JoinChannel joins the channel given by channel string
func (i *Conn) JoinChannel(channel string) {
	if !i.IsConnected() {
		return
	}
	i.channel = channel
	i.write([]byte("JOIN #" + channel + "\r\n"))
}

// SetChannel stores the channel name without sending JOIN.
// Used by the reader to defer the actual JOIN until the server sends 001 (welcome),
// avoiding a "451 You have not registered" rejection when JOIN is sent too early.
func (i *Conn) SetChannel(channel string) {
	i.channel = channel
}

// Channel returns the configured channel name (without leading #).
func (i *Conn) Channel() string {
	return i.channel
}

// GetUsers sends a NAMES request to the IRC server
func (i *Conn) GetUsers(channel string) {
	if !i.IsConnected() {
		return
	}
	i.write([]byte("NAMES #" + channel + "\r\n"))
}

// Pong sends a Pong message to the server, often used after a PING request
func (i *Conn) Pong(server string) {
	if !i.IsConnected() {
		return
	}
	i.write([]byte("PONG " + server + "\r\n"))
}

// IsConnected returns true if the IRC connection is established and not known to be dead.
func (i *Conn) IsConnected() bool {
	return i.Conn != nil && !i.disconnected.Load()
}

// MarkDisconnected marks the connection as dead without closing it.
// Called by the reader after EOF so IsConnected() returns false immediately.
func (i *Conn) MarkDisconnected() {
	i.disconnected.Store(true)
}

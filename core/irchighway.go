package core

import (
	"fmt"
	"strings"

	"github.com/jeeftor/openbooks/irc"
)

// Specific irc.irchighway.net commands

// Join connects to the irc.irchighway.net server and registers the nickname.
// The actual channel JOIN is deferred to the reader, which sends it after
// receiving the 001 welcome message — sending JOIN before 001 results in
// "451 You have not registered" on some servers.
func Join(irc *irc.Conn, address string, enableTLS bool) error {
	err := irc.Connect(address, enableTLS)
	if err != nil {
		return err
	}
	// Store the channel name; the reader will send JOIN after 001 is received.
	irc.SetChannel("ebooks")
	return nil
}

// SearchBook sends a search query to the search bot
func SearchBook(irc *irc.Conn, searchBot string, query string) {
	searchBot = strings.TrimPrefix(searchBot, "@")
	irc.SendMessage(fmt.Sprintf("@%s %s", searchBot, query))
}

// DownloadBook sends the book string to the download bot
func DownloadBook(irc *irc.Conn, book string) {
	irc.SendMessage(book)
}

// Send a CTCP Version response
func SendVersionInfo(irc *irc.Conn, line string, version string) {
	// Line format is like ":messager PRIVMSG #channel: message"
	// we just want the messager without the colon
	sender := strings.Split(line, " ")[0][1:]
	// TODO: Figure out if there's an automated way to adjust this...
	irc.SendNotice(sender, fmt.Sprintf("\x01%s\x01", version))
}

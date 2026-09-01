package core

import (
	"bufio"
	"context"
	"log"
	"strings"

	"github.com/jeeftor/openbooks/irc"
)

type event int

const (
	noOp           = event(0)
	Message        = event(1)
	SearchResult   = event(2)
	BookResult     = event(3)
	NoResults      = event(4)
	BadServer      = event(5)
	SearchAccepted = event(6)
	MatchesFound   = event(7)
	ServerList     = event(8)
	Ping           = event(9)
	Version        = event(10)
	ChannelBanned  = event(11)
	ChannelFull    = event(12)
	InviteOnly     = event(13)
	BadChannelKey  = event(14)
	NickInUse      = event(15)
)

// Unique identifiers found in the message for various different events.
const (
	pingMessage            = "PING"
	sendMessage            = "DCC SEND"
	noticeMessage          = "NOTICE"
	noResults              = "Sorry"
	serverUnavailable      = "try another server"
	searchAccepted         = "has been accepted"
	searchResultIdentifier = "_results_for"
	numMatches             = "matches"
	beginUserList          = "353"
	endUserList            = "366"
	versionInquiry         = "\x01VERSION\x01"
)

type HandlerFunc func(text string)
type EventHandler map[event]HandlerFunc

func StartReader(ctx context.Context, irc *irc.Conn, handler EventHandler) {
	var users strings.Builder
	scanner := bufio.NewScanner(irc)

	for scanner.Scan() {
		select {
		case <-ctx.Done():
			return
		default:
			text := scanner.Text()
			if err := scanner.Err(); err != nil {
				log.Println(err)
			}

			// Send raw message if they want to recieve it (logging purposes)
			if invoke, ok := handler[Message]; ok {
				invoke(text)
			}

			// Log important IRC numerics for connection diagnostics.
			// These appear during handshake and channel join; none are handled
			// as events but they're invaluable when diagnosing search failures.
			switch {
			case strings.Contains(text, " 001 "): // RPL_WELCOME
				log.Printf("[IRC] connected: %s", text)
				// Now that the server has accepted our registration, join the channel.
				// Sending JOIN before 001 results in "451 You have not registered".
				if ch := irc.Channel(); ch != "" {
				irc.JoinChannel(ch)
				log.Printf("[IRC] joining #%s", ch)
				}
			case strings.Contains(text, " 332 "): // RPL_TOPIC — channel topic often mentions bot status
				log.Printf("[IRC] channel topic: %s", text)
			case strings.Contains(text, " 372 "): // RPL_MOTD
				log.Printf("[IRC] MOTD: %s", text)
			case strings.Contains(text, " 376 "): // RPL_ENDOFMOTD
				log.Printf("[IRC] MOTD end")
			case strings.Contains(text, " JOIN "): // channel join confirmation
				log.Printf("[IRC] JOIN: %s", text)
			case strings.Contains(text, " 433 "): // ERR_NICKNAMEINUSE
				log.Printf("[IRC] ERROR nick in use: %s", text)
			case strings.Contains(text, " 471 "): // ERR_CHANNELISFULL
				log.Printf("[IRC] ERROR channel full: %s", text)
			case strings.Contains(text, " 473 "): // ERR_INVITEONLYCHAN
				log.Printf("[IRC] ERROR invite only: %s", text)
			case strings.Contains(text, " 474 "): // ERR_BANNEDFROMCHAN
			log.Printf("[IRC] ERROR banned from channel: %s", text)
			case strings.Contains(text, " 475 "): // ERR_BADCHANNELKEY
				log.Printf("[IRC] ERROR bad channel key: %s", text)
			case strings.HasPrefix(text, "ERROR "): // server-level error / kill
				log.Printf("[IRC] ERROR: %s", text)
			}

			event := noOp
			if strings.Contains(text, sendMessage) {
				if strings.Contains(text, searchResultIdentifier) {
					event = SearchResult
					log.Printf("[IRC] <- DCC SEND: search results file")
				} else {
					event = BookResult
					log.Printf("[IRC] <- DCC SEND: book file")
				}
			} else if strings.Contains(text, noticeMessage) {
				if strings.Contains(text, noResults) {
					event = NoResults
					log.Printf("[IRC] <- NOTICE: no results")
				} else if strings.Contains(text, serverUnavailable) {
					event = BadServer
					log.Printf("[IRC] <- NOTICE: server unavailable")
				} else if strings.Contains(text, searchAccepted) {
					event = SearchAccepted
					log.Printf("[IRC] <- NOTICE: search accepted")
				} else if strings.Contains(text, numMatches) {
					start := strings.LastIndex(text, "returned") + 9
					end := strings.LastIndex(text, "matches") - 1
					text = text[start:end]
					event = MatchesFound
					log.Printf("[IRC] <- NOTICE: %s matches found", text)
				}
			} else if strings.Contains(text, beginUserList) {
				// RPL_NAMREPLY (353) can span multiple lines for large
				// channels. Separate accumulated lines with a space so a nick
				// at a line boundary doesn't merge into the next line's prefix.
				if users.Len() > 0 {
					users.WriteString(" ")
				}
				users.WriteString(text)
			} else if strings.Contains(text, endUserList) {
				event = ServerList
				text = users.String()
				users.Reset()
			} else if strings.Contains(text, pingMessage) {
				event = Ping
			} else if strings.Contains(text, versionInquiry) {
				event = Version
			} else if strings.Contains(text, " 474 ") {
				event = ChannelBanned
			} else if strings.Contains(text, " 471 ") {
				event = ChannelFull
			} else if strings.Contains(text, " 473 ") {
				event = InviteOnly
			} else if strings.Contains(text, " 475 ") {
				event = BadChannelKey
			} else if strings.Contains(text, " 433 ") {
				event = NickInUse
			}

			if invoke, ok := handler[event]; ok {
				go invoke(text)
			}
		}
	}
}

package mock

import (
	"bufio"
	"flag"
	"fmt"
	"log"
	"net"
	"os"
	"strings"
	"time"
)

type IrcServer struct {
	Port string
	log  *log.Logger
	ban  bool
}

func (irc *IrcServer) Start(ready chan<- struct{}) {
	irc.log = log.New(os.Stdout, "MOCK SERVER: ", 0)

	// --ban simulates a 474 ERR_BANNEDFROMCHAN response for testing the ban UI.
	ban := flag.Bool("ban", false, "Simulate channel ban (474) instead of normal join")
	flag.Parse()
	irc.ban = *ban

	server, err := net.Listen("tcp", irc.Port)
	if err != nil {
		panic(err)
	}
	irc.log.Println("Listening on " + irc.Port)
	if irc.ban {
		irc.log.Println("Ban mode ENABLED — will send 474 instead of channel join")
	}
	ready <- struct{}{}

	for {
		conn, err := server.Accept()
		if err != nil {
			panic(err)
		}
		go irc.handler(conn)
	}
}

func (irc *IrcServer) handler(conn net.Conn) {
	irc.log.Printf("Connection received from %s", conn.RemoteAddr().String())
	scanner := bufio.NewScanner(conn)

	irc.serverHandler(conn)

	irc.sendVersionRequest(conn)

	for scanner.Scan() {
		request := scanner.Text()
		if err := scanner.Err(); err != nil {
			irc.log.Println(err)
		}

		irc.log.Printf("Request Received: %s\n", request)

		if strings.Contains(request, "@search") {
			go irc.searchHandler(request, conn)
		}

		if strings.Contains(request, "!") {
			go irc.downloadHandler(request, conn)
		}
	}

	irc.log.Println("Connection closed.")
}

func (irc *IrcServer) sendVersionRequest(conn net.Conn) {
	irc.log.Println("Sending CTCP Version inquiry.")
	fmt.Fprintf(conn, ":mock_server PRIVMSG evan_28 :\x01VERSION\x01\r\n")
}

// serverHandler sends the IRC handshake. The real irchighway.net server sends:
//   001 RPL_WELCOME — registration accepted
//   372 RPL_MOTD    — message of the day lines
//   376 RPL_ENDOFMOTD
//   353 RPL_NAMREPLY — channel names list
//   366 RPL_ENDOFNAMES
//
// The reader sends JOIN only after receiving 001, so the mock must send it.
// In --ban mode, we send 001 (so registration succeeds) then 474 (banned from
// channel) instead of the names list, which triggers the ChannelBanned event.
func (irc *IrcServer) serverHandler(conn net.Conn) {
	// Registration accepted — this is what triggers the reader to send JOIN.
	fmt.Fprintf(conn, ":mock_server 001 evan_28 :Welcome to the mock IRC server\r\n")

	// Minimal MOTD so the handshake looks realistic.
	fmt.Fprintf(conn, ":mock_server 372 evan_28 :- Mock IRC server for OpenBooks development\r\n")
	fmt.Fprintf(conn, ":mock_server 376 evan_28 :End of MOTD\r\n")

	if irc.ban {
		// Simulate a channel ban — the reader will fire ChannelBanned.
		fmt.Fprintf(conn, ":mock_server 474 evan_28 #ebooks :Cannot join channel (You're banned)\r\n")
		return
	}

	// Normal join — send the channel names list.
	fmt.Fprintf(conn, ":mock_server 353 evan_28 = #ebooks :~SearchOok ~server1 ~server2 ~evan_irc\r\n")
	fmt.Fprintf(conn, ":mock_server 366 evan_28 #ebooks :End of names list\r\n")
}

func (irc *IrcServer) searchHandler(request string, conn net.Conn) {
	irc.log.Printf("Sending search results.")
	fmt.Fprint(conn, ":SearchOok!ook@only.ook NOTICE evan_28 :Search returned 27 matches\r\n")
	fmt.Fprint(conn, ":SearchOok!ook@only.ook PRIVMSG evan_28 :DCC SEND SearchOok_results_for__the_great_gatsby.txt.zip 2130706433 6668 1184\r\n")
}

func (irc *IrcServer) downloadHandler(request string, conn net.Conn) {
	irc.log.Println("Sending book file.")
	time.Sleep(time.Second * 4)
	fmt.Fprint(conn, ":SearchOok!ook@only.ook PRIVMSG evan_28 :DCC SEND great-gatsby.epub 2130706433 6669 358887\r\n")
}

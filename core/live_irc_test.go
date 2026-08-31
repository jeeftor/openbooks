//go:build liveirc

// Live IRC integration tests against irc.irchighway.net.
//
// These tests connect to the real IRC server and exercise the full search flow:
// connect → 001 welcome → JOIN #ebooks → send @SearchOok → receive NOTICE
// (search accepted) → receive NOTICE (N matches) → DCC SEND of results file.
//
// They are gated behind the "liveirc" build tag so they never run in normal
// CI. Run them manually with:
//
//	go test -tags liveirc -v -run TestLiveIRC ./core/ -timeout 120s
//
// Requirements:
//   - Network access to irc.irchighway.net:6697
//   - No more than 2 existing IRC connections from your IP (session limit)
//
// The test uses a unique nick per run to avoid collisions and cleans up with
// QUIT on exit. It does not download the DCC file — it only verifies that the
// search bot sends a DCC SEND offer, which proves the full round-trip works.

package core

import (
	"context"
	"fmt"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/jeeftor/openbooks/irc"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// liveTestBot is the search bot to query. SearchOok is consistently online
// and responds in under 5 seconds. The channel topic says "@Search or
// @Searchook" — both work, but SearchOok is the most reliable.
const liveTestBot = "SearchOok"

// liveTestServer is the IRC server to connect to.
const liveTestServer = "irc.irchighway.net:6697"

// liveTestChannel is the channel to join.
const liveTestChannel = "ebooks"

// liveTestQuery is a search term guaranteed to have results.
const liveTestQuery = "tolkien"

// liveNick generates a unique IRC nick for the test run.
func liveNick() string {
	return fmt.Sprintf("ob_test_%d", time.Now().UnixNano()%100000)
}

// TestLiveIRC_ConnectAndJoin verifies that we can connect to irc.irchighway.net,
// receive the 001 welcome, and join #ebooks without a 451 "not registered"
// error. This is the regression test for the JOIN timing race condition.
func TestLiveIRC_ConnectAndJoin(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	conn := irc.New(liveNick(), "openbooks-live-test")
	defer conn.Disconnect()

	// Join connects and sets the channel; the reader sends JOIN after 001.
	err := Join(conn, liveTestServer, true)
	require.NoError(t, err, "failed to connect to IRC server")

	// Start the reader with a handler that tracks events.
	var (
		mu          sync.Mutex
		gotWelcome  bool
		gotJoin     bool
		gotTopic    bool
		gotNamesEnd bool
		joinErr     string
		topicText   string
	)

	handler := EventHandler{
		Message: func(text string) {
			mu.Lock()
			defer mu.Unlock()

			switch {
			case strings.Contains(text, " 001 "):
				gotWelcome = true
			case strings.Contains(text, " 332 "):
				gotTopic = true
				topicText = text
			case strings.Contains(text, " 366 "):
				gotNamesEnd = true
			case strings.Contains(text, " JOIN ") && strings.Contains(text, "#ebooks"):
				gotJoin = true
			case strings.Contains(text, " 451 "):
				joinErr = text
			case strings.Contains(text, " 404 "):
				joinErr = text
			case strings.Contains(text, " 474 "):
				joinErr = text
			case strings.HasPrefix(text, "ERROR "):
				joinErr = text
			}
		},
		Ping: func(text string) {
			// Respond to PING to keep the connection alive.
			parts := strings.SplitN(text, " ", 2)
			if len(parts) == 2 {
				conn.Pong(parts[1])
			}
		},
	}

	go StartReader(ctx, conn, handler)

	// Wait for join + names end (indicates we're fully in the channel).
	deadline := time.Now().Add(20 * time.Second)
	for time.Now().Before(deadline) {
		mu.Lock()
		done := gotWelcome && gotJoin && gotNamesEnd
		errMsg := joinErr
		mu.Unlock()
		if done || errMsg != "" {
			break
		}
		time.Sleep(200 * time.Millisecond)
	}

	mu.Lock()
	defer mu.Unlock()

	require.True(t, gotWelcome, "did not receive 001 RPL_WELCOME — server may be down")
	require.Empty(t, joinErr, "IRC error during connect/join: %s", joinErr)
	require.True(t, gotJoin, "did not see our own JOIN confirmation for #ebooks")
	require.True(t, gotNamesEnd, "did not receive 366 End of NAMES — not fully joined")
	assert.True(t, gotTopic, "did not receive 332 channel topic")
	assert.Contains(t, topicText, "#ebooks", "topic should mention #ebooks")
}

// TestLiveIRC_SearchFlow verifies the complete search round-trip:
// send @SearchOok tolkien → receive "search accepted" NOTICE → receive
// "N matches found" NOTICE → receive DCC SEND offer for the results file.
func TestLiveIRC_SearchFlow(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), 60*time.Second)
	defer cancel()

	conn := irc.New(liveNick(), "openbooks-live-test")
	defer conn.Disconnect()

	err := Join(conn, liveTestServer, true)
	require.NoError(t, err, "failed to connect to IRC server")

	var (
		mu             sync.Mutex
		joined         bool
		gotAccepted    bool
		gotMatches     bool
		gotDCCSend     bool
		matchesText    string
		dccText        string
		connError      string
		searchSent     bool
	)

	handler := EventHandler{
		Message: func(text string) {
			mu.Lock()
			defer mu.Unlock()

			// Track connection errors
			switch {
			case strings.Contains(text, " 404 "):
				connError = text
			case strings.Contains(text, " 451 "):
				connError = text
			case strings.Contains(text, " 474 "):
				connError = text
			case strings.HasPrefix(text, "ERROR "):
				connError = text
			}

			// Track join
			if strings.Contains(text, " 366 ") {
				joined = true
			}

			// Track search responses
			if strings.Contains(text, "NOTICE") {
				if strings.Contains(text, "has been accepted") {
					gotAccepted = true
				}
				if strings.Contains(text, "matches") && strings.Contains(text, "returned") {
					gotMatches = true
					matchesText = text
				}
			}

			// Track DCC SEND of search results
			if strings.Contains(text, "DCC SEND") && strings.Contains(text, "results_for") {
				gotDCCSend = true
				dccText = text
			}
		},
		SearchAccepted: func(text string) {
			mu.Lock()
			gotAccepted = true
			mu.Unlock()
		},
		MatchesFound: func(text string) {
			mu.Lock()
			gotMatches = true
			matchesText = text
			mu.Unlock()
		},
		SearchResult: func(text string) {
			mu.Lock()
			gotDCCSend = true
			dccText = text
			mu.Unlock()
		},
		Ping: func(text string) {
			parts := strings.SplitN(text, " ", 2)
			if len(parts) == 2 {
				conn.Pong(parts[1])
			}
		},
	}

	go StartReader(ctx, conn, handler)

	// Wait for join, then send search.
	deadline := time.Now().Add(20 * time.Second)
	for time.Now().Before(deadline) {
		mu.Lock()
		isJoined := joined
		errMsg := connError
		mu.Unlock()
		if isJoined || errMsg != "" {
			break
		}
		time.Sleep(200 * time.Millisecond)
	}

	mu.Lock()
	require.True(t, joined, "did not join #ebooks in time")
	require.Empty(t, connError, "connection error before search: %s", connError)
	mu.Unlock()

	// Send the search query.
	SearchBook(conn, liveTestBot, liveTestQuery)
	mu.Lock()
	searchSent = true
	mu.Unlock()
	t.Logf("sent @%s %s", liveTestBot, liveTestQuery)

	// Wait for responses (accepted + matches + DCC).
	deadline = time.Now().Add(45 * time.Second)
	for time.Now().Before(deadline) {
		mu.Lock()
		done := gotAccepted && gotMatches && gotDCCSend
		errMsg := connError
		mu.Unlock()
		if done || errMsg != "" {
			break
		}
		time.Sleep(500 * time.Millisecond)
	}

	mu.Lock()
	defer mu.Unlock()

	require.True(t, searchSent, "search was not sent")
	require.Empty(t, connError, "connection error during search: %s", connError)

	assert.True(t, gotAccepted, "did not receive 'search accepted' NOTICE — bot may be offline or ignoring")
	assert.True(t, gotMatches, "did not receive 'N matches found' NOTICE")
	assert.True(t, gotDCCSend, "did not receive DCC SEND for results file")

	if gotMatches {
		t.Logf("matches response: %s", matchesText)
	}
	if gotDCCSend {
		t.Logf("DCC SEND response: %s", dccText)
	}
}

// TestLiveIRC_No404AfterJoin verifies that we can send PRIVMSG to #ebooks
// immediately after joining, without receiving a 404 "Cannot send to channel"
// error. This is the direct regression test for the JOIN timing bug.
func TestLiveIRC_No404AfterJoin(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	conn := irc.New(liveNick(), "openbooks-live-test")
	defer conn.Disconnect()

	err := Join(conn, liveTestServer, true)
	require.NoError(t, err, "failed to connect to IRC server")

	var (
		mu       sync.Mutex
		joined   bool
		got404   bool
		got451   bool
	)

	handler := EventHandler{
		Message: func(text string) {
			mu.Lock()
			defer mu.Unlock()

			if strings.Contains(text, " 366 ") {
				joined = true
			}
			if strings.Contains(text, " 404 ") {
				got404 = true
			}
			if strings.Contains(text, " 451 ") {
				got451 = true
			}
		},
		Ping: func(text string) {
			parts := strings.SplitN(text, " ", 2)
			if len(parts) == 2 {
				conn.Pong(parts[1])
			}
		},
	}

	go StartReader(ctx, conn, handler)

	// Wait for join.
	deadline := time.Now().Add(20 * time.Second)
	for time.Now().Before(deadline) {
		mu.Lock()
		isJoined := joined
		mu.Unlock()
		if isJoined {
			break
		}
		time.Sleep(200 * time.Millisecond)
	}

	mu.Lock()
	require.True(t, joined, "did not join #ebooks in time")
	mu.Unlock()

	// Send a harmless message to the channel. We use a non-command message
	// to avoid triggering search bots. A simple "hello" won't be processed
	// by any bot but will test whether PRIVMSG is accepted.
	conn.SendMessage("test connection")

	// Wait a moment to see if 404 comes back.
	time.Sleep(3 * time.Second)

	mu.Lock()
	defer mu.Unlock()

	assert.False(t, got451, "received 451 'You have not registered' — JOIN timing race still present")
	assert.False(t, got404, "received 404 'Cannot send to channel' — not properly joined")
}

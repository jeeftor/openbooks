package server

import (
	"testing"
)

// TestMessageTypeValues guards the Go↔TypeScript protocol contract.
// The TS MessageType enum uses numeric values that must match the Go iota exactly.
// If you reorder or insert constants here, update server/app/src/types/messages.ts too.
func TestMessageTypeValues(t *testing.T) {
	t.Parallel()

	cases := []struct {
		name string
		got  MessageType
		want MessageType
	}{
		{"STATUS", STATUS, 0},
		{"CONNECT", CONNECT, 1},
		{"SEARCH", SEARCH, 2},
		{"DOWNLOAD", DOWNLOAD, 3},
		{"RATELIMIT", RATELIMIT, 4},
		{"RENAME_PROMPT", RENAME_PROMPT, 5},
		{"RENAME_CONFIRM", RENAME_CONFIRM, 6},
		{"DOWNLOAD_WAITING", DOWNLOAD_WAITING, 7},
		{"DOWNLOAD_STARTED", DOWNLOAD_STARTED, 8},
		{"POST_PROCESS_STARTED", POST_PROCESS_STARTED, 9},
		{"STAGED_BOOKS_NOTIFY", STAGED_BOOKS_NOTIFY, 10},
		{"STAGED_BOOK_RESUME", STAGED_BOOK_RESUME, 11},
		{"STAGED_QUEUE_LATER", STAGED_QUEUE_LATER, 12},
		{"SERIES_AUTOCOMPLETE", SERIES_AUTOCOMPLETE, 13},
		{"PROCESS_STAGED_BOOKS", PROCESS_STAGED_BOOKS, 14},
	}

	for _, tc := range cases {
		if tc.got != tc.want {
			t.Errorf("MessageType %s = %d, want %d (TS enum out of sync)", tc.name, tc.got, tc.want)
		}
	}
}

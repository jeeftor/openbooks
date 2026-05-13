package server

import (
	"path/filepath"
	"testing"

	"github.com/evan-buss/openbooks/core"
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

// TestBuildRenameOptionsNonEPUB verifies non-EPUB files only get the "keep" option.
func TestBuildRenameOptionsNonEPUB(t *testing.T) {
	t.Parallel()

	opts := buildRenameOptions("book.mobi", &core.EPUBMetadata{Title: "Dune", Author: "Frank Herbert"}, "")
	if len(opts) != 1 || opts[0].ID != "keep" {
		t.Fatalf("non-EPUB: got %d options (IDs: %v), want only [keep]", len(opts), optionIDs(opts))
	}
}

// TestBuildRenameOptionsNoMetadata verifies no metadata → only "keep".
func TestBuildRenameOptionsNoMetadata(t *testing.T) {
	t.Parallel()

	opts := buildRenameOptions("book.epub", nil, "")
	if len(opts) != 1 || opts[0].ID != "keep" {
		t.Fatalf("nil metadata: got %d options, want only [keep]", len(opts))
	}
}

// TestBuildRenameOptionsTitleOnly verifies title-only metadata gives keep + title.
func TestBuildRenameOptionsTitleOnly(t *testing.T) {
	t.Parallel()

	opts := buildRenameOptions("irc-file.epub", &core.EPUBMetadata{Title: "Dune"}, "")
	ids := optionIDs(opts)
	wantIDs := []string{"keep", "title"}
	if !equalStringSlices(ids, wantIDs) {
		t.Fatalf("title-only metadata: got %v, want %v", ids, wantIDs)
	}
	if opts[1].Preview != "Dune.epub" {
		t.Fatalf("title preview = %q, want %q", opts[1].Preview, "Dune.epub")
	}
}

// TestBuildRenameOptionsAuthorAndTitle verifies author+title gives keep, title, flat, organized.
func TestBuildRenameOptionsAuthorAndTitle(t *testing.T) {
	t.Parallel()

	meta := &core.EPUBMetadata{Author: "Frank Herbert", Title: "Dune"}
	opts := buildRenameOptions("irc-file.epub", meta, "")
	ids := optionIDs(opts)
	wantIDs := []string{"keep", "title", "author-title-flat", "organized"}
	if !equalStringSlices(ids, wantIDs) {
		t.Fatalf("author+title: got %v, want %v", ids, wantIDs)
	}
	if opts[2].Preview != "Frank Herbert - Dune.epub" {
		t.Fatalf("flat preview = %q, want %q", opts[2].Preview, "Frank Herbert - Dune.epub")
	}
	if opts[3].Preview != "Frank Herbert/Dune/Dune.epub" {
		t.Fatalf("organized preview = %q, want %q", opts[3].Preview, "Frank Herbert/Dune/Dune.epub")
	}
	if !opts[3].IsOrganized {
		t.Fatal("organized option should have IsOrganized=true")
	}
}

// TestBuildRenameOptionsWithSeries verifies all five options are generated when series is set.
func TestBuildRenameOptionsWithSeries(t *testing.T) {
	t.Parallel()

	meta := &core.EPUBMetadata{Author: "Frank Herbert", Title: "Dune", Series: "Dune Chronicles"}
	opts := buildRenameOptions("irc-file.epub", meta, "")
	ids := optionIDs(opts)
	wantIDs := []string{"keep", "title", "author-title-flat", "organized", "series"}
	if !equalStringSlices(ids, wantIDs) {
		t.Fatalf("with series: got %v, want %v", ids, wantIDs)
	}
	wantPreview := "Frank Herbert/Dune Chronicles/Dune/Dune.epub"
	if opts[4].Preview != wantPreview {
		t.Fatalf("series preview = %q, want %q", opts[4].Preview, wantPreview)
	}
}

// TestBuildRenameOptionsReplaceSpace verifies space replacement is applied.
func TestBuildRenameOptionsReplaceSpace(t *testing.T) {
	t.Parallel()

	meta := &core.EPUBMetadata{Author: "Frank Herbert", Title: "Dune Messiah"}
	opts := buildRenameOptions("irc-file.epub", meta, ".")
	// title option: spaces in title replaced with "."
	if opts[1].Preview != "Dune.Messiah.epub" {
		t.Fatalf("replaceSpace title preview = %q, want %q", opts[1].Preview, "Dune.Messiah.epub")
	}
}

// TestResolveFinalPathKeep verifies "keep" returns the IRC filename in downloadDir.
func TestResolveFinalPathKeep(t *testing.T) {
	t.Parallel()

	got := resolveFinalPath("books", RenameChoice{OptionID: "keep"}, "some-irc-file.epub", nil, "")
	want := filepath.Join("books", "some-irc-file.epub")
	if got != want {
		t.Fatalf("keep: got %q, want %q", got, want)
	}
}

// TestResolveFinalPathFlat verifies the author-title-flat option.
func TestResolveFinalPathFlat(t *testing.T) {
	t.Parallel()

	meta := &core.EPUBMetadata{Author: "Frank Herbert", Title: "Dune"}
	got := resolveFinalPath("books", RenameChoice{
		OptionID: "author-title-flat",
		Author:   "Frank Herbert",
		Title:    "Dune",
	}, "irc.epub", meta, "")
	want := filepath.Join("books", "Frank Herbert - Dune.epub")
	if got != want {
		t.Fatalf("flat: got %q, want %q", got, want)
	}
}

// TestResolveFinalPathCustom verifies the custom filename option.
func TestResolveFinalPathCustom(t *testing.T) {
	t.Parallel()

	got := resolveFinalPath("books", RenameChoice{
		OptionID:   "custom",
		CustomName: "my-special-book.epub",
	}, "irc.epub", nil, "")
	want := filepath.Join("books", "my-special-book.epub")
	if got != want {
		t.Fatalf("custom: got %q, want %q", got, want)
	}
}

// TestResolveFinalPathCustomEmptyFallsBack verifies empty custom name falls back to IRC filename.
func TestResolveFinalPathCustomEmptyFallsBack(t *testing.T) {
	t.Parallel()

	got := resolveFinalPath("books", RenameChoice{OptionID: "custom", CustomName: "  "}, "irc.epub", nil, "")
	want := filepath.Join("books", "irc.epub")
	if got != want {
		t.Fatalf("custom empty fallback: got %q, want %q", got, want)
	}
}

// TestResolveFinalPathSeriesNoSeriesField falls back to organized when series field is empty.
func TestResolveFinalPathSeriesNoSeriesField(t *testing.T) {
	t.Parallel()

	got := resolveFinalPath("books", RenameChoice{
		OptionID: "series",
		Author:   "Frank Herbert",
		Title:    "Dune",
		Series:   "", // no series
	}, "irc.epub", nil, "")
	want := filepath.Join("books", "Frank Herbert", "Dune", "Dune.epub")
	if got != want {
		t.Fatalf("series-no-series: got %q, want %q", got, want)
	}
}

// helpers

func optionIDs(opts []RenameOption) []string {
	ids := make([]string, len(opts))
	for i, o := range opts {
		ids[i] = o.ID
	}
	return ids
}

func equalStringSlices(a, b []string) bool {
	if len(a) != len(b) {
		return false
	}
	for i := range a {
		if a[i] != b[i] {
			return false
		}
	}
	return true
}

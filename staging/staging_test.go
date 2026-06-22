package staging

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/evan-buss/openbooks/core"
)

func TestOriginalCopyPath(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name string
		path string
		want string
	}{
		{
			name: "epub",
			path: filepath.Join("books", "Title.epub"),
			want: filepath.Join("books", "Title.orig.epub"),
		},
		{
			name: "multiple dots",
			path: filepath.Join("books", "A.Title.v2.epub"),
			want: filepath.Join("books", "A.Title.v2.orig.epub"),
		},
		{
			name: "no extension",
			path: filepath.Join("books", "Title"),
			want: filepath.Join("books", "Title.orig"),
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			if got := OriginalCopyPath(tt.path); got != tt.want {
				t.Fatalf("OriginalCopyPath(%q) = %q, want %q", tt.path, got, tt.want)
			}
		})
	}
}

func TestCopyFileCreatesParentAndCopiesBytes(t *testing.T) {
	t.Parallel()

	dir := t.TempDir()
	src := filepath.Join(dir, "src.epub")
	dst := filepath.Join(dir, "nested", "src.orig.epub")
	want := []byte("raw epub bytes")

	if err := os.WriteFile(src, want, 0644); err != nil {
		t.Fatal(err)
	}

	if err := CopyFile(src, dst); err != nil {
		t.Fatalf("CopyFile() error = %v", err)
	}

	got, err := os.ReadFile(dst)
	if err != nil {
		t.Fatal(err)
	}
	if string(got) != string(want) {
		t.Fatalf("copied bytes = %q, want %q", got, want)
	}
}

func TestResolveFinalPathUsesEditedFileName(t *testing.T) {
	t.Parallel()

	meta := &core.EPUBMetadata{
		Author: "F Scott Fitzgerald",
		Title:  "The Great Gatsby",
		Series: "Classics",
	}

	tests := []struct {
		name   string
		choice Choice
		want   string
	}{
		{
			name: "series organization uses custom file name",
			choice: Choice{
				OptionID: "series",
				Author:   "F Scott Fitzgerald",
				Title:    "The Great Gatsby",
				Series:   "Classics",
				FileName: "Gatsby - cleaned.epub",
			},
			want: filepath.Join("books", "F Scott Fitzgerald", "Classics", "The Great Gatsby", "Gatsby - cleaned.epub"),
		},
		{
			name: "series organization appends extension",
			choice: Choice{
				OptionID: "series",
				Author:   "F Scott Fitzgerald",
				Title:    "The Great Gatsby",
				Series:   "Classics",
				FileName: "Gatsby - cleaned",
			},
			want: filepath.Join("books", "F Scott Fitzgerald", "Classics", "The Great Gatsby", "Gatsby - cleaned.epub"),
		},
		{
			name: "organized falls back to title file name",
			choice: Choice{
				OptionID: "organized",
				Author:   "F Scott Fitzgerald",
				Title:    "The Great Gatsby",
			},
			want: filepath.Join("books", "F Scott Fitzgerald", "The Great Gatsby", "The Great Gatsby.epub"),
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			got := ResolveFinalPath("books", tt.choice, "irc-file.epub", meta, "")
			if got != tt.want {
				t.Fatalf("ResolveFinalPath() = %q, want %q", got, tt.want)
			}
		})
	}
}

// TestBuildRenameOptionsNonEPUB verifies non-EPUB files only get the "keep" option.
func TestBuildRenameOptionsNonEPUB(t *testing.T) {
	t.Parallel()

	opts := BuildOptions("book.mobi", &core.EPUBMetadata{Title: "Dune", Author: "Frank Herbert"}, "")
	if len(opts) != 1 || opts[0].ID != "keep" {
		t.Fatalf("non-EPUB: got %d options (IDs: %v), want only [keep]", len(opts), optionIDs(opts))
	}
}

// TestBuildRenameOptionsNoMetadata verifies no metadata → only "keep".
func TestBuildRenameOptionsNoMetadata(t *testing.T) {
	t.Parallel()

	opts := BuildOptions("book.epub", nil, "")
	if len(opts) != 1 || opts[0].ID != "keep" {
		t.Fatalf("nil metadata: got %d options, want only [keep]", len(opts))
	}
}

// TestBuildRenameOptionsTitleOnly verifies title-only metadata gives keep + title.
func TestBuildRenameOptionsTitleOnly(t *testing.T) {
	t.Parallel()

	opts := BuildOptions("irc-file.epub", &core.EPUBMetadata{Title: "Dune"}, "")
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
	opts := BuildOptions("irc-file.epub", meta, "")
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
	opts := BuildOptions("irc-file.epub", meta, "")
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
	opts := BuildOptions("irc-file.epub", meta, ".")
	// title option: spaces in title replaced with "."
	if opts[1].Preview != "Dune.Messiah.epub" {
		t.Fatalf("replaceSpace title preview = %q, want %q", opts[1].Preview, "Dune.Messiah.epub")
	}
}

// TestResolveFinalPathKeep verifies "keep" returns the IRC filename in downloadDir.
func TestResolveFinalPathKeep(t *testing.T) {
	t.Parallel()

	got := ResolveFinalPath("books", Choice{OptionID: "keep"}, "some-irc-file.epub", nil, "")
	want := filepath.Join("books", "some-irc-file.epub")
	if got != want {
		t.Fatalf("keep: got %q, want %q", got, want)
	}
}

// TestResolveFinalPathFlat verifies the author-title-flat option.
func TestResolveFinalPathFlat(t *testing.T) {
	t.Parallel()

	meta := &core.EPUBMetadata{Author: "Frank Herbert", Title: "Dune"}
	got := ResolveFinalPath("books", Choice{
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

	got := ResolveFinalPath("books", Choice{
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

	got := ResolveFinalPath("books", Choice{OptionID: "custom", CustomName: "  "}, "irc.epub", nil, "")
	want := filepath.Join("books", "irc.epub")
	if got != want {
		t.Fatalf("custom empty fallback: got %q, want %q", got, want)
	}
}

// TestResolveFinalPathSeriesNoSeriesField falls back to organized when series field is empty.
func TestResolveFinalPathSeriesNoSeriesField(t *testing.T) {
	t.Parallel()

	got := ResolveFinalPath("books", Choice{
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

func optionIDs(opts []Option) []string {
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

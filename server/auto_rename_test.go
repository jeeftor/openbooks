package server

import (
	"testing"

	"github.com/jeeftor/openbooks/core"
	"github.com/jeeftor/openbooks/staging"
)

// TestAutoRenameChoiceValidOption verifies that autoRenameChoice selects the
// requested option and populates metadata fields from the extracted EPUBMetadata.
func TestAutoRenameChoiceValidOption(t *testing.T) {
	t.Parallel()

	meta := &core.EPUBMetadata{
		Author:      "Frank Herbert",
		Title:       "Dune",
		Series:      "Dune Chronicles",
		SeriesIndex: "1",
	}
	options := staging.BuildOptions("irc-file.epub", meta, "")

	choice := autoRenameChoice("author-title-flat", options, meta)
	if choice.OptionID != "author-title-flat" {
		t.Fatalf("OptionID = %q, want %q", choice.OptionID, "author-title-flat")
	}
	if choice.Author != "Frank Herbert" {
		t.Errorf("Author = %q, want %q", choice.Author, "Frank Herbert")
	}
	if choice.Title != "Dune" {
		t.Errorf("Title = %q, want %q", choice.Title, "Dune")
	}
	if choice.Series != "Dune Chronicles" {
		t.Errorf("Series = %q, want %q", choice.Series, "Dune Chronicles")
	}
	if choice.SeriesIndex != "1" {
		t.Errorf("SeriesIndex = %q, want %q", choice.SeriesIndex, "1")
	}
}

// TestAutoRenameChoiceFallbackToKeep verifies that when the requested option
// isn't available (e.g. no metadata), autoRenameChoice falls back to "keep".
func TestAutoRenameChoiceFallbackToKeep(t *testing.T) {
	t.Parallel()

	// No metadata → only "keep" option is available.
	options := staging.BuildOptions("book.epub", nil, "")
	choice := autoRenameChoice("author-title-flat", options, nil)
	if choice.OptionID != "keep" {
		t.Fatalf("OptionID = %q, want %q (fallback)", choice.OptionID, "keep")
	}
}

// TestAutoRenameChoiceNilMeta verifies that a nil metadata doesn't panic and
// produces a valid choice with empty metadata fields.
func TestAutoRenameChoiceNilMeta(t *testing.T) {
	t.Parallel()

	options := []staging.Option{
		{ID: "keep", Label: "Keep IRC filename", Preview: "file.epub"},
		{ID: "title", Label: "Title only", Preview: "Title.epub"},
	}
	choice := autoRenameChoice("title", options, nil)
	if choice.OptionID != "title" {
		t.Fatalf("OptionID = %q, want %q", choice.OptionID, "title")
	}
	if choice.Author != "" {
		t.Errorf("Author = %q, want empty", choice.Author)
	}
	if choice.Title != "" {
		t.Errorf("Title = %q, want empty", choice.Title)
	}
}

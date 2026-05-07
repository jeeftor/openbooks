package server

import (
	"github.com/evan-buss/openbooks/core"
	"os"
	"path/filepath"
	"testing"
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

			if got := originalCopyPath(tt.path); got != tt.want {
				t.Fatalf("originalCopyPath(%q) = %q, want %q", tt.path, got, tt.want)
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

	if err := copyFile(src, dst); err != nil {
		t.Fatalf("copyFile() error = %v", err)
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
		choice RenameChoice
		want   string
	}{
		{
			name: "series organization uses custom file name",
			choice: RenameChoice{
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
			choice: RenameChoice{
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
			choice: RenameChoice{
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

			got := resolveFinalPath("books", tt.choice, "irc-file.epub", meta, "")
			if got != tt.want {
				t.Fatalf("resolveFinalPath() = %q, want %q", got, tt.want)
			}
		})
	}
}

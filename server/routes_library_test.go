package server

import (
	"archive/zip"
	"os"
	"path/filepath"
	"testing"
	"time"
)

func writeLibraryTestEPUB(t *testing.T, filePath string) {
	t.Helper()
	file, err := os.Create(filePath)
	if err != nil {
		t.Fatal(err)
	}
	writer := zip.NewWriter(file)
	entries := map[string]string{
		"META-INF/container.xml": `<?xml version="1.0"?><container><rootfiles><rootfile full-path="OEBPS/content.opf"/></rootfiles></container>`,
		"OEBPS/content.opf":      `<?xml version="1.0"?><package><metadata><title>The Metadata Title</title><creator role="aut">Metadata Author</creator><meta name="calibre:series" content="Metadata Series"/><meta name="calibre:series_index" content="3"/></metadata><manifest></manifest></package>`,
	}
	for name, content := range entries {
		entry, err := writer.Create(name)
		if err != nil {
			t.Fatal(err)
		}
		if _, err := entry.Write([]byte(content)); err != nil {
			t.Fatal(err)
		}
	}
	if err := writer.Close(); err != nil {
		t.Fatal(err)
	}
	if err := file.Close(); err != nil {
		t.Fatal(err)
	}
}

func TestLibraryFileFromPathUsesEPUBMetadata(t *testing.T) {
	dir := t.TempDir()
	filePath := filepath.Join(dir, "book.epub")
	writeLibraryTestEPUB(t, filePath)
	modified := time.Date(2026, time.September, 1, 12, 0, 0, 0, time.UTC)

	book := libraryFileFromPath(filePath, filepath.Join("Fallback Author", "Fallback Title", "book.epub"), modified)

	if book.Author != "Metadata Author" || book.Title != "The Metadata Title" {
		t.Fatalf("expected EPUB metadata, got author=%q title=%q", book.Author, book.Title)
	}
	if book.Series != "Metadata Series" || book.SeriesIndex != "3" {
		t.Fatalf("expected series metadata, got series=%q index=%q", book.Series, book.SeriesIndex)
	}
	if book.Format != "epub" || book.Path != "Fallback Author/Fallback Title/book.epub" {
		t.Fatalf("unexpected library file details: %#v", book)
	}
}

func TestLibraryFileFromPathFallsBackToOrganizedPath(t *testing.T) {
	book := libraryFileFromPath(
		filepath.Join("library", "Writer", "Novel", "novel.mobi"),
		filepath.Join("Writer", "Novel", "novel.mobi"),
		time.Time{},
	)

	if book.Author != "Writer" || book.Title != "Novel" || book.Format != "mobi" {
		t.Fatalf("unexpected fallback details: %#v", book)
	}
}

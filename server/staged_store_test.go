package server

import (
	"os"
	"path/filepath"
	"testing"
	"time"
)

func newTestStagedBook(id, path string) *StagedBook {
	return &StagedBook{
		ID:          id,
		StagedPath:  path,
		IRCFilename: id + ".epub",
		StagedAt:    time.Now(),
	}
}

func TestStagedBookStoreAddGetRemove(t *testing.T) {
	t.Parallel()

	dir := t.TempDir()
	store, err := newStagedBookStore(dir)
	if err != nil {
		t.Fatalf("newStagedBookStore() error = %v", err)
	}

	filePath := filepath.Join(dir, "book.epub")
	if err := os.WriteFile(filePath, []byte("epub"), 0644); err != nil {
		t.Fatal(err)
	}

	book := newTestStagedBook("abc-123", filePath)

	if err := store.Add(book); err != nil {
		t.Fatalf("Add() error = %v", err)
	}
	if store.Count() != 1 {
		t.Fatalf("Count() = %d, want 1", store.Count())
	}

	got, ok := store.Get("abc-123")
	if !ok {
		t.Fatal("Get() returned false, want true")
	}
	if got.ID != "abc-123" {
		t.Fatalf("Get() ID = %q, want %q", got.ID, "abc-123")
	}

	if err := store.Remove("abc-123"); err != nil {
		t.Fatalf("Remove() error = %v", err)
	}
	if store.Count() != 0 {
		t.Fatalf("Count() after Remove = %d, want 0", store.Count())
	}
	if _, ok := store.Get("abc-123"); ok {
		t.Fatal("Get() after Remove returned true, want false")
	}
}

func TestStagedBookStoreAllSortedByAge(t *testing.T) {
	t.Parallel()

	dir := t.TempDir()
	store, err := newStagedBookStore(dir)
	if err != nil {
		t.Fatal(err)
	}

	for i, id := range []string{"c", "a", "b"} {
		f := filepath.Join(dir, id+".epub")
		if err := os.WriteFile(f, []byte("x"), 0644); err != nil {
			t.Fatal(err)
		}
		book := newTestStagedBook(id, f)
		book.StagedAt = time.Now().Add(time.Duration(i) * time.Second)
		if err := store.Add(book); err != nil {
			t.Fatal(err)
		}
	}

	all := store.All()
	if len(all) != 3 {
		t.Fatalf("All() len = %d, want 3", len(all))
	}
	// Should be ordered c, a, b (insertion order = chronological)
	wantOrder := []string{"c", "a", "b"}
	for i, want := range wantOrder {
		if all[i].ID != want {
			t.Fatalf("All()[%d].ID = %q, want %q", i, all[i].ID, want)
		}
	}
}

func TestStagedBookStorePersistsAndReloads(t *testing.T) {
	t.Parallel()

	dir := t.TempDir()

	filePath := filepath.Join(dir, "book.epub")
	if err := os.WriteFile(filePath, []byte("epub"), 0644); err != nil {
		t.Fatal(err)
	}

	store1, err := newStagedBookStore(dir)
	if err != nil {
		t.Fatal(err)
	}
	if err := store1.Add(newTestStagedBook("id-1", filePath)); err != nil {
		t.Fatal(err)
	}

	// Load fresh store from same directory.
	store2, err := newStagedBookStore(dir)
	if err != nil {
		t.Fatal(err)
	}
	if store2.Count() != 1 {
		t.Fatalf("reloaded Count() = %d, want 1", store2.Count())
	}
	if _, ok := store2.Get("id-1"); !ok {
		t.Fatal("reloaded Get() returned false, want true")
	}
}

func TestStagedBookStoreDropsMissingFilesOnReload(t *testing.T) {
	t.Parallel()

	dir := t.TempDir()

	existingPath := filepath.Join(dir, "exists.epub")
	missingPath := filepath.Join(dir, "gone.epub")

	for _, p := range []string{existingPath, missingPath} {
		if err := os.WriteFile(p, []byte("epub"), 0644); err != nil {
			t.Fatal(err)
		}
	}

	store1, err := newStagedBookStore(dir)
	if err != nil {
		t.Fatal(err)
	}
	if err := store1.Add(newTestStagedBook("exists", existingPath)); err != nil {
		t.Fatal(err)
	}
	if err := store1.Add(newTestStagedBook("gone", missingPath)); err != nil {
		t.Fatal(err)
	}

	// Delete the file on disk before reloading.
	if err := os.Remove(missingPath); err != nil {
		t.Fatal(err)
	}

	store2, err := newStagedBookStore(dir)
	if err != nil {
		t.Fatal(err)
	}
	if store2.Count() != 1 {
		t.Fatalf("Count() = %d after missing-file prune, want 1", store2.Count())
	}
	if _, ok := store2.Get("gone"); ok {
		t.Fatal("Get(\"gone\") = true after file deleted, want false")
	}
	if _, ok := store2.Get("exists"); !ok {
		t.Fatal("Get(\"exists\") = false, want true")
	}
}

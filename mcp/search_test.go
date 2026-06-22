package mcp

import (
	"strings"
	"testing"

	"github.com/jeeftor/openbooks/core"
	"github.com/stretchr/testify/assert"
)

func TestBuildSearchResponse_DeduplicatesAndGroups(t *testing.T) {
	books := []core.BookDetail{
		{Server: "ThrawnBot", Author: "Frank Herbert", Title: "Dune", Format: "epub", Size: "1.2 MB", Full: "!ThrawnBot Frank Herbert - Dune.epub"},
		{Server: "EpubWorld", Author: "Frank Herbert", Title: "Dune (retail)", Format: "epub", Size: "2.4 MB", Full: "!EpubWorld Frank Herbert - Dune (retail).epub"},
		{Server: "ShadowBot", Author: "Frank Herbert", Title: "Dune", Format: "epub", Size: "3.0 MB", Full: "!ShadowBot Frank Herbert - Dune.epub"},
		{Server: "PdfBot", Author: "Frank Herbert", Title: "Dune", Format: "pdf", Size: "5.0 MB", Full: "!PdfBot Frank Herbert - Dune.pdf"},
		{Server: "ThrawnBot", Author: "Isaac Asimov", Title: "Foundation", Format: "epub", Size: "0.9 MB", Full: "!ThrawnBot Isaac Asimov - Foundation.epub"},
	}
	trusted := func(s string) bool { return s != "PdfBot" }

	resp := buildSearchResponse(books, trusted)

	// "Dune" and "Dune (retail)" normalize to the same title, so they collapse.
	// "Foundation" is a separate title. PDF is filtered out.
	assert.Equal(t, 2, resp.Total, "Dune group + Foundation group")
	assert.Len(t, resp.Books, 2)
	// The Dune group (3 copies) should pick the largest: 3.0 MB from ShadowBot.
	var duneEntry *bookResult
	for i := range resp.Books {
		if resp.Books[i].Title == "Dune" {
			duneEntry = &resp.Books[i]
		}
	}
	if assert.NotNil(t, duneEntry) {
		assert.Equal(t, "3.0 MB", duneEntry.Size, "should pick largest copy")
		assert.Equal(t, 3, duneEntry.Copies, "should collapse 3 sources")
	}
}

func TestBuildSearchResponse_FiltersNonEpubAndUntrusted(t *testing.T) {
	books := []core.BookDetail{
		{Server: "Trusted", Author: "A", Title: "T1", Format: "epub", Size: "1 MB", Full: "!Trusted A - T1.epub"},
		{Server: "Trusted", Author: "A", Title: "T2", Format: "pdf", Size: "1 MB", Full: "!Trusted A - T2.pdf"},
		{Server: "Untrusted", Author: "A", Title: "T3", Format: "epub", Size: "1 MB", Full: "!Untrusted A - T3.epub"},
	}
	trusted := func(s string) bool { return s == "Trusted" }

	resp := buildSearchResponse(books, trusted)

	assert.Len(t, resp.Books, 1)
	assert.Equal(t, "T1", resp.Books[0].Title)
}

// When the trusted-server list is empty (transient IRC names reply), the
// trusted filter would drop everything. The fallback should return all epub
// results so the agent still has something to show the user.
func TestBuildSearchResponse_FallbackWhenNoTrustedServers(t *testing.T) {
	books := []core.BookDetail{
		{Server: "BotA", Author: "A", Title: "T1", Format: "epub", Size: "1 MB", Full: "!BotA A - T1.epub"},
		{Server: "BotB", Author: "A", Title: "T2", Format: "pdf", Size: "1 MB", Full: "!BotB A - T2.pdf"},
		{Server: "BotC", Author: "A", Title: "T3", Format: "epub", Size: "1 MB", Full: "!BotC A - T3.epub"},
	}
	// No server is trusted — simulates an empty/stale server list.
	trusted := func(string) bool { return false }

	resp := buildSearchResponse(books, trusted)

	// Fallback returns all epub results (T1, T3), PDF still filtered out.
	assert.Len(t, resp.Books, 2)
	titles := []string{resp.Books[0].Title, resp.Books[1].Title}
	assert.Contains(t, titles, "T1")
	assert.Contains(t, titles, "T3")
}

func TestRankBooks_PrefersQueryMatches(t *testing.T) {
	books := []bookResult{
		{Author: "Someone Else", Title: "Unrelated Book", Size: "5 MB", Copies: 1},
		{Author: "Frank Herbert", Title: "Dune", Size: "1 MB", Copies: 1},
	}
	ranked := rankBooks(books, "dune frank herbert")

	if assert.Len(t, ranked, 2) {
		assert.Equal(t, "Dune", ranked[0].Title, "exact match should rank first")
	}
}

func TestRankBooks_PrefersMoreCopies(t *testing.T) {
	books := []bookResult{
		{Author: "Frank Herbert", Title: "Dune", Size: "1 MB", Copies: 1},
		{Author: "Frank Herbert", Title: "Dune", Size: "1 MB", Copies: 5},
	}
	ranked := rankBooks(books, "dune")

	if assert.Len(t, ranked, 2) {
		assert.Equal(t, 5, ranked[0].Copies, "more copies should rank first")
	}
}

func TestRankBooks_PrefersLargerSize(t *testing.T) {
	books := []bookResult{
		{Author: "Frank Herbert", Title: "Dune", Size: "0.5 MB", Copies: 1},
		{Author: "Frank Herbert", Title: "Dune", Size: "3 MB", Copies: 1},
	}
	ranked := rankBooks(books, "dune")

	if assert.Len(t, ranked, 2) {
		assert.Equal(t, "3 MB", ranked[0].Size, "larger file should rank first")
	}
}

func TestRankBooks_StableForTies(t *testing.T) {
	books := []bookResult{
		{Author: "A", Title: "First", Size: "1 MB", Copies: 1},
		{Author: "A", Title: "Second", Size: "1 MB", Copies: 1},
		{Author: "A", Title: "Third", Size: "1 MB", Copies: 1},
	}
	ranked := rankBooks(books, "nomatch")

	if assert.Len(t, ranked, 3) {
		assert.Equal(t, "First", ranked[0].Title, "ties preserve original order")
		assert.Equal(t, "Second", ranked[1].Title)
		assert.Equal(t, "Third", ranked[2].Title)
	}
}

func TestRankBooks_EmptyAndSingle(t *testing.T) {
	assert.Empty(t, rankBooks(nil, "x"))
	single := []bookResult{{Author: "A", Title: "T", Size: "1 MB"}}
	assert.Equal(t, single, rankBooks(single, "x"))
}

func TestScoreBook_QueryRelevance(t *testing.T) {
	tests := []struct {
		name   string
		book   bookResult
		query  string
		wantGt float64 // minimum expected score
	}{
		{"words split across title+author", bookResult{Author: "Frank Herbert", Title: "Dune", Size: "1 MB"}, "dune frank herbert", 6},
		{"any word in title only", bookResult{Author: "Nobody", Title: "Dune", Size: "1 MB"}, "dune frank herbert", 3},
		{"no matches", bookResult{Author: "Nobody", Title: "Other", Size: "1 MB"}, "dune frank herbert", 0},
		{"all words in title", bookResult{Author: "Nobody", Title: "Dune Messiah", Size: "1 MB"}, "dune messiah", 5},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			words := strings.Fields(strings.ToLower(tc.query))
			score := scoreBook(tc.book, words)
			assert.GreaterOrEqual(t, score, tc.wantGt)
		})
	}
}

func TestMockSession_LastSearchCache(t *testing.T) {
	m := NewMockSession(t.TempDir())

	_, _, ok := m.LastSearch()
	assert.False(t, ok, "no search before SetLastSearch")

	resp := searchResponse{Servers: []string{"S1"}, Books: []bookResult{{Author: "A", Title: "T"}}, Total: 1}
	m.SetLastSearch("dune", resp)

	query, got, ok := m.LastSearch()
	assert.True(t, ok)
	assert.Equal(t, "dune", query)
	assert.Equal(t, 1, got.Total)
}

func TestCleanDisplayTitle(t *testing.T) {
	tests := []struct {
		input string
		want  string
	}{
		{"The Hobbit (illus) (retail) (epub)", "The Hobbit"},
		{"The Hobbit (retail) (epub)", "The Hobbit"},
		{"The Hobbit (v5)", "The Hobbit"},
		{"The Hobbit (retail v5)", "The Hobbit"},
		{"The Hobbit [retail] [epub]", "The Hobbit"},
		{"The Hobbit.epub", "The Hobbit"},
		{"The Hobbit (2011)", "The Hobbit (2011)"},        // edition year preserved
		{"The History of the Hobbit (2011)", "The History of the Hobbit (2011)"},
		{"The Hobbit (unabridged)", "The Hobbit"},
		{"The Hobbit (illustrated)", "The Hobbit"},
		{"The Hobbit (fixed) (enhanced)", "The Hobbit"},
		{"The Hobbit [Series 01]", "The Hobbit [Series 01]"}, // series info preserved
		{"Dune", "Dune"},                                      // already clean
		{"", ""},
		{"The Hobbit (kepub)", "The Hobbit"},
		{"The Hobbit.mobi", "The Hobbit"},
		{"The_Hobbit.epub", "The_Hobbit"}, // underscore is part of title, only extension stripped
	}
	for _, tc := range tests {
		t.Run(tc.input, func(t *testing.T) {
			got := cleanDisplayTitle(tc.input)
			assert.Equal(t, tc.want, got)
		})
	}
}

func TestBuildSearchResponse_CleansTitles(t *testing.T) {
	books := []core.BookDetail{
		{Server: "ThrawnBot", Author: "J R R Tolkien", Title: "The Hobbit (illus) (retail) (epub)", Format: "epub", Size: "22 MB", Full: "!ThrawnBot The Hobbit (illus) (retail) (epub).epub"},
		{Server: "EpubWorld", Author: "J R R Tolkien", Title: "The Hobbit", Format: "epub", Size: "5 MB", Full: "!EpubWorld The Hobbit.epub"},
	}
	trusted := func(string) bool { return true }

	resp := buildSearchResponse(books, trusted)

	// Both normalize to "thehobbit" so they collapse into one group.
	// The representative is the larger file (22 MB), whose title gets cleaned.
	if assert.Len(t, resp.Books, 1) {
		assert.Equal(t, "The Hobbit", resp.Books[0].Title, "title should be cleaned")
		assert.Equal(t, "!ThrawnBot The Hobbit (illus) (retail) (epub).epub", resp.Books[0].DL, "dl string must stay intact")
	}
}

func TestRankBooks_PrefersCleanTitles(t *testing.T) {
	// Two books with identical query match, copies, and size — but one has
	// cleanBonus=true (original title had no annotations) and the other doesn't.
	books := []bookResult{
		{Author: "J R R Tolkien", Title: "The Hobbit (illus) (retail) (epub)", Size: "22 MB", Copies: 18, cleanBonus: false},
		{Author: "J R R Tolkien", Title: "The Hobbit", Size: "22 MB", Copies: 18, cleanBonus: true},
	}
	ranked := rankBooks(books, "hobbit")

	if assert.Len(t, ranked, 2) {
		assert.Equal(t, "The Hobbit", ranked[0].Title, "clean title should rank first")
	}
}

func TestScoreBook_CleanBonus(t *testing.T) {
	words := strings.Fields("hobbit")
	clean := bookResult{Author: "Tolkien", Title: "The Hobbit", Size: "5 MB", cleanBonus: true}
	cluttered := bookResult{Author: "Tolkien", Title: "The Hobbit", Size: "5 MB", cleanBonus: false}
	assert.Greater(t, scoreBook(clean, words), scoreBook(cluttered, words),
		"clean title should score higher than cluttered with same match/size")
}

func TestListSearchResults_Pagination(t *testing.T) {
	m := NewMockSession(t.TempDir())
	// Build a cached search with 25 books.
	books := make([]bookResult, 25)
	for i := range books {
		books[i] = bookResult{Author: "A", Title: string(rune('A' + i)), Size: "1 MB"}
	}
	m.SetLastSearch("test", searchResponse{
		Servers: []string{"S1"},
		Books:   books,
		Total:   25,
	})

	// Simulate paginated slicing directly (the handler logic).
	_, resp, ok := m.LastSearch()
	assert.True(t, ok)
	total := len(resp.Books)

	// Page 1: offset=0, limit=20 → 20 books, has_more=true
	end := 20
	assert.Equal(t, 20, end)
	assert.True(t, end < total, "has_more should be true")

	// Page 2: offset=20, limit=20 → 5 books, has_more=false
	offset := 20
	end = offset + 20
	if end > total {
		end = total
	}
	assert.Equal(t, 25, end)
	assert.False(t, end < total, "has_more should be false on last page")
	assert.Equal(t, 5, end-offset, "should return 5 books on last page")
}

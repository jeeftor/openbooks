package mcp

import (
	"strings"
	"testing"

	"github.com/evan-buss/openbooks/core"
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

package server

import (
	"encoding/json"
	"strings"
	"testing"

	"github.com/evan-buss/openbooks/core"
)

// TestEPUBMetadataJSONContract verifies that the Go EPUBMetadata struct
// serializes to JSON field names that match the frontend's TypeScript
// EPUBMetadata interface (server/app/src/types/messages.ts).
//
// If this test fails, the frontend will show empty metadata fields because
// the Vue components access .author, .title, .series, .series_index on the
// parsed JSON object.
func TestEPUBMetadataJSONContract(t *testing.T) {
	t.Parallel()

	meta := core.EPUBMetadata{
		Author:      "Frank Herbert",
		Title:       "Dune",
		Series:      "Dune Chronicles",
		SeriesIndex: "1",
	}
	data, err := json.Marshal(meta)
	if err != nil {
		t.Fatalf("marshal EPUBMetadata: %v", err)
	}
	s := string(data)

	requiredFields := []string{
		`"author"`,
		`"title"`,
		`"series"`,
		`"series_index"`,
	}
	for _, field := range requiredFields {
		if !strings.Contains(s, field) {
			t.Fatalf("EPUBMetadata JSON missing %s — frontend expects lowercase field names. Got: %s", field, s)
		}
	}

	// Verify PascalCase fields are NOT present (the old format that would
	// cause the frontend to see empty values).
	forbiddenFields := []string{
		`"Author"`,
		`"Title"`,
		`"Series"`,
		`"SeriesIndex"`,
	}
	for _, field := range forbiddenFields {
		if strings.Contains(s, field) {
			t.Fatalf("EPUBMetadata JSON has PascalCase %s — frontend expects lowercase. Got: %s", field, s)
		}
	}
}

// TestEPUBMetadataOmitemptyContract verifies that empty fields are omitted
// from JSON, matching the frontend's optional (?:) field declarations.
func TestEPUBMetadataOmitemptyContract(t *testing.T) {
	t.Parallel()

	// Series and SeriesIndex empty — should be omitted.
	meta := core.EPUBMetadata{Author: "Frank Herbert", Title: "Dune"}
	data, _ := json.Marshal(meta)
	s := string(data)

	if strings.Contains(s, `"series"`) {
		t.Fatalf("empty series should be omitted via omitempty. Got: %s", s)
	}
	if strings.Contains(s, `"series_index"`) {
		t.Fatalf("empty series_index should be omitted via omitempty. Got: %s", s)
	}
}

// TestRenamePromptResponseJSONContract verifies the WebSocket message that
// carries EPUBMetadata to the frontend has the correct envelope field names.
// The Vue components access .metadata?.author etc. on this object.
func TestRenamePromptResponseJSONContract(t *testing.T) {
	t.Parallel()

	resp := RenamePromptResponse{
		IRCFilename: "test.epub",
		Metadata: &core.EPUBMetadata{
			Author:      "Frank Herbert",
			Title:       "Dune",
			Series:      "Dune Chronicles",
			SeriesIndex: "1",
		},
		ReplaceSpace: "",
	}
	data, err := json.Marshal(resp)
	if err != nil {
		t.Fatalf("marshal RenamePromptResponse: %v", err)
	}
	s := string(data)

	// Envelope fields the frontend expects (from RenamePromptResponse in messages.ts).
	envelopeFields := []string{
		`"ircFilename"`,
		`"metadata"`,
		`"options"`,
	}
	for _, field := range envelopeFields {
		if !strings.Contains(s, field) {
			t.Fatalf("RenamePromptResponse JSON missing %s. Got: %s", field, s)
		}
	}

	// Nested metadata fields.
	nestedFields := []string{
		`"author"`,
		`"title"`,
		`"series"`,
		`"series_index"`,
	}
	for _, field := range nestedFields {
		if !strings.Contains(s, field) {
			t.Fatalf("RenamePromptResponse.metadata missing %s. Got: %s", field, s)
		}
	}
}

// TestStagedBookSummaryJSONContract verifies the staged books list message
// has the correct field names for the frontend StagedBookSummary interface.
func TestStagedBookSummaryJSONContract(t *testing.T) {
	t.Parallel()

	summary := StagedBookSummary{
		ID:          "test-id",
		IRCFilename: "test.epub",
		Metadata: &core.EPUBMetadata{
			Author: "Frank Herbert",
			Title:  "Dune",
		},
		StagedAt: "2024-01-01T00:00:00Z",
	}
	data, err := json.Marshal(summary)
	if err != nil {
		t.Fatalf("marshal StagedBookSummary: %v", err)
	}
	s := string(data)

	envelopeFields := []string{
		`"id"`,
		`"ircFilename"`,
		`"metadata"`,
		`"stagedAt"`,
	}
	for _, field := range envelopeFields {
		if !strings.Contains(s, field) {
			t.Fatalf("StagedBookSummary JSON missing %s. Got: %s", field, s)
		}
	}
}

// TestStagedBookResumeResponseJSONContract verifies the staged book resume
// message has the correct field names for the frontend.
func TestStagedBookResumeResponseJSONContract(t *testing.T) {
	t.Parallel()

	resp := StagedBookResumeResponse{
		StagedID:    "test-id",
		IRCFilename: "test.epub",
		Metadata: &core.EPUBMetadata{
			Author: "Frank Herbert",
			Title:  "Dune",
		},
	}
	data, err := json.Marshal(resp)
	if err != nil {
		t.Fatalf("marshal StagedBookResumeResponse: %v", err)
	}
	s := string(data)

	envelopeFields := []string{
		`"stagedId"`,
		`"ircFilename"`,
		`"metadata"`,
	}
	for _, field := range envelopeFields {
		if !strings.Contains(s, field) {
			t.Fatalf("StagedBookResumeResponse JSON missing %s. Got: %s", field, s)
		}
	}
}

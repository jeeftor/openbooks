package core

import "strings"

// resultsForMarker separates the bot name from the query inside a search
// results filename, e.g. "SearchOok_results_for__the_great_gatsby.txt.zip".
const resultsForMarker = "_results_for__"

// ResultsQueryFromFilename extracts the search query embedded in a bot
// results filename such as "SearchOok_results_for__the_great_gatsby.txt.zip".
// It returns the raw (still underscore-encoded) query, or "" when the
// filename does not match the known pattern.
func ResultsQueryFromFilename(filename string) string {
	idx := strings.Index(filename, resultsForMarker)
	if idx == -1 {
		return ""
	}
	rest := filename[idx+len(resultsForMarker):]
	// Strip the file extension(s): the query ends at the first dot.
	if dot := strings.Index(rest, "."); dot != -1 {
		rest = rest[:dot]
	}
	return rest
}

// normalizeQueryMatch lowercases a query/filename fragment and treats
// underscores and whitespace as equivalent, since bots encode spaces as
// underscores in DCC filenames.
func normalizeQueryMatch(s string) string {
	s = strings.ToLower(strings.ReplaceAll(s, "_", " "))
	return strings.Join(strings.Fields(s), " ")
}

// QueryMatchesResultsFilename reports whether a search-results filename
// plausibly belongs to the given query. It is used to detect stale results:
// a response for an earlier (timed-out) query must not be attributed to a
// newer in-flight search.
//
// The check is deliberately conservative — it returns true whenever it
// cannot prove a mismatch, so legitimate results are never dropped:
//   - filename doesn't match the "*_results_for__*" pattern → true
//   - extracted query is empty → true
//   - normalized queries are equal → true
//   - extracted query is a strict prefix of the search query → true
//     (bots may truncate long queries in filenames)
func QueryMatchesResultsFilename(query, filename string) bool {
	extracted := ResultsQueryFromFilename(filename)
	if extracted == "" {
		return true
	}
	eq := normalizeQueryMatch(extracted)
	nq := normalizeQueryMatch(query)
	if eq == "" || nq == "" || eq == nq {
		return true
	}
	// Truncation tolerance: only when the filename fragment is meaningfully
	// long, so short queries can't false-positive on shared prefixes.
	if len(eq) >= 8 && strings.HasPrefix(nq, eq) {
		return true
	}
	return false
}

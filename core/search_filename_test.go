package core

import "testing"

func TestResultsQueryFromFilename(t *testing.T) {
	cases := []struct {
		name     string
		filename string
		want     string
	}{
		{"searchbot zip", "SearchBot_results_for__the_great_gatsby.txt.zip", "the_great_gatsby"},
		{"searchook zip", "SearchOok_results_for__a_mother_tongue.txt.zip", "a_mother_tongue"},
		{"epub ext", "SearchOok_results_for__dune.epub", "dune"},
		{"no marker", "great-gatsby.epub", ""},
		{"book file", "Stephen King - The Stand.epub", ""},
		{"empty", "", ""},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			if got := ResultsQueryFromFilename(tc.filename); got != tc.want {
				t.Errorf("ResultsQueryFromFilename(%q) = %q, want %q", tc.filename, got, tc.want)
			}
		})
	}
}

func TestQueryMatchesResultsFilename(t *testing.T) {
	cases := []struct {
		name     string
		query    string
		filename string
		want     bool
	}{
		{"exact match", "the great gatsby", "SearchOok_results_for__the_great_gatsby.txt.zip", true},
		{"case insensitive", "The Great Gatsby", "SearchOok_results_for__the_great_gatsby.txt.zip", true},
		{"extra whitespace", "  the   great  gatsby ", "SearchOok_results_for__the_great_gatsby.txt.zip", true},
		{"stale different query", "dune", "SearchOok_results_for__the_great_gatsby.txt.zip", false},
		{"stale unrelated query", "project hail mary", "SearchOok_results_for__the_great_gatsby.txt.zip", false},
		{"unparseable filename accepted", "dune", "random-file.epub", true},
		{"truncated filename tolerated", "a very long search query about maritime history", "SearchOok_results_for__a_very_long_search_query.txt.zip", true},
		{"prefix extension accepted", "the great gatsby book club edition", "SearchOok_results_for__the_great_gatsby.txt.zip", true},
		{"short prefix not tolerated", "dune messiah", "SearchOok_results_for__dune.txt.zip", false},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			if got := QueryMatchesResultsFilename(tc.query, tc.filename); got != tc.want {
				t.Errorf("QueryMatchesResultsFilename(%q, %q) = %v, want %v", tc.query, tc.filename, got, tc.want)
			}
		})
	}
}

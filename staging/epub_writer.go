package staging

import (
	"archive/zip"
	"bytes"
	"fmt"
	"io"
	"os"
	"regexp"
	"strings"
)

// RewriteEPUBMetadata patches the OPF metadata inside an EPUB zip file in-place.
// It updates dc:title, dc:creator (first author), calibre:series, and calibre:series_index meta.
// Pass an empty string to skip a field (leave it unchanged).
// When clearSeries/clearSeriesIndex is true, the corresponding calibre: meta tag is
// removed from the OPF entirely. Errors are non-fatal — caller should log them.
func RewriteEPUBMetadata(epubPath, title, author, series, seriesIndex string, clearSeries, clearSeriesIndex bool) error {
	r, err := zip.OpenReader(epubPath)
	if err != nil {
		return fmt.Errorf("open epub: %w", err)
	}

	opfPath := findOPFInZip(r)

	// Buffer all zip entries so we can close the reader before rewriting.
	type entry struct {
		name    string
		content []byte
	}
	entries := make([]entry, 0, len(r.File))
	for _, f := range r.File {
		rc, err := f.Open()
		if err != nil {
			r.Close()
			return fmt.Errorf("read %s: %w", f.Name, err)
		}
		data, err := io.ReadAll(rc)
		rc.Close()
		if err != nil {
			r.Close()
			return fmt.Errorf("read data %s: %w", f.Name, err)
		}
		if f.Name == opfPath && opfPath != "" {
			data = patchOPF(data, title, author, series, seriesIndex, clearSeries, clearSeriesIndex)
		}
		entries = append(entries, entry{name: f.Name, content: data})
	}
	r.Close()

	// Write new zip to a temp file alongside the original.
	tmpPath := epubPath + ".rewrite.tmp"
	w, err := os.Create(tmpPath)
	if err != nil {
		return fmt.Errorf("create temp: %w", err)
	}

	zw := zip.NewWriter(w)
	for _, e := range entries {
		fw, err := zw.Create(e.name)
		if err != nil {
			w.Close()
			os.Remove(tmpPath)
			return fmt.Errorf("write entry %s: %w", e.name, err)
		}
		if _, err := fw.Write(e.content); err != nil {
			w.Close()
			os.Remove(tmpPath)
			return fmt.Errorf("write content %s: %w", e.name, err)
		}
	}
	if err := zw.Close(); err != nil {
		w.Close()
		os.Remove(tmpPath)
		return fmt.Errorf("close zip: %w", err)
	}
	w.Close()

	if err := os.Rename(tmpPath, epubPath); err != nil {
		os.Remove(tmpPath)
		return fmt.Errorf("replace epub: %w", err)
	}
	return nil
}

// findOPFInZip locates the .opf file by scanning zip entries.
func findOPFInZip(r *zip.ReadCloser) string {
	for _, f := range r.File {
		if strings.HasSuffix(strings.ToLower(f.Name), ".opf") {
			return f.Name
		}
	}
	return ""
}

var (
	// Matches <dc:title ...>content</dc:title> (case-insensitive, single-line values)
	reDCTitle = regexp.MustCompile(`(?i)<dc:title[^>]*>[^<]*</dc:title>`)

	// Matches <dc:creator ...>content</dc:creator>
	reDCCreator = regexp.MustCompile(`(?i)<dc:creator[^>]*>[^<]*</dc:creator>`)

	// Matches any <meta> tag that has name="calibre:series"
	reCalSeries = regexp.MustCompile(`(?i)<meta\b[^>]*\bname="calibre:series"[^>]*/?>`)

	// Matches any <meta> tag that has name="calibre:series_index"
	reCalSeriesIndex = regexp.MustCompile(`(?i)<meta\b[^>]*\bname="calibre:series_index"[^>]*/?>`)

	// Matches closing OPF metadata tags, including namespaced forms like </opf:metadata>.
	reMetadataClose = regexp.MustCompile(`(?i)</(?:[a-z_][\w.-]*:)?metadata\s*>`)
)

// patchOPF applies targeted substitutions to OPF XML bytes.
// When clearSeries/clearSeriesIndex is true, the corresponding meta tag is
// removed entirely instead of being updated.
func patchOPF(data []byte, title, author, series, seriesIndex string, clearSeries, clearSeriesIndex bool) []byte {
	s := string(data)

	if title != "" {
		s = reDCTitle.ReplaceAllString(s, "<dc:title>"+xmlEsc(title)+"</dc:title>")
	}
	if author != "" {
		// Only replace the first dc:creator to leave other contributors intact.
		s = replaceFirstMatch(s, reDCCreator,
			`<dc:creator opf:role="aut">`+xmlEsc(author)+`</dc:creator>`)
	}
	if clearSeries {
		s = reCalSeries.ReplaceAllString(s, "")
	} else if series != "" {
		repl := `<meta name="calibre:series" content="` + xmlEsc(series) + `"/>`
		if reCalSeries.MatchString(s) {
			s = reCalSeries.ReplaceAllString(s, repl)
		} else {
			s = insertBeforeMetadataClose(s, repl)
		}
	}
	if clearSeriesIndex {
		s = reCalSeriesIndex.ReplaceAllString(s, "")
	} else if seriesIndex != "" {
		repl := `<meta name="calibre:series_index" content="` + xmlEsc(seriesIndex) + `"/>`
		if reCalSeriesIndex.MatchString(s) {
			s = reCalSeriesIndex.ReplaceAllString(s, repl)
		} else {
			s = insertBeforeMetadataClose(s, repl)
		}
	}

	return []byte(s)
}

func insertBeforeMetadataClose(s, repl string) string {
	loc := reMetadataClose.FindStringIndex(s)
	if loc == nil {
		return s
	}
	return s[:loc[0]] + repl + "\n    " + s[loc[0]:]
}

// replaceFirstMatch replaces only the first occurrence found by re.
func replaceFirstMatch(s string, re *regexp.Regexp, repl string) string {
	loc := re.FindStringIndex(s)
	if loc == nil {
		return s
	}
	var b bytes.Buffer
	b.WriteString(s[:loc[0]])
	b.WriteString(repl)
	b.WriteString(s[loc[1]:])
	return b.String()
}

// xmlEsc escapes characters that are special in XML attribute/element values.
func xmlEsc(s string) string {
	s = strings.ReplaceAll(s, "&", "&amp;")
	s = strings.ReplaceAll(s, "<", "&lt;")
	s = strings.ReplaceAll(s, ">", "&gt;")
	s = strings.ReplaceAll(s, `"`, "&quot;")
	return s
}

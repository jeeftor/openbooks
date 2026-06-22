package staging

import (
	"strings"
	"testing"
)

func TestPatchOPFWritesAudiobookshelfSeriesTags(t *testing.T) {
	t.Parallel()

	input := []byte(`<package xmlns:dc="http://purl.org/dc/elements/1.1/">
  <metadata>
    <dc:title>Old Title</dc:title>
    <dc:creator>Old Author</dc:creator>
  </metadata>
</package>`)

	got := string(patchOPF(input, "Dragons of Autumn Twilight", "Margaret Weis", "Dragonlance Chronicles", "1", false, false))

	wantSeries := `<meta name="calibre:series" content="Dragonlance Chronicles"/>`
	wantIndex := `<meta name="calibre:series_index" content="1"/>`
	if !strings.Contains(got, wantSeries) {
		t.Fatalf("patched OPF missing series tag %q:\n%s", wantSeries, got)
	}
	if !strings.Contains(got, wantIndex) {
		t.Fatalf("patched OPF missing series index tag %q:\n%s", wantIndex, got)
	}
	if strings.Index(got, wantSeries) > strings.Index(got, wantIndex) {
		t.Fatalf("series tag must precede series index tag for ABS pairing:\n%s", got)
	}
}

func TestPatchOPFInsertsSeriesTagsBeforeNamespacedMetadataClose(t *testing.T) {
	t.Parallel()

	input := []byte(`<opf:package xmlns:opf="http://www.idpf.org/2007/opf">
  <opf:metadata>
    <dc:title>Old Title</dc:title>
  </opf:metadata>
</opf:package>`)

	got := string(patchOPF(input, "", "", "Dragonlance Chronicles", "1", false, false))

	if !strings.Contains(got, `<meta name="calibre:series" content="Dragonlance Chronicles"/>`) {
		t.Fatalf("patched namespaced OPF missing series tag:\n%s", got)
	}
	if !strings.Contains(got, `<meta name="calibre:series_index" content="1"/>`) {
		t.Fatalf("patched namespaced OPF missing series index tag:\n%s", got)
	}
	if !strings.Contains(got, `</opf:metadata>`) {
		t.Fatalf("patched namespaced OPF should preserve metadata close tag:\n%s", got)
	}
}

func TestPatchOPFClearsSeriesTags(t *testing.T) {
	t.Parallel()

	input := []byte(`<package xmlns:dc="http://purl.org/dc/elements/1.1/">
  <metadata>
    <dc:title>The Hobbit</dc:title>
    <dc:creator>J R R Tolkien</dc:creator>
    <meta name="calibre:series" content="The Lord of the Rings"/>
    <meta name="calibre:series_index" content="0"/>
  </metadata>
</package>`)

	got := string(patchOPF(input, "", "", "", "", true, true))

	if strings.Contains(got, "calibre:series") {
		t.Fatalf("clear_series should remove calibre:series tag:\n%s", got)
	}
	if strings.Contains(got, "calibre:series_index") {
		t.Fatalf("clear_series_index should remove calibre:series_index tag:\n%s", got)
	}
	if !strings.Contains(got, "<dc:title>The Hobbit</dc:title>") {
		t.Fatalf("clearing series should not affect title:\n%s", got)
	}
	if !strings.Contains(got, "<dc:creator>J R R Tolkien</dc:creator>") {
		t.Fatalf("clearing series should not affect author:\n%s", got)
	}
}

func TestPatchOPFClearsSeriesOnly(t *testing.T) {
	t.Parallel()

	input := []byte(`<package xmlns:dc="http://purl.org/dc/elements/1.1/">
  <metadata>
    <dc:title>The Hobbit</dc:title>
    <meta name="calibre:series" content="The Lord of the Rings"/>
    <meta name="calibre:series_index" content="0"/>
  </metadata>
</package>`)

	got := string(patchOPF(input, "", "", "", "", true, false))

	if strings.Contains(got, "calibre:series\"") {
		t.Fatalf("clear_series should remove calibre:series tag:\n%s", got)
	}
	// series_index should still be present since clearSeriesIndex is false
	if !strings.Contains(got, "calibre:series_index") {
		t.Fatalf("series_index should be preserved when clear_series_index is false:\n%s", got)
	}
}

func TestPatchOPFClearsSeriesIndexOnly(t *testing.T) {
	t.Parallel()

	input := []byte(`<package xmlns:dc="http://purl.org/dc/elements/1.1/">
  <metadata>
    <dc:title>The Hobbit</dc:title>
    <meta name="calibre:series" content="The Lord of the Rings"/>
    <meta name="calibre:series_index" content="0"/>
  </metadata>
</package>`)

	got := string(patchOPF(input, "", "", "", "", false, true))

	if strings.Contains(got, "calibre:series_index") {
		t.Fatalf("clear_series_index should remove calibre:series_index tag:\n%s", got)
	}
	// series should still be present since clearSeries is false
	if !strings.Contains(got, "calibre:series") {
		t.Fatalf("series should be preserved when clear_series is false:\n%s", got)
	}
}

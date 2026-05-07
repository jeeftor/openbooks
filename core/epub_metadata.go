package core

import (
	"archive/zip"
	"encoding/xml"
	"io"
	"path"
	"strings"
)

// EPUBMetadata holds the author, title, and optional series extracted from an EPUB's OPF file.
type EPUBMetadata struct {
	Author      string
	Title       string
	Series      string
	SeriesIndex string // e.g. "1", "2.5" — calibre:series_index
}

type containerXML struct {
	Rootfile struct {
		FullPath string `xml:"full-path,attr"`
	} `xml:"rootfiles>rootfile"`
}

type opfPackage struct {
	Metadata struct {
		Titles   []string     `xml:"title"`
		Creators []opfCreator `xml:"creator"`
		Metas    []opfMeta    `xml:"meta"`
	} `xml:"metadata"`
	Manifest struct {
		Items []opfItem `xml:"item"`
	} `xml:"manifest"`
}

type opfCreator struct {
	Role  string `xml:"role,attr"`
	Value string `xml:",chardata"`
}

type opfMeta struct {
	Name    string `xml:"name,attr"`
	Content string `xml:"content,attr"`
}

type opfItem struct {
	ID         string `xml:"id,attr"`
	Href       string `xml:"href,attr"`
	MediaType  string `xml:"media-type,attr"`
	Properties string `xml:"properties,attr"`
}

// ReadEPUBMetadata opens an EPUB file and extracts author, title, and series from its OPF metadata.
func ReadEPUBMetadata(path string) (*EPUBMetadata, error) {
	r, err := zip.OpenReader(path)
	if err != nil {
		return nil, err
	}
	defer r.Close()

	// Step 1: read META-INF/container.xml to find the OPF rootfile path
	opfPath, err := findOPFPath(r)
	if err != nil {
		return nil, err
	}

	// Step 2: parse the OPF file
	opfFile := findZipFile(r, opfPath)
	if opfFile == nil {
		return nil, nil
	}

	rc, err := opfFile.Open()
	if err != nil {
		return nil, err
	}
	defer rc.Close()

	data, err := io.ReadAll(rc)
	if err != nil {
		return nil, err
	}

	var pkg opfPackage
	if err := xml.Unmarshal(data, &pkg); err != nil {
		return nil, err
	}

	meta := &EPUBMetadata{}

	if len(pkg.Metadata.Titles) > 0 {
		meta.Title = strings.TrimSpace(pkg.Metadata.Titles[0])
	}

	// Prefer creator with role="aut", fall back to first creator
	for _, c := range pkg.Metadata.Creators {
		if strings.EqualFold(c.Role, "aut") {
			meta.Author = strings.TrimSpace(c.Value)
			break
		}
	}
	if meta.Author == "" && len(pkg.Metadata.Creators) > 0 {
		meta.Author = strings.TrimSpace(pkg.Metadata.Creators[0].Value)
	}

	// Extract Calibre series and series_index tags
	for _, m := range pkg.Metadata.Metas {
		switch {
		case strings.EqualFold(m.Name, "calibre:series"):
			meta.Series = strings.TrimSpace(m.Content)
		case strings.EqualFold(m.Name, "calibre:series_index"):
			meta.SeriesIndex = strings.TrimSpace(m.Content)
		}
	}

	return meta, nil
}

func findOPFPath(r *zip.ReadCloser) (string, error) {
	f := findZipFile(r, "META-INF/container.xml")
	if f == nil {
		return "", nil
	}
	rc, err := f.Open()
	if err != nil {
		return "", err
	}
	defer rc.Close()

	var container containerXML
	if err := xml.NewDecoder(rc).Decode(&container); err != nil {
		return "", err
	}
	return container.Rootfile.FullPath, nil
}

func findZipFile(r *zip.ReadCloser, name string) *zip.File {
	for _, f := range r.File {
		if f.Name == name {
			return f
		}
	}
	return nil
}

// ExtractCoverImage reads an EPUB and returns the cover image bytes and MIME type.
// Returns nil, "", nil if no cover is found.
func ExtractCoverImage(epubPath string) ([]byte, string, error) {
	r, err := zip.OpenReader(epubPath)
	if err != nil {
		return nil, "", err
	}
	defer r.Close()

	opfPath, err := findOPFPath(r)
	if err != nil || opfPath == "" {
		return nil, "", err
	}

	opfFile := findZipFile(r, opfPath)
	if opfFile == nil {
		return nil, "", nil
	}

	rc, err := opfFile.Open()
	if err != nil {
		return nil, "", err
	}
	data, err := io.ReadAll(rc)
	rc.Close()
	if err != nil {
		return nil, "", err
	}

	var pkg opfPackage
	if err := xml.Unmarshal(data, &pkg); err != nil {
		return nil, "", err
	}

	opfDir := path.Dir(opfPath)

	// Strategy 1: EPUB 3 — manifest item with properties="cover-image"
	for _, item := range pkg.Manifest.Items {
		if strings.Contains(item.Properties, "cover-image") && strings.HasPrefix(item.MediaType, "image/") {
			return readZipImage(r, opfDir, item)
		}
	}

	// Strategy 2: EPUB 2 — <meta name="cover" content="itemId"/>
	for _, m := range pkg.Metadata.Metas {
		if strings.EqualFold(m.Name, "cover") && m.Content != "" {
			for _, item := range pkg.Manifest.Items {
				if item.ID == m.Content && strings.HasPrefix(item.MediaType, "image/") {
					return readZipImage(r, opfDir, item)
				}
			}
		}
	}

	// Strategy 3: Fallback — look for an image with "cover" in its href
	for _, item := range pkg.Manifest.Items {
		if strings.HasPrefix(item.MediaType, "image/") {
			lower := strings.ToLower(item.Href)
			if strings.Contains(lower, "cover") {
				return readZipImage(r, opfDir, item)
			}
		}
	}

	return nil, "", nil
}

func readZipImage(r *zip.ReadCloser, opfDir string, item opfItem) ([]byte, string, error) {
	// Href is relative to OPF directory.
	fullPath := item.Href
	if opfDir != "." {
		fullPath = opfDir + "/" + item.Href
	}
	f := findZipFile(r, fullPath)
	if f == nil {
		return nil, "", nil
	}
	rc, err := f.Open()
	if err != nil {
		return nil, "", err
	}
	defer rc.Close()
	data, err := io.ReadAll(rc)
	if err != nil {
		return nil, "", err
	}
	return data, item.MediaType, nil
}

package core

import (
	"archive/zip"
	"encoding/xml"
	"io"
	"strings"
)

// EPUBMetadata holds the author, title, and optional series extracted from an EPUB's OPF file.
type EPUBMetadata struct {
	Author string
	Title  string
	Series string
}

type containerXML struct {
	Rootfile struct {
		FullPath string `xml:"full-path,attr"`
	} `xml:"rootfiles>rootfile"`
}

type opfPackage struct {
	Metadata struct {
		Titles   []string    `xml:"title"`
		Creators []opfCreator `xml:"creator"`
		Metas    []opfMeta   `xml:"meta"`
	} `xml:"metadata"`
}

type opfCreator struct {
	Role  string `xml:"role,attr"`
	Value string `xml:",chardata"`
}

type opfMeta struct {
	Name    string `xml:"name,attr"`
	Content string `xml:"content,attr"`
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

	// Extract Calibre series tag
	for _, m := range pkg.Metadata.Metas {
		if strings.EqualFold(m.Name, "calibre:series") {
			meta.Series = strings.TrimSpace(m.Content)
			break
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

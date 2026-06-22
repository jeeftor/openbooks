// Package staging contains the shared download→rename→organize logic used by
// both the web server (websocket flow) and the MCP server (agent flow).
//
// A downloaded book is staged in a hidden directory, its EPUB metadata is read,
// rename options are built, and once the caller (browser user or AI agent)
// confirms a Choice, the file is moved to its final organised path with the
// EPUB internal metadata optionally rewritten.
package staging

import (
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"

	"github.com/jeeftor/openbooks/core"
)

// Option is one naming choice presented to the user/agent.
type Option struct {
	ID          string `json:"id"`
	Label       string `json:"label"`
	Preview     string `json:"preview"`               // path relative to downloadDir, forward slashes
	IsOrganized bool   `json:"isOrganized,omitempty"` // true if it creates subdirectories
}

// Choice is the user/agent's rename decision. Metadata fields may differ from
// the extracted EPUBMetadata when the caller edited them before confirming.
// ClearSeries/ClearSeriesIndex explicitly remove a field instead of falling
// back to the extracted value.
type Choice struct {
	OptionID          string
	CustomName        string
	FileName          string
	RewriteMetadata   bool
	Author            string
	Title             string
	Series            string
	SeriesIndex       string
	ClearSeries       bool
	ClearSeriesIndex  bool
}

// StagingDir returns the hidden staging subdirectory inside downloadDir.
func StagingDir(downloadDir string) string {
	return filepath.Join(downloadDir, ".staging")
}

// EnsureStagingDir creates the staging directory if it does not exist.
func EnsureStagingDir(downloadDir string) error {
	return os.MkdirAll(StagingDir(downloadDir), 0755)
}

// BuildOptions generates the list of naming choices for the rename prompt.
// The preview strings use forward slashes and are relative to downloadDir.
func BuildOptions(ircFilename string, meta *core.EPUBMetadata, replaceSpace string) []Option {
	isEPUB := strings.EqualFold(filepath.Ext(ircFilename), ".epub")

	opts := []Option{
		{ID: "keep", Label: "Keep IRC filename", Preview: ircFilename},
	}

	if !isEPUB || meta == nil || meta.Title == "" {
		return opts
	}

	rs := replaceSpace
	ext := strings.ToLower(filepath.Ext(ircFilename))
	title := SanitizePathComponent(meta.Title, rs)

	opts = append(opts, Option{
		ID:      "title",
		Label:   "Title only",
		Preview: title + ext,
	})

	if meta.Author != "" {
		author := SanitizePathComponent(meta.Author, rs)

		opts = append(opts, Option{
			ID:      "author-title-flat",
			Label:   "Author - Title (flat)",
			Preview: fmt.Sprintf("%s - %s%s", author, title, ext),
		})

		opts = append(opts, Option{
			ID:          "organized",
			Label:       "Organized: Author / Title /",
			Preview:     fmt.Sprintf("%s/%s/%s%s", author, title, title, ext),
			IsOrganized: true,
		})

		// Always offer the series option when we have author + title.
		// If the extracted series is empty, show a placeholder so the user
		// knows they can provide one at confirm time.
		seriesLabel := meta.Series
		if seriesLabel == "" {
			seriesLabel = "[series]"
		}
		series := SanitizePathComponent(seriesLabel, rs)
		opts = append(opts, Option{
			ID:          "series",
			Label:       "Organized: Author / Series / Title /",
			Preview:     fmt.Sprintf("%s/%s/%s/%s%s", author, series, title, title, ext),
			IsOrganized: true,
		})
	}

	return opts
}

// ResolveFinalPath computes the absolute destination path given the caller's
// rename choice. It rebuilds the path from the choice's metadata fields (which
// may have been edited) rather than from the original extracted metadata.
func ResolveFinalPath(
	downloadDir string,
	choice Choice,
	ircFilename string,
	meta *core.EPUBMetadata,
	replaceSpace string,
) string {
	rs := replaceSpace
	ext := strings.ToLower(filepath.Ext(ircFilename))

	// Use the caller-supplied metadata fields (may differ from extracted if they edited them)
	author := SanitizePathComponent(choice.Author, rs)
	title := SanitizePathComponent(choice.Title, rs)
	series := SanitizePathComponent(choice.Series, rs)
	if choice.ClearSeries {
		series = ""
	}

	// Fall back to extracted metadata when the caller didn't supply edited values
	// and didn't explicitly clear the field.
	if title == "" && meta != nil {
		title = SanitizePathComponent(meta.Title, rs)
	}
	if author == "" && meta != nil {
		author = SanitizePathComponent(meta.Author, rs)
	}
	if series == "" && meta != nil && !choice.ClearSeries {
		series = SanitizePathComponent(meta.Series, rs)
	}
	fileName := resolveChoiceFileName(choice, title, ext, rs)

	switch choice.OptionID {
	case "keep":
		return filepath.Join(downloadDir, ircFilename)

	case "title":
		if fileName == "" {
			return filepath.Join(downloadDir, ircFilename)
		}
		return filepath.Join(downloadDir, fileName)

	case "author-title-flat":
		if author == "" || fileName == "" {
			return filepath.Join(downloadDir, ircFilename)
		}
		return filepath.Join(downloadDir, fmt.Sprintf("%s - %s", author, fileName))

	case "organized":
		if author == "" || title == "" || fileName == "" {
			return filepath.Join(downloadDir, ircFilename)
		}
		return filepath.Join(downloadDir, author, title, fileName)

	case "series":
		if author == "" || title == "" || fileName == "" {
			return filepath.Join(downloadDir, ircFilename)
		}
		if series == "" {
			return filepath.Join(downloadDir, author, title, fileName)
		}
		return filepath.Join(downloadDir, author, series, title, fileName)

	case "custom":
		name := strings.TrimSpace(choice.CustomName)
		if name == "" {
			return filepath.Join(downloadDir, ircFilename)
		}
		return filepath.Join(downloadDir, SanitizePathComponent(name, rs))

	default:
		return filepath.Join(downloadDir, ircFilename)
	}
}

func resolveChoiceFileName(choice Choice, title, ext, replaceSpace string) string {
	fileName := SanitizePathComponent(choice.FileName, replaceSpace)
	if fileName == "" && title != "" {
		fileName = title + ext
	}
	if fileName == "" || filepath.Ext(fileName) != "" {
		return fileName
	}
	return fileName + ext
}

// MoveFile moves src to dst, creating parent directories as needed.
// Falls back to copy-and-delete when os.Rename fails (e.g. cross-device).
func MoveFile(src, dst string) error {
	if err := os.MkdirAll(filepath.Dir(dst), 0755); err != nil {
		return fmt.Errorf("create target dir: %w", err)
	}
	if err := os.Rename(src, dst); err == nil {
		return nil
	}
	// Cross-device fallback
	if err := CopyFile(src, dst); err != nil {
		return err
	}
	return os.Remove(src)
}

// CopyFile copies src to dst, creating parent directories as needed.
func CopyFile(src, dst string) error {
	if err := os.MkdirAll(filepath.Dir(dst), 0755); err != nil {
		return fmt.Errorf("create target dir: %w", err)
	}

	in, err := os.Open(src)
	if err != nil {
		return fmt.Errorf("open source: %w", err)
	}
	defer in.Close()

	out, err := os.Create(dst)
	if err != nil {
		return fmt.Errorf("create target: %w", err)
	}
	defer out.Close()

	if _, err := io.Copy(out, in); err != nil {
		os.Remove(dst)
		return fmt.Errorf("copy: %w", err)
	}
	return out.Close()
}

// OriginalCopyPath returns the path used to preserve a pristine copy of the
// downloaded file alongside the final (possibly metadata-rewritten) one.
func OriginalCopyPath(path string) string {
	ext := filepath.Ext(path)
	if ext == "" {
		return path + ".orig"
	}
	return strings.TrimSuffix(path, ext) + ".orig" + ext
}

// SanitizePathComponent trims whitespace, replaces path separators with dashes,
// and optionally replaces spaces with the given character.
func SanitizePathComponent(s, replaceSpace string) string {
	s = strings.TrimSpace(s)
	s = strings.ReplaceAll(s, "/", "-")
	s = strings.ReplaceAll(s, "\\", "-")
	if replaceSpace != "" {
		s = strings.ReplaceAll(s, " ", replaceSpace)
	}
	// Collapse consecutive dashes.
	for strings.Contains(s, "--") {
		s = strings.ReplaceAll(s, "--", "-")
	}
	// Trim leading/trailing dots and dashes after dash collapsing.
	s = strings.Trim(s, ".-")
	// Remove control characters.
	s = strings.Map(func(r rune) rune {
		if r < 0x20 {
			return -1
		}
		return r
	}, s)
	return s
}

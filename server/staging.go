package server

import (
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"

	"github.com/evan-buss/openbooks/core"
)

// stagingDir returns the hidden staging subdirectory inside downloadDir.
func stagingDir(downloadDir string) string {
	return filepath.Join(downloadDir, ".staging")
}

// ensureStagingDir creates the staging directory if it does not exist.
func ensureStagingDir(downloadDir string) error {
	return os.MkdirAll(stagingDir(downloadDir), 0755)
}

// buildRenameOptions generates the list of naming choices for the rename modal.
// The preview strings use forward slashes and are relative to downloadDir.
func buildRenameOptions(ircFilename string, meta *core.EPUBMetadata, replaceSpace string) []RenameOption {
	isEPUB := strings.EqualFold(filepath.Ext(ircFilename), ".epub")

	opts := []RenameOption{
		{ID: "keep", Label: "Keep IRC filename", Preview: ircFilename},
	}

	if !isEPUB || meta == nil || meta.Title == "" {
		return opts
	}

	rs := replaceSpace
	ext := strings.ToLower(filepath.Ext(ircFilename))
	title := sanitizePathComponent(meta.Title, rs)

	opts = append(opts, RenameOption{
		ID:      "title",
		Label:   "Title only",
		Preview: title + ext,
	})

	if meta.Author != "" {
		author := sanitizePathComponent(meta.Author, rs)

		opts = append(opts, RenameOption{
			ID:      "author-title-flat",
			Label:   "Author - Title (flat)",
			Preview: fmt.Sprintf("%s - %s%s", author, title, ext),
		})

		opts = append(opts, RenameOption{
			ID:          "organized",
			Label:       "Organized: Author / Title /",
			Preview:     fmt.Sprintf("%s/%s/%s%s", author, title, title, ext),
			IsOrganized: true,
		})

		if meta.Series != "" {
			series := sanitizePathComponent(meta.Series, rs)
			opts = append(opts, RenameOption{
				ID:          "series",
				Label:       "Organized: Author / Series / Title /",
				Preview:     fmt.Sprintf("%s/%s/%s/%s%s", author, series, title, title, ext),
				IsOrganized: true,
			})
		}
	}

	return opts
}

// resolveFinalPath computes the absolute destination path given the user's rename choice.
// It rebuilds the path from the choice's metadata fields (which may have been edited by the user)
// rather than from the original extracted metadata.
func resolveFinalPath(
	downloadDir string,
	choice RenameChoice,
	ircFilename string,
	meta *core.EPUBMetadata,
	replaceSpace string,
) string {
	rs := replaceSpace
	ext := strings.ToLower(filepath.Ext(ircFilename))

	// Use the user-supplied metadata fields (may differ from extracted if they edited them)
	author := sanitizePathComponent(choice.Author, rs)
	title := sanitizePathComponent(choice.Title, rs)
	series := sanitizePathComponent(choice.Series, rs)

	// Fall back to extracted metadata when user didn't supply edited values
	if title == "" && meta != nil {
		title = sanitizePathComponent(meta.Title, rs)
	}
	if author == "" && meta != nil {
		author = sanitizePathComponent(meta.Author, rs)
	}
	if series == "" && meta != nil {
		series = sanitizePathComponent(meta.Series, rs)
	}

	switch choice.OptionID {
	case "keep":
		return filepath.Join(downloadDir, ircFilename)

	case "title":
		if title == "" {
			return filepath.Join(downloadDir, ircFilename)
		}
		return filepath.Join(downloadDir, title+ext)

	case "author-title-flat":
		if author == "" || title == "" {
			return filepath.Join(downloadDir, ircFilename)
		}
		return filepath.Join(downloadDir, fmt.Sprintf("%s - %s%s", author, title, ext))

	case "organized":
		if author == "" || title == "" {
			return filepath.Join(downloadDir, ircFilename)
		}
		return filepath.Join(downloadDir, author, title, title+ext)

	case "series":
		if author == "" || title == "" {
			return filepath.Join(downloadDir, ircFilename)
		}
		if series == "" {
			return filepath.Join(downloadDir, author, title, title+ext)
		}
		return filepath.Join(downloadDir, author, series, title, title+ext)

	case "custom":
		name := strings.TrimSpace(choice.CustomName)
		if name == "" {
			return filepath.Join(downloadDir, ircFilename)
		}
		return filepath.Join(downloadDir, sanitizePathComponent(name, rs))

	default:
		return filepath.Join(downloadDir, ircFilename)
	}
}

// moveFile moves src to dst, creating parent directories as needed.
// Falls back to copy-and-delete when os.Rename fails (e.g. cross-device).
func moveFile(src, dst string) error {
	if err := os.MkdirAll(filepath.Dir(dst), 0755); err != nil {
		return fmt.Errorf("create target dir: %w", err)
	}
	if err := os.Rename(src, dst); err == nil {
		return nil
	}
	// Cross-device fallback
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
	out.Close()
	in.Close()
	return os.Remove(src)
}

// copyFile copies src to dst, creating parent directories as needed.
func copyFile(src, dst string) error {
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

func originalCopyPath(path string) string {
	ext := filepath.Ext(path)
	if ext == "" {
		return path + ".orig"
	}
	return strings.TrimSuffix(path, ext) + ".orig" + ext
}

package server

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/jeeftor/openbooks/core"
	"github.com/jeeftor/openbooks/staging"
)

func fileSizeMB(path string) string {
	info, err := os.Stat(path)
	if err != nil {
		return "? MB"
	}
	return formatBytes(info.Size())
}

// autoRenameChoice builds a RenameChoice for the configured auto-rename option.
// Falls back to "keep" if the requested option isn't available (e.g. no metadata).
func autoRenameChoice(optionID string, options []staging.Option, meta *core.EPUBMetadata) RenameChoice {
	found := false
	for _, opt := range options {
		if opt.ID == optionID {
			found = true
			break
		}
	}
	if !found {
		return RenameChoice{OptionID: "keep"}
	}

	choice := RenameChoice{OptionID: optionID}
	if meta != nil {
		choice.Author = meta.Author
		choice.Title = meta.Title
		choice.Series = meta.Series
		choice.SeriesIndex = meta.SeriesIndex
	}
	return choice
}

// finalizeRename moves the file to its final path, handles dev-mode originals,
// optional metadata rewrite, series tracking, and notifications. Shared by the
// auto-rename and interactive rename paths. The caller is responsible for
// resolving finalPath (and handling conflicts) before calling this function.
func finalizeRename(
	choice RenameChoice,
	options []staging.Option,
	meta *core.EPUBMetadata,
	ircFilename string,
	extractedPath string,
	stagedOriginalPath string,
	finalPath string,
	dir string,
	config *Config,
	sess_lb *logSession,
	sess *session,
	seriesReg *SeriesRegistry,
) {
	optionLabel := choice.OptionID
	for _, opt := range options {
		if opt.ID == choice.OptionID {
			optionLabel = opt.Label
			break
		}
	}

	if err := staging.MoveFile(extractedPath, finalPath); err != nil {
		sess_lb.error(fmt.Sprintf("Failed to move file: %v", err))
		finalPath = extractedPath
	}
	if stagedOriginalPath != "" {
		originalFinalPath := staging.OriginalCopyPath(finalPath)
		if err := staging.MoveFile(stagedOriginalPath, originalFinalPath); err != nil {
			sess_lb.warn(fmt.Sprintf("Failed to save original copy: %v", err))
		} else {
			relOrig, _ := filepath.Rel(dir, originalFinalPath)
			sess_lb.infoDetail(
				fmt.Sprintf("🧪 Original preserved: %s", filepath.ToSlash(relOrig)),
				fmt.Sprintf("Path: %s", originalFinalPath),
			)
		}
	}

	if choice.RewriteMetadata && strings.EqualFold(filepath.Ext(finalPath), ".epub") {
		if err := staging.RewriteEPUBMetadata(finalPath, choice.Title, choice.Author, choice.Series, choice.SeriesIndex, choice.ClearSeries, choice.ClearSeriesIndex); err != nil {
			sess_lb.warn(fmt.Sprintf("Metadata rewrite failed: %v", err))
		} else {
			sess_lb.infoDetail("✏️  Metadata rewritten",
				fmt.Sprintf("Author: %s\nTitle: %s\nSeries: %s\nBook #: %s",
					choice.Author, choice.Title, choice.Series, choice.SeriesIndex))
		}
	}

	if choice.Series != "" {
		seriesReg.AddIfNew(choice.Series)
	}

	rel, _ := filepath.Rel(dir, finalPath)
	relSlash := filepath.ToSlash(rel)
	savedDetail := fmt.Sprintf("Option: %s\nAuthor: %s\nTitle: %s\nSeries: %s\nBook #: %s\nPath: %s",
		optionLabel, choice.Author, choice.Title, choice.Series, choice.SeriesIndex, finalPath)
	sess_lb.infoDetail(fmt.Sprintf("✅ Saved [%s]: %s", optionLabel, relSlash), savedDetail)

	if sess != nil {
		broadcastToClients(sess.getClients(), newDownloadResponse(finalPath, dir))
	}
}

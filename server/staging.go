package server

import (
	"github.com/evan-buss/openbooks/core"
	"github.com/evan-buss/openbooks/staging"
)

// stagingDir returns the hidden staging subdirectory inside downloadDir.
func stagingDir(downloadDir string) string {
	return staging.StagingDir(downloadDir)
}

// ensureStagingDir creates the staging directory if it does not exist.
func ensureStagingDir(downloadDir string) error {
	return staging.EnsureStagingDir(downloadDir)
}

// buildRenameOptions generates the list of naming choices for the rename modal.
// The preview strings use forward slashes and are relative to downloadDir.
func buildRenameOptions(ircFilename string, meta *core.EPUBMetadata, replaceSpace string) []RenameOption {
	return staging.BuildOptions(ircFilename, meta, replaceSpace)
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
	return staging.ResolveFinalPath(downloadDir, choice, ircFilename, meta, replaceSpace)
}

// moveFile moves src to dst, creating parent directories as needed.
// Falls back to copy-and-delete when os.Rename fails (e.g. cross-device).
func moveFile(src, dst string) error {
	return staging.MoveFile(src, dst)
}

// copyFile copies src to dst, creating parent directories as needed.
func copyFile(src, dst string) error {
	return staging.CopyFile(src, dst)
}

func originalCopyPath(path string) string {
	return staging.OriginalCopyPath(path)
}

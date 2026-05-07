package server

import (
	"fmt"
	"log"
	"log/slog"
	"os"
	"os/exec"
)

// validatePostProcessCmd checks that the configured command binary exists in PATH.
// Logs a prominent warning if not found so the problem is obvious at startup.
func validatePostProcessCmd(cmd []string, logger *log.Logger) {
	if len(cmd) == 0 {
		return
	}
	bin := cmd[0]
	path, err := exec.LookPath(bin)
	if err != nil {
		logger.Printf("WARNING: post-process command %q not found in PATH — downloads will succeed but post-processing will fail. Check your --post-process-cmd flag and that the binary is installed.", bin)
	} else {
		logger.Printf("Post-process command verified: %s", path)
	}
}

// runPostProcess executes the configured post-process command with filePath
// appended as the final argument. Runs synchronously; errors are logged, not fatal.
func runPostProcess(cmd []string, filePath string, lb bookLogger) {
	if len(cmd) == 0 {
		return
	}

	var sizeBefore int64
	if info, err := os.Stat(filePath); err == nil {
		sizeBefore = info.Size()
	}

	args := append(append([]string{}, cmd[1:]...), filePath)
	out, err := exec.Command(cmd[0], args...).CombinedOutput()
	detail := string(out)
	if err != nil {
		slog.Error("post-process command failed", "cmd", cmd[0], "file", filePath,
			"err", err, "output", detail)
		lb.errorDetail(fmt.Sprintf("❌ Cleaning failed: %s", cmd[0]), detail)
		return
	}

	slog.Info("post-process complete", "cmd", cmd[0], "file", filePath)
	msg := fmt.Sprintf("✨ Cleaned: %s", cmd[0])
	if sizeBefore > 0 {
		if info, err := os.Stat(filePath); err == nil {
			sizeAfter := info.Size()
			delta := sizeAfter - sizeBefore
			pct := float64(delta) / float64(sizeBefore) * 100
			sign := "+"
			if delta <= 0 {
				sign = ""
			}
			msg += fmt.Sprintf(" (%s%.1f%%, %s → %s)",
				sign, pct,
				formatBytes(sizeBefore), formatBytes(sizeAfter))
		}
	}
	lb.infoDetail(msg, detail)
}

func formatBytes(b int64) string {
	const unit = 1024
	if b < unit {
		return fmt.Sprintf("%d B", b)
	}
	div, exp := int64(unit), 0
	for n := b / unit; n >= unit; n /= unit {
		div *= unit
		exp++
	}
	return fmt.Sprintf("%.1f %cB", float64(b)/float64(div), "KMGTPE"[exp])
}

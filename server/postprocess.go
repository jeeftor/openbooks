package server

import (
	"fmt"
	"log"
	"log/slog"
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
func runPostProcess(cmd []string, filePath string, lb *logBuffer) {
	if len(cmd) == 0 {
		return
	}
	args := append(append([]string{}, cmd[1:]...), filePath)
	out, err := exec.Command(cmd[0], args...).CombinedOutput()
	if err != nil {
		slog.Error("post-process command failed", "cmd", cmd[0], "file", filePath,
			"err", err, "output", string(out))
		lb.error(fmt.Sprintf("Post-process failed (%s): %v", cmd[0], err))
		return
	}
	slog.Info("post-process complete", "cmd", cmd[0], "file", filePath)
	lb.info(fmt.Sprintf("Post-process complete: %s", cmd[0]))
}

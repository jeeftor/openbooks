package server

import (
	"fmt"
	"log/slog"
	"os/exec"
)

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

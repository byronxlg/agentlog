package cli

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"syscall"
	"time"
)

// Stop sends SIGTERM to the running agentlogd daemon and waits for it to exit.
func Stop(dir string) error {
	pidPath := filepath.Join(dir, "agentlogd.pid")

	pid, err := ReadPID(pidPath)
	if err != nil {
		if errors.Is(err, os.ErrNotExist) {
			return fmt.Errorf("daemon is not running")
		}
		return err
	}

	if !IsProcessRunning(pid) {
		_ = CleanStalePID(pidPath)
		return fmt.Errorf("daemon is not running (stale PID file cleaned)")
	}

	proc, err := os.FindProcess(pid)
	if err != nil {
		return fmt.Errorf("find process %d: %w", pid, err)
	}

	if err := proc.Signal(syscall.SIGTERM); err != nil {
		return fmt.Errorf("send SIGTERM to %d: %w", pid, err)
	}

	if err := WaitForExit(pid, 5*time.Second); err != nil {
		return fmt.Errorf("daemon did not stop within timeout")
	}

	_, _ = fmt.Fprintln(os.Stdout, "agentlogd stopped")
	return nil
}

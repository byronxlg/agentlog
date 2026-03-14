package cli

import (
	"fmt"
	"os"
	"strconv"
	"syscall"
	"time"
)

// ReadPID reads and parses the PID from the given file path.
func ReadPID(pidPath string) (int, error) {
	data, err := os.ReadFile(pidPath)
	if err != nil {
		return 0, fmt.Errorf("read pid file: %w", err)
	}

	pid, err := strconv.Atoi(string(data))
	if err != nil {
		return 0, fmt.Errorf("parse pid file: %w", err)
	}

	return pid, nil
}

// IsProcessRunning checks whether a process with the given PID exists.
func IsProcessRunning(pid int) bool {
	proc, err := os.FindProcess(pid)
	if err != nil {
		return false
	}
	return proc.Signal(syscall.Signal(0)) == nil
}

// IsStale returns true if the PID file exists but the process is not running.
func IsStale(pidPath string) bool {
	pid, err := ReadPID(pidPath)
	if err != nil {
		return false
	}
	return !IsProcessRunning(pid)
}

// CleanStalePID removes the PID file and its associated socket file.
func CleanStalePID(pidPath string) error {
	if err := os.Remove(pidPath); err != nil && !os.IsNotExist(err) {
		return fmt.Errorf("remove stale pid file: %w", err)
	}

	sockPath := pidPath[:len(pidPath)-len(".pid")] + ".sock"
	if err := os.Remove(sockPath); err != nil && !os.IsNotExist(err) {
		return fmt.Errorf("remove stale socket: %w", err)
	}

	return nil
}

// WaitForExit polls until the process exits or the timeout is reached.
func WaitForExit(pid int, timeout time.Duration) error {
	deadline := time.Now().Add(timeout)
	for time.Now().Before(deadline) {
		if !IsProcessRunning(pid) {
			return nil
		}
		time.Sleep(50 * time.Millisecond)
	}
	return fmt.Errorf("process %d did not exit within %s", pid, timeout)
}

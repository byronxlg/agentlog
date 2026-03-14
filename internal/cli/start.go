package cli

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"syscall"
	"time"
)

// Start launches the agentlogd daemon as a background process.
func Start(dir string) error {
	pidPath := filepath.Join(dir, "agentlogd.pid")

	if _, err := os.Stat(pidPath); err == nil {
		if IsStale(pidPath) {
			if err := CleanStalePID(pidPath); err != nil {
				return fmt.Errorf("clean stale pid: %w", err)
			}
		} else {
			pid, _ := ReadPID(pidPath)
			return fmt.Errorf("daemon already running (PID %d)", pid)
		}
	}

	daemonBin, err := findDaemonBinary()
	if err != nil {
		return err
	}

	cmd := exec.Command(daemonBin, "--dir", dir)
	cmd.SysProcAttr = &syscall.SysProcAttr{Setsid: true}
	cmd.Stdout = nil
	cmd.Stderr = nil

	if err := cmd.Start(); err != nil {
		return fmt.Errorf("start daemon: %w", err)
	}

	// Detach so the daemon survives this process exiting
	if err := cmd.Process.Release(); err != nil {
		return fmt.Errorf("release daemon process: %w", err)
	}

	pid, err := waitForPIDFile(pidPath, 2*time.Second)
	if err != nil {
		return fmt.Errorf("daemon failed to start: %w", err)
	}

	_, _ = fmt.Fprintf(os.Stdout, "agentlogd started (PID %d)\n", pid)
	return nil
}

func findDaemonBinary() (string, error) {
	exe, err := os.Executable()
	if err != nil {
		return "", fmt.Errorf("resolve executable path: %w", err)
	}

	exe, err = filepath.EvalSymlinks(exe)
	if err != nil {
		return "", fmt.Errorf("resolve symlinks: %w", err)
	}

	candidate := filepath.Join(filepath.Dir(exe), "agentlogd")
	if _, err := os.Stat(candidate); err == nil {
		return candidate, nil
	}

	// Fall back to PATH lookup
	path, err := exec.LookPath("agentlogd")
	if err != nil {
		return "", fmt.Errorf("agentlogd binary not found (looked in %s and PATH)", filepath.Dir(exe))
	}
	return path, nil
}

func waitForPIDFile(pidPath string, timeout time.Duration) (int, error) {
	deadline := time.Now().Add(timeout)
	for time.Now().Before(deadline) {
		pid, err := ReadPID(pidPath)
		if err == nil && IsProcessRunning(pid) {
			return pid, nil
		}
		time.Sleep(50 * time.Millisecond)
	}
	return 0, fmt.Errorf("pid file not created within %s", timeout)
}

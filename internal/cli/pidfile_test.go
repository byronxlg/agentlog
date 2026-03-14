package cli

import (
	"os"
	"path/filepath"
	"strconv"
	"testing"
	"time"
)

func TestReadPID_ValidFile(t *testing.T) {
	dir := t.TempDir()
	pidPath := filepath.Join(dir, "test.pid")

	if err := os.WriteFile(pidPath, []byte("12345"), 0o644); err != nil {
		t.Fatal(err)
	}

	pid, err := ReadPID(pidPath)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if pid != 12345 {
		t.Fatalf("got pid %d, want 12345", pid)
	}
}

func TestReadPID_MissingFile(t *testing.T) {
	dir := t.TempDir()
	pidPath := filepath.Join(dir, "missing.pid")

	_, err := ReadPID(pidPath)
	if err == nil {
		t.Fatal("expected error for missing file")
	}
}

func TestReadPID_InvalidContent(t *testing.T) {
	dir := t.TempDir()
	pidPath := filepath.Join(dir, "test.pid")

	if err := os.WriteFile(pidPath, []byte("notanumber"), 0o644); err != nil {
		t.Fatal(err)
	}

	_, err := ReadPID(pidPath)
	if err == nil {
		t.Fatal("expected error for invalid content")
	}
}

func TestIsProcessRunning_CurrentProcess(t *testing.T) {
	if !IsProcessRunning(os.Getpid()) {
		t.Fatal("current process should be running")
	}
}

func TestIsProcessRunning_NonExistentPID(t *testing.T) {
	// PID 99999999 is extremely unlikely to exist
	if IsProcessRunning(99999999) {
		t.Fatal("non-existent PID should not be running")
	}
}

func TestIsStale_NoFile(t *testing.T) {
	dir := t.TempDir()
	pidPath := filepath.Join(dir, "missing.pid")

	if IsStale(pidPath) {
		t.Fatal("missing file should not be considered stale")
	}
}

func TestIsStale_RunningProcess(t *testing.T) {
	dir := t.TempDir()
	pidPath := filepath.Join(dir, "test.pid")

	pid := os.Getpid()
	if err := os.WriteFile(pidPath, []byte(strconv.Itoa(pid)), 0o644); err != nil {
		t.Fatal(err)
	}

	if IsStale(pidPath) {
		t.Fatal("PID file for running process should not be stale")
	}
}

func TestIsStale_DeadProcess(t *testing.T) {
	dir := t.TempDir()
	pidPath := filepath.Join(dir, "test.pid")

	if err := os.WriteFile(pidPath, []byte("99999999"), 0o644); err != nil {
		t.Fatal(err)
	}

	if !IsStale(pidPath) {
		t.Fatal("PID file for dead process should be stale")
	}
}

func TestCleanStalePID_RemovesFiles(t *testing.T) {
	dir := t.TempDir()
	pidPath := filepath.Join(dir, "agentlogd.pid")
	sockPath := filepath.Join(dir, "agentlogd.sock")

	if err := os.WriteFile(pidPath, []byte("12345"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(sockPath, []byte(""), 0o644); err != nil {
		t.Fatal(err)
	}

	if err := CleanStalePID(pidPath); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if _, err := os.Stat(pidPath); !os.IsNotExist(err) {
		t.Fatal("pid file should be removed")
	}
	if _, err := os.Stat(sockPath); !os.IsNotExist(err) {
		t.Fatal("socket file should be removed")
	}
}

func TestCleanStalePID_MissingFiles(t *testing.T) {
	dir := t.TempDir()
	pidPath := filepath.Join(dir, "agentlogd.pid")

	if err := CleanStalePID(pidPath); err != nil {
		t.Fatalf("should not error on missing files: %v", err)
	}
}

func TestWaitForExit_AlreadyExited(t *testing.T) {
	// Non-existent process should return immediately
	if err := WaitForExit(99999999, 100*time.Millisecond); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}

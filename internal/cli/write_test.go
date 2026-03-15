package cli

import (
	"bytes"
	"context"
	"net"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/byronxlg/agentlog/internal/daemon"
)

func TestParseWriteArgs_AllFlags(t *testing.T) {
	args := []string{
		"--type", "decision",
		"--title", "use SQLite",
		"--body", "lightweight and embedded",
		"--tags", "database,storage",
		"--files", "internal/store/db.go,go.mod",
		"--session", "sess-123",
	}

	opts, err := ParseWriteArgs("/tmp/agentlog", args)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if opts.Dir != "/tmp/agentlog" {
		t.Fatalf("Dir = %q, want %q", opts.Dir, "/tmp/agentlog")
	}
	if opts.Type != "decision" {
		t.Fatalf("Type = %q, want %q", opts.Type, "decision")
	}
	if opts.Title != "use SQLite" {
		t.Fatalf("Title = %q, want %q", opts.Title, "use SQLite")
	}
	if opts.Body != "lightweight and embedded" {
		t.Fatalf("Body = %q, want %q", opts.Body, "lightweight and embedded")
	}
	if opts.Tags != "database,storage" {
		t.Fatalf("Tags = %q, want %q", opts.Tags, "database,storage")
	}
	if opts.Files != "internal/store/db.go,go.mod" {
		t.Fatalf("Files = %q, want %q", opts.Files, "internal/store/db.go,go.mod")
	}
	if opts.Session != "sess-123" {
		t.Fatalf("Session = %q, want %q", opts.Session, "sess-123")
	}
}

func TestParseWriteArgs_RequiredOnly(t *testing.T) {
	args := []string{"--type", "question", "--title", "should we use gRPC?"}

	opts, err := ParseWriteArgs("/tmp/agentlog", args)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if opts.Type != "question" {
		t.Fatalf("Type = %q, want %q", opts.Type, "question")
	}
	if opts.Title != "should we use gRPC?" {
		t.Fatalf("Title = %q, want %q", opts.Title, "should we use gRPC?")
	}
	if opts.Body != "" {
		t.Fatalf("Body = %q, want empty", opts.Body)
	}
	if opts.Tags != "" {
		t.Fatalf("Tags = %q, want empty", opts.Tags)
	}
	if opts.Files != "" {
		t.Fatalf("Files = %q, want empty", opts.Files)
	}
	if opts.Session != "" {
		t.Fatalf("Session = %q, want empty", opts.Session)
	}
}

func TestParseWriteArgs_MissingType(t *testing.T) {
	args := []string{"--title", "some title"}

	_, err := ParseWriteArgs("/tmp/agentlog", args)
	if err == nil {
		t.Fatal("expected error for missing --type")
	}
	if err.Error() != "--type is required" {
		t.Fatalf("error = %q, want %q", err.Error(), "--type is required")
	}
}

func TestParseWriteArgs_MissingTitle(t *testing.T) {
	args := []string{"--type", "decision"}

	_, err := ParseWriteArgs("/tmp/agentlog", args)
	if err == nil {
		t.Fatal("expected error for missing --title")
	}
	if err.Error() != "--title is required" {
		t.Fatalf("error = %q, want %q", err.Error(), "--title is required")
	}
}

func TestParseWriteArgs_InvalidType(t *testing.T) {
	args := []string{"--type", "invalid", "--title", "some title"}

	_, err := ParseWriteArgs("/tmp/agentlog", args)
	if err == nil {
		t.Fatal("expected error for invalid type")
	}
	want := `invalid type "invalid": must be one of decision, attempt_failed, deferred, assumption, question`
	if err.Error() != want {
		t.Fatalf("error = %q, want %q", err.Error(), want)
	}
}

func TestParseWriteArgs_AllValidTypes(t *testing.T) {
	types := []string{"decision", "attempt_failed", "deferred", "assumption", "question"}
	for _, typ := range types {
		args := []string{"--type", typ, "--title", "test"}
		opts, err := ParseWriteArgs("/tmp/agentlog", args)
		if err != nil {
			t.Fatalf("type %q: unexpected error: %v", typ, err)
		}
		if opts.Type != typ {
			t.Fatalf("type %q: got %q", typ, opts.Type)
		}
	}
}

func TestSplitCSV_MultipleValues(t *testing.T) {
	got := splitCSV("foo,bar,baz")
	if len(got) != 3 {
		t.Fatalf("len = %d, want 3", len(got))
	}
	if got[0] != "foo" || got[1] != "bar" || got[2] != "baz" {
		t.Fatalf("got %v, want [foo bar baz]", got)
	}
}

func TestSplitCSV_WithSpaces(t *testing.T) {
	got := splitCSV(" foo , bar , baz ")
	if len(got) != 3 {
		t.Fatalf("len = %d, want 3", len(got))
	}
	if got[0] != "foo" || got[1] != "bar" || got[2] != "baz" {
		t.Fatalf("got %v, want [foo bar baz]", got)
	}
}

func TestSplitCSV_Empty(t *testing.T) {
	got := splitCSV("")
	if got != nil {
		t.Fatalf("got %v, want nil", got)
	}
}

func TestSplitCSV_SingleValue(t *testing.T) {
	got := splitCSV("solo")
	if len(got) != 1 || got[0] != "solo" {
		t.Fatalf("got %v, want [solo]", got)
	}
}

// shortTempDir creates a temp directory with a short path to stay under
// macOS's 104-byte Unix socket path limit.
func shortTempDir(t *testing.T) string {
	t.Helper()
	dir, err := os.MkdirTemp("", "al")
	if err != nil {
		t.Fatalf("create temp dir: %v", err)
	}
	t.Cleanup(func() { os.RemoveAll(dir) })
	return dir
}

// startTestDaemon creates and starts a daemon in the given directory.
// The socket is placed at agentlogd.sock to match the CLI's expectation.
// Returns a cancel function to shut down the daemon.
func startTestDaemon(t *testing.T, dir string) context.CancelFunc {
	t.Helper()
	socketPath := filepath.Join(dir, "agentlogd.sock")
	cfg := daemon.Config{
		Dir:        dir,
		SocketPath: socketPath,
		PIDPath:    filepath.Join(dir, "test.pid"),
		LogPath:    filepath.Join(dir, "test.log"),
	}
	d, err := daemon.New(cfg)
	if err != nil {
		t.Fatalf("create daemon: %v", err)
	}

	ctx, cancel := context.WithCancel(context.Background())
	errCh := make(chan error, 1)
	go func() { errCh <- d.Run(ctx) }()

	// Wait for socket to be available.
	for i := 0; i < 50; i++ {
		conn, dialErr := net.Dial("unix", socketPath)
		if dialErr == nil {
			_ = conn.Close()
			break
		}
		if i == 49 {
			cancel()
			t.Fatalf("daemon did not start: %v", dialErr)
		}
		time.Sleep(10 * time.Millisecond)
	}

	t.Cleanup(func() {
		cancel()
		<-errCh
	})
	return cancel
}

func TestWrite_AutoSessionCreation(t *testing.T) {
	dir := shortTempDir(t)
	startTestDaemon(t, dir)

	// Capture stderr to verify session ID is printed.
	origStderr := os.Stderr
	r, w, err := os.Pipe()
	if err != nil {
		t.Fatalf("create pipe: %v", err)
	}
	os.Stderr = w

	opts := WriteOptions{
		Dir:   dir,
		Type:  "decision",
		Title: "test auto-session",
	}
	writeErr := Write(opts)

	_ = w.Close()
	os.Stderr = origStderr

	if writeErr != nil {
		t.Fatalf("Write() error: %v", writeErr)
	}

	var buf bytes.Buffer
	_, _ = buf.ReadFrom(r)
	sessionID := strings.TrimSpace(buf.String())
	if sessionID == "" {
		t.Fatal("expected session ID on stderr, got empty")
	}
}

func TestWrite_ExplicitSessionUnchanged(t *testing.T) {
	dir := shortTempDir(t)
	startTestDaemon(t, dir)

	// Create a session first.
	socketPath := filepath.Join(dir, "agentlogd.sock")
	sid, err := createSession(socketPath)
	if err != nil {
		t.Fatalf("createSession: %v", err)
	}

	// Capture stderr - should be empty when session is explicit.
	origStderr := os.Stderr
	r, w, err := os.Pipe()
	if err != nil {
		t.Fatalf("create pipe: %v", err)
	}
	os.Stderr = w

	opts := WriteOptions{
		Dir:     dir,
		Type:    "decision",
		Title:   "test explicit session",
		Session: sid,
	}
	writeErr := Write(opts)

	_ = w.Close()
	os.Stderr = origStderr

	if writeErr != nil {
		t.Fatalf("Write() error: %v", writeErr)
	}

	var buf bytes.Buffer
	_, _ = buf.ReadFrom(r)
	stderrOutput := buf.String()
	if stderrOutput != "" {
		t.Fatalf("expected no stderr output for explicit session, got %q", stderrOutput)
	}
}

package cli

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/byronxlg/agentlog/internal/daemon"
	"github.com/byronxlg/agentlog/internal/store"
)

func TestParseContextArgs_FilesOnly(t *testing.T) {
	args := []string{"--files", "foo.go", "--files", "bar.go"}
	opts, err := ParseContextArgs("/tmp/test", args)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(opts.Files) != 2 || opts.Files[0] != "foo.go" || opts.Files[1] != "bar.go" {
		t.Fatalf("Files = %v, want [foo.go bar.go]", opts.Files)
	}
	if opts.Topic != "" {
		t.Fatalf("Topic = %q, want empty", opts.Topic)
	}
	if opts.Limit != 10 {
		t.Fatalf("Limit = %d, want 10", opts.Limit)
	}
	if opts.JSON {
		t.Fatal("JSON should default to false")
	}
}

func TestParseContextArgs_TopicOnly(t *testing.T) {
	args := []string{"--topic", "connection pooling"}
	opts, err := ParseContextArgs("/tmp/test", args)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if opts.Topic != "connection pooling" {
		t.Fatalf("Topic = %q, want %q", opts.Topic, "connection pooling")
	}
	if len(opts.Files) != 0 {
		t.Fatalf("Files = %v, want empty", opts.Files)
	}
}

func TestParseContextArgs_AllFlags(t *testing.T) {
	args := []string{"--files", "foo.go", "--topic", "auth", "--limit", "5", "--json"}
	opts, err := ParseContextArgs("/tmp/test", args)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(opts.Files) != 1 || opts.Files[0] != "foo.go" {
		t.Fatalf("Files = %v, want [foo.go]", opts.Files)
	}
	if opts.Topic != "auth" {
		t.Fatalf("Topic = %q, want auth", opts.Topic)
	}
	if opts.Limit != 5 {
		t.Fatalf("Limit = %d, want 5", opts.Limit)
	}
	if !opts.JSON {
		t.Fatal("JSON should be true")
	}
}

func TestParseContextArgs_NeitherFilesNorTopic(t *testing.T) {
	_, err := ParseContextArgs("/tmp/test", nil)
	if err == nil {
		t.Fatal("expected error for neither files nor topic")
	}
}

func TestParseContextArgs_NegativeLimit(t *testing.T) {
	_, err := ParseContextArgs("/tmp/test", []string{"--topic", "x", "--limit", "-1"})
	if err == nil {
		t.Fatal("expected error for negative limit")
	}
}

func TestFormatContext_Empty(t *testing.T) {
	got := FormatContext(nil)
	if got != "No relevant decisions found." {
		t.Fatalf("got %q", got)
	}
}

func TestFormatContext_WithEntries(t *testing.T) {
	entries := []store.Entry{
		{
			Type:      store.EntryTypeDecision,
			Title:     "Use Redis",
			Timestamp: time.Date(2026, 3, 15, 10, 0, 0, 0, time.UTC),
			Body:      "Fast caching layer.",
			FileRefs:  []string{"config/redis.yaml"},
			Tags:      []string{"infrastructure"},
		},
	}
	got := FormatContext(entries)
	if got == "" {
		t.Fatal("expected non-empty output")
	}
	if !strings.Contains(got, "# Relevant decisions") {
		t.Error("missing header")
	}
	if !strings.Contains(got, "[decision] Use Redis") {
		t.Error("missing entry header")
	}
	if !strings.Contains(got, "Fast caching layer.") {
		t.Error("missing body")
	}
	if !strings.Contains(got, "Files: config/redis.yaml") {
		t.Error("missing file refs")
	}
	if !strings.Contains(got, "Tags: infrastructure") {
		t.Error("missing tags")
	}
}

func TestContext_Integration(t *testing.T) {
	dir := shortTempDir(t)
	startTestDaemon(t, dir)

	socketPath := filepath.Join(dir, "agentlogd.sock")

	// Create a session.
	sid, err := createSession(socketPath)
	if err != nil {
		t.Fatalf("createSession: %v", err)
	}

	// Write entries with file_refs.
	for _, e := range []daemon.EntryParams{
		{SessionID: sid, Type: "decision", Title: "Use Redis for caching", Body: "Sub-ms reads.", FileRefs: []string{"config/redis.yaml"}, Tags: []string{"infra"}},
		{SessionID: sid, Type: "decision", Title: "Use PostgreSQL for persistence", Body: "ACID compliance.", FileRefs: []string{"config/db.yaml"}, Tags: []string{"infra"}},
		{SessionID: sid, Type: "assumption", Title: "Auth tokens expire in 1h", Body: "Token TTL assumption.", FileRefs: []string{"internal/auth/token.go"}, Tags: []string{"auth"}},
	} {
		params, _ := json.Marshal(daemon.WriteParams{Entry: e})
		req := daemon.Request{Method: "write", Params: params}
		resp, connErr := SendRequest(socketPath, req)
		if connErr != nil {
			t.Fatalf("write: %v", connErr)
		}
		if !resp.OK {
			t.Fatalf("write failed: %s", resp.Error)
		}
	}

	// Test context by files.
	opts := ContextOptions{Dir: dir, Files: []string{"config/redis.yaml"}, Limit: 10}
	origStdout := os.Stdout
	r, w, _ := os.Pipe()
	os.Stdout = w
	err = Context(opts)
	_ = w.Close()
	os.Stdout = origStdout
	if err != nil {
		t.Fatalf("Context by files: %v", err)
	}
	var buf [4096]byte
	n, _ := r.Read(buf[:])
	output := string(buf[:n])
	if !strings.Contains(output, "Use Redis") {
		t.Errorf("expected output to contain 'Use Redis', got: %s", output)
	}

	// Test context by topic.
	opts = ContextOptions{Dir: dir, Topic: "PostgreSQL", Limit: 10}
	r2, w2, _ := os.Pipe()
	os.Stdout = w2
	err = Context(opts)
	_ = w2.Close()
	os.Stdout = origStdout
	if err != nil {
		t.Fatalf("Context by topic: %v", err)
	}
	n, _ = r2.Read(buf[:])
	output = string(buf[:n])
	if !strings.Contains(output, "PostgreSQL") {
		t.Errorf("expected output to contain 'PostgreSQL', got: %s", output)
	}
}

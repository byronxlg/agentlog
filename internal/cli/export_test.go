package cli

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/byronxlg/agentlog/internal/daemon"
)

func TestParseExportArgs_Defaults(t *testing.T) {
	opts, err := ParseExportArgs("/tmp/test", nil)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if opts.Dir != "/tmp/test" {
		t.Fatalf("Dir = %q, want /tmp/test", opts.Dir)
	}
	if opts.Format != "markdown" {
		t.Fatalf("Format = %q, want markdown", opts.Format)
	}
	if opts.Template != "" {
		t.Fatalf("Template = %q, want empty", opts.Template)
	}
}

func TestParseExportArgs_AllFlags(t *testing.T) {
	args := []string{
		"--session", "sess-123",
		"--since", "7d",
		"--until", "1h",
		"--file", "main.go",
		"--tag", "architecture",
		"--type", "decision",
		"--format", "json",
		"--template", "pr",
	}
	opts, err := ParseExportArgs("/tmp/test", args)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if opts.SessionID != "sess-123" {
		t.Fatalf("SessionID = %q", opts.SessionID)
	}
	if opts.Since != "7d" {
		t.Fatalf("Since = %q", opts.Since)
	}
	if opts.Until != "1h" {
		t.Fatalf("Until = %q", opts.Until)
	}
	if opts.FilePath != "main.go" {
		t.Fatalf("FilePath = %q", opts.FilePath)
	}
	if opts.Tag != "architecture" {
		t.Fatalf("Tag = %q", opts.Tag)
	}
	if opts.Type != "decision" {
		t.Fatalf("Type = %q", opts.Type)
	}
	if opts.Format != "json" {
		t.Fatalf("Format = %q", opts.Format)
	}
	if opts.Template != "pr" {
		t.Fatalf("Template = %q", opts.Template)
	}
}

func TestParseExportArgs_InvalidFormat(t *testing.T) {
	_, err := ParseExportArgs("/tmp/test", []string{"--format", "csv"})
	if err == nil {
		t.Fatal("expected error for invalid format")
	}
	if !strings.Contains(err.Error(), "invalid format") {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestParseExportArgs_InvalidTemplate(t *testing.T) {
	_, err := ParseExportArgs("/tmp/test", []string{"--template", "newsletter"})
	if err == nil {
		t.Fatal("expected error for invalid template")
	}
	if !strings.Contains(err.Error(), "invalid template") {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestParseExportArgs_ValidFormats(t *testing.T) {
	for _, f := range []string{"markdown", "json", "text"} {
		_, err := ParseExportArgs("/tmp/test", []string{"--format", f})
		if err != nil {
			t.Fatalf("format %q: unexpected error: %v", f, err)
		}
	}
}

func TestParseExportArgs_ValidTemplates(t *testing.T) {
	for _, tmpl := range []string{"pr", "retro", "handoff"} {
		_, err := ParseExportArgs("/tmp/test", []string{"--template", tmpl})
		if err != nil {
			t.Fatalf("template %q: unexpected error: %v", tmpl, err)
		}
	}
}

func TestExport_Integration(t *testing.T) {
	dir := shortTempDir(t)
	startTestDaemon(t, dir)

	socketPath := filepath.Join(dir, "agentlogd.sock")

	// Create a session.
	sid, err := createSession(socketPath)
	if err != nil {
		t.Fatalf("createSession: %v", err)
	}

	// Write entries.
	for _, e := range []daemon.EntryParams{
		{SessionID: sid, Type: "decision", Title: "Use JSONL format", Body: "Simple and appendable.", Tags: []string{"storage"}, FileRefs: []string{"internal/store/store.go"}},
		{SessionID: sid, Type: "assumption", Title: "Entries are small", Body: "Under 1KB each.", Tags: []string{"performance"}},
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

	// Test export with markdown format (default).
	origStdout := os.Stdout
	r, w, _ := os.Pipe()
	os.Stdout = w
	err = Export(ExportOptions{Dir: dir})
	_ = w.Close()
	os.Stdout = origStdout
	if err != nil {
		t.Fatalf("Export: %v", err)
	}

	var buf [8192]byte
	n, _ := r.Read(buf[:])
	output := string(buf[:n])

	if !strings.Contains(output, "# Decision Log Export") {
		t.Error("missing markdown header in export output")
	}
	if !strings.Contains(output, "Use JSONL format") {
		t.Error("missing entry in export output")
	}

	// Test export with JSON format.
	r2, w2, _ := os.Pipe()
	os.Stdout = w2
	err = Export(ExportOptions{Dir: dir, Format: "json"})
	_ = w2.Close()
	os.Stdout = origStdout
	if err != nil {
		t.Fatalf("Export json: %v", err)
	}

	n, _ = r2.Read(buf[:])
	output = string(buf[:n])
	if !strings.HasPrefix(strings.TrimSpace(output), "[") {
		t.Error("JSON export should start with [")
	}

	// Test export with template.
	r3, w3, _ := os.Pipe()
	os.Stdout = w3
	err = Export(ExportOptions{Dir: dir, Template: "pr"})
	_ = w3.Close()
	os.Stdout = origStdout
	if err != nil {
		t.Fatalf("Export template: %v", err)
	}

	n, _ = r3.Read(buf[:])
	output = string(buf[:n])
	if !strings.Contains(output, "## What changed") {
		t.Error("missing PR template header")
	}
}

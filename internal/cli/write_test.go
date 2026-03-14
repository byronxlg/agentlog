package cli

import (
	"testing"
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

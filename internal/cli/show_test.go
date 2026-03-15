package cli

import (
	"strings"
	"testing"
	"time"

	"github.com/byronxlg/agentlog/internal/store"
)

func TestMatchSession_ExactMatch(t *testing.T) {
	sessions := []string{"abc123", "abc456", "def789"}
	got, err := matchSession("abc123", sessions)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if got != "abc123" {
		t.Errorf("got %q, want %q", got, "abc123")
	}
}

func TestMatchSession_PrefixMatch(t *testing.T) {
	sessions := []string{"abc123-full-id", "def456-full-id"}
	got, err := matchSession("abc", sessions)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if got != "abc123-full-id" {
		t.Errorf("got %q, want %q", got, "abc123-full-id")
	}
}

func TestMatchSession_NoMatch(t *testing.T) {
	sessions := []string{"abc123", "def456"}
	_, err := matchSession("xyz", sessions)
	if err == nil {
		t.Fatal("expected error, got nil")
	}
	if !strings.Contains(err.Error(), "no session found") {
		t.Errorf("expected 'no session found' error, got: %v", err)
	}
}

func TestMatchSession_Ambiguous(t *testing.T) {
	sessions := []string{"abc123", "abc456", "def789"}
	_, err := matchSession("abc", sessions)
	if err == nil {
		t.Fatal("expected error, got nil")
	}
	if !strings.Contains(err.Error(), "ambiguous") {
		t.Errorf("expected 'ambiguous' error, got: %v", err)
	}
	if !strings.Contains(err.Error(), "abc123") || !strings.Contains(err.Error(), "abc456") {
		t.Errorf("expected error to list matching sessions, got: %v", err)
	}
}

func TestMatchSession_EmptySessions(t *testing.T) {
	_, err := matchSession("abc", nil)
	if err == nil {
		t.Fatal("expected error, got nil")
	}
	if !strings.Contains(err.Error(), "no session found") {
		t.Errorf("expected 'no session found' error, got: %v", err)
	}
}

func TestFormatEntryDetail_AllFields(t *testing.T) {
	entry := store.Entry{
		ID:        "entry-1",
		Timestamp: time.Date(2026, 3, 15, 10, 30, 0, 0, time.UTC),
		SessionID: "session-abc123",
		Type:      store.EntryTypeDecision,
		Title:     "Use SQLite for indexing",
		Body:      "SQLite provides good performance\nfor our use case.",
		Tags:      []string{"architecture", "storage"},
		FileRefs:  []string{"internal/store/db.go", "internal/store/index.go"},
	}

	got := FormatEntryDetail(entry)

	if !strings.Contains(got, "2026-03-15 10:30:00") {
		t.Errorf("missing timestamp in output: %s", got)
	}
	if !strings.Contains(got, "[decision]") {
		t.Errorf("missing type in output: %s", got)
	}
	if !strings.Contains(got, "Use SQLite for indexing") {
		t.Errorf("missing title in output: %s", got)
	}
	if !strings.Contains(got, "SQLite provides good performance") {
		t.Errorf("missing body in output: %s", got)
	}
	if !strings.Contains(got, "Tags: architecture, storage") {
		t.Errorf("missing tags in output: %s", got)
	}
	if !strings.Contains(got, "Files: internal/store/db.go, internal/store/index.go") {
		t.Errorf("missing file refs in output: %s", got)
	}
}

func TestFormatEntryDetail_MinimalFields(t *testing.T) {
	entry := store.Entry{
		ID:        "entry-2",
		Timestamp: time.Date(2026, 1, 1, 0, 0, 0, 0, time.UTC),
		SessionID: "session-xyz",
		Type:      store.EntryTypeQuestion,
		Title:     "Should we use GRPC?",
	}

	got := FormatEntryDetail(entry)

	if !strings.Contains(got, "2026-01-01 00:00:00") {
		t.Errorf("missing timestamp in output: %s", got)
	}
	if !strings.Contains(got, "[question]") {
		t.Errorf("missing type in output: %s", got)
	}
	if !strings.Contains(got, "Should we use GRPC?") {
		t.Errorf("missing title in output: %s", got)
	}
	if strings.Contains(got, "Tags:") {
		t.Errorf("should not contain Tags when empty: %s", got)
	}
	if strings.Contains(got, "Files:") {
		t.Errorf("should not contain Files when empty: %s", got)
	}
}

func TestFormatEntryDetail_BodyOnly(t *testing.T) {
	entry := store.Entry{
		ID:        "entry-3",
		Timestamp: time.Date(2026, 6, 15, 12, 0, 0, 0, time.UTC),
		SessionID: "session-123",
		Type:      store.EntryTypeAssumption,
		Title:     "Config file format",
		Body:      "Assuming TOML is sufficient",
	}

	got := FormatEntryDetail(entry)

	if !strings.Contains(got, "  Assuming TOML is sufficient") {
		t.Errorf("body should be indented: %s", got)
	}
	if strings.Contains(got, "Tags:") {
		t.Errorf("should not contain Tags when empty: %s", got)
	}
}

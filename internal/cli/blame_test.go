package cli

import (
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/byronxlg/agentlog/internal/store"
)

func TestShortSession_FullLength(t *testing.T) {
	got := ShortSession("abcdef1234567890")
	if got != "abcdef12" {
		t.Fatalf("got %q, want %q", got, "abcdef12")
	}
}

func TestShortSession_ExactlyEight(t *testing.T) {
	got := ShortSession("abcdef12")
	if got != "abcdef12" {
		t.Fatalf("got %q, want %q", got, "abcdef12")
	}
}

func TestShortSession_ShorterThanEight(t *testing.T) {
	got := ShortSession("abc")
	if got != "abc" {
		t.Fatalf("got %q, want %q", got, "abc")
	}
}

func TestShortSession_Empty(t *testing.T) {
	got := ShortSession("")
	if got != "" {
		t.Fatalf("got %q, want empty string", got)
	}
}

func TestFormatTimestamp(t *testing.T) {
	ts := time.Date(2026, 3, 15, 14, 30, 45, 0, time.UTC)
	got := FormatTimestamp(ts)
	want := "2026-03-15 14:30:45"
	if got != want {
		t.Fatalf("got %q, want %q", got, want)
	}
}

func TestFormatEntry_WithoutVerbose(t *testing.T) {
	entry := store.Entry{
		Timestamp: time.Date(2026, 3, 15, 14, 30, 45, 0, time.UTC),
		Type:      store.EntryTypeDecision,
		SessionID: "abcdef1234567890",
		Title:     "chose SQLite over Postgres",
		Body:      "this should not appear",
	}

	got := FormatEntry(entry, false)
	want := "2026-03-15 14:30:45 [decision] (abcdef12) chose SQLite over Postgres"
	if got != want {
		t.Fatalf("got %q, want %q", got, want)
	}
}

func TestFormatEntry_WithVerbose(t *testing.T) {
	entry := store.Entry{
		Timestamp: time.Date(2026, 3, 15, 14, 30, 45, 0, time.UTC),
		Type:      store.EntryTypeDecision,
		SessionID: "abcdef1234567890",
		Title:     "chose SQLite over Postgres",
		Body:      "line one\nline two",
	}

	got := FormatEntry(entry, true)
	if !strings.Contains(got, "chose SQLite over Postgres") {
		t.Fatal("output should contain the title")
	}
	if !strings.Contains(got, "    line one") {
		t.Fatal("output should contain indented body line one")
	}
	if !strings.Contains(got, "    line two") {
		t.Fatal("output should contain indented body line two")
	}
}

func TestFormatEntry_VerboseEmptyBody(t *testing.T) {
	entry := store.Entry{
		Timestamp: time.Date(2026, 3, 15, 14, 30, 45, 0, time.UTC),
		Type:      store.EntryTypeQuestion,
		SessionID: "sess1234",
		Title:     "should we use gRPC?",
	}

	got := FormatEntry(entry, true)
	if strings.Contains(got, "\n") {
		t.Fatal("verbose with empty body should not add extra lines")
	}
}

func TestPathResolution_RelativeToAbsolute(t *testing.T) {
	rel := "internal/cli/blame.go"
	abs, err := filepath.Abs(rel)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !filepath.IsAbs(abs) {
		t.Fatalf("expected absolute path, got %q", abs)
	}
	if !strings.HasSuffix(abs, rel) {
		t.Fatalf("absolute path %q should end with %q", abs, rel)
	}
}

func TestPathResolution_AlreadyAbsolute(t *testing.T) {
	input := "/usr/local/bin/tool"
	abs, err := filepath.Abs(input)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if abs != input {
		t.Fatalf("got %q, want %q", abs, input)
	}
}

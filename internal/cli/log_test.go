package cli

import (
	"testing"
	"time"
)

func TestParseLogArgs_Defaults(t *testing.T) {
	opts, err := ParseLogArgs("/tmp/test", nil)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if opts.Dir != "/tmp/test" {
		t.Fatalf("Dir = %q, want /tmp/test", opts.Dir)
	}
	if opts.Limit != 50 {
		t.Fatalf("Limit = %d, want 50", opts.Limit)
	}
	if opts.Offset != 0 {
		t.Fatalf("Offset = %d, want 0", opts.Offset)
	}
	if opts.Verbose {
		t.Fatal("Verbose should default to false")
	}
	if opts.Type != "" {
		t.Fatalf("Type = %q, want empty", opts.Type)
	}
}

func TestParseLogArgs_AllFlags(t *testing.T) {
	args := []string{
		"--type", "decision",
		"--session", "sess-123",
		"--tag", "architecture",
		"--since", "1h",
		"--until", "2026-03-15T00:00:00Z",
		"--file", "main.go",
		"--verbose",
		"--limit", "10",
		"--offset", "5",
	}

	opts, err := ParseLogArgs("/tmp/test", args)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if opts.Type != "decision" {
		t.Fatalf("Type = %q, want decision", opts.Type)
	}
	if opts.Session != "sess-123" {
		t.Fatalf("Session = %q, want sess-123", opts.Session)
	}
	if opts.Tag != "architecture" {
		t.Fatalf("Tag = %q, want architecture", opts.Tag)
	}
	if opts.Since != "1h" {
		t.Fatalf("Since = %q, want 1h", opts.Since)
	}
	if opts.Until != "2026-03-15T00:00:00Z" {
		t.Fatalf("Until = %q, want 2026-03-15T00:00:00Z", opts.Until)
	}
	if opts.File != "main.go" {
		t.Fatalf("File = %q, want main.go", opts.File)
	}
	if !opts.Verbose {
		t.Fatal("Verbose should be true")
	}
	if opts.Limit != 10 {
		t.Fatalf("Limit = %d, want 10", opts.Limit)
	}
	if opts.Offset != 5 {
		t.Fatalf("Offset = %d, want 5", opts.Offset)
	}
}

func TestParseLogArgs_NegativeLimit(t *testing.T) {
	_, err := ParseLogArgs("/tmp/test", []string{"--limit", "-1"})
	if err == nil {
		t.Fatal("expected error for negative limit")
	}
}

func TestParseLogArgs_NegativeOffset(t *testing.T) {
	_, err := ParseLogArgs("/tmp/test", []string{"--offset", "-1"})
	if err == nil {
		t.Fatal("expected error for negative offset")
	}
}

func TestParseTimeFlag_Empty(t *testing.T) {
	now := time.Date(2026, 3, 15, 12, 0, 0, 0, time.UTC)
	got, err := ParseTimeFlag("", now)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !got.IsZero() {
		t.Fatalf("expected zero time, got %v", got)
	}
}

func TestParseTimeFlag_RFC3339(t *testing.T) {
	now := time.Date(2026, 3, 15, 12, 0, 0, 0, time.UTC)
	input := "2026-03-14T10:30:00Z"
	got, err := ParseTimeFlag(input, now)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	want := time.Date(2026, 3, 14, 10, 30, 0, 0, time.UTC)
	if !got.Equal(want) {
		t.Fatalf("got %v, want %v", got, want)
	}
}

func TestParseTimeFlag_DateOnly(t *testing.T) {
	now := time.Date(2026, 3, 15, 12, 0, 0, 0, time.UTC)
	got, err := ParseTimeFlag("2026-03-14", now)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	want := time.Date(2026, 3, 14, 0, 0, 0, 0, time.UTC)
	if !got.Equal(want) {
		t.Fatalf("got %v, want %v", got, want)
	}
}

func TestParseTimeFlag_RelativeMinutes(t *testing.T) {
	now := time.Date(2026, 3, 15, 12, 0, 0, 0, time.UTC)
	got, err := ParseTimeFlag("30m", now)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	want := now.Add(-30 * time.Minute)
	if !got.Equal(want) {
		t.Fatalf("got %v, want %v", got, want)
	}
}

func TestParseTimeFlag_RelativeHours(t *testing.T) {
	now := time.Date(2026, 3, 15, 12, 0, 0, 0, time.UTC)
	got, err := ParseTimeFlag("1h", now)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	want := now.Add(-1 * time.Hour)
	if !got.Equal(want) {
		t.Fatalf("got %v, want %v", got, want)
	}
}

func TestParseTimeFlag_RelativeDays(t *testing.T) {
	now := time.Date(2026, 3, 15, 12, 0, 0, 0, time.UTC)
	got, err := ParseTimeFlag("7d", now)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	want := now.AddDate(0, 0, -7)
	if !got.Equal(want) {
		t.Fatalf("got %v, want %v", got, want)
	}
}

func TestParseTimeFlag_RelativeWeeks(t *testing.T) {
	now := time.Date(2026, 3, 15, 12, 0, 0, 0, time.UTC)
	got, err := ParseTimeFlag("2w", now)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	want := now.AddDate(0, 0, -14)
	if !got.Equal(want) {
		t.Fatalf("got %v, want %v", got, want)
	}
}

func TestParseTimeFlag_24h(t *testing.T) {
	now := time.Date(2026, 3, 15, 12, 0, 0, 0, time.UTC)
	got, err := ParseTimeFlag("24h", now)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	want := now.Add(-24 * time.Hour)
	if !got.Equal(want) {
		t.Fatalf("got %v, want %v", got, want)
	}
}

func TestParseTimeFlag_InvalidFormat(t *testing.T) {
	now := time.Date(2026, 3, 15, 12, 0, 0, 0, time.UTC)
	_, err := ParseTimeFlag("yesterday", now)
	if err == nil {
		t.Fatal("expected error for invalid format")
	}
}

func TestParseTimeFlag_InvalidUnit(t *testing.T) {
	now := time.Date(2026, 3, 15, 12, 0, 0, 0, time.UTC)
	_, err := ParseTimeFlag("5x", now)
	if err == nil {
		t.Fatal("expected error for invalid unit")
	}
}

func TestParseTimeFlag_SingleChar(t *testing.T) {
	now := time.Date(2026, 3, 15, 12, 0, 0, 0, time.UTC)
	_, err := ParseTimeFlag("h", now)
	if err == nil {
		t.Fatal("expected error for single character")
	}
}

func TestParseTimeFlag_ZeroDuration(t *testing.T) {
	now := time.Date(2026, 3, 15, 12, 0, 0, 0, time.UTC)
	_, err := ParseTimeFlag("0h", now)
	if err == nil {
		t.Fatal("expected error for zero duration")
	}
}

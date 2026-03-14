package cli

import (
	"bytes"
	"strings"
	"testing"
	"time"

	"github.com/byronxlg/agentlog/internal/store"
)

func TestParseQueryArgs(t *testing.T) {
	tests := []struct {
		name    string
		args    []string
		wantErr bool
		check   func(*testing.T, *QueryConfig)
	}{
		{
			name: "basic search term",
			args: []string{"auth"},
			check: func(t *testing.T, cfg *QueryConfig) {
				if cfg.SearchTerm != "auth" {
					t.Errorf("SearchTerm = %q, want %q", cfg.SearchTerm, "auth")
				}
				if cfg.Limit != 20 {
					t.Errorf("Limit = %d, want 20", cfg.Limit)
				}
			},
		},
		{
			name: "multi-word search term",
			args: []string{"auth", "token", "refresh"},
			check: func(t *testing.T, cfg *QueryConfig) {
				if cfg.SearchTerm != "auth token refresh" {
					t.Errorf("SearchTerm = %q, want %q", cfg.SearchTerm, "auth token refresh")
				}
			},
		},
		{
			name: "all flags",
			args: []string{"--type", "decision", "--session", "abc123", "--tag", "security",
				"--since", "1h", "--until", "30m", "--file", "main.go", "--limit", "5", "search term"},
			check: func(t *testing.T, cfg *QueryConfig) {
				if cfg.Type != "decision" {
					t.Errorf("Type = %q, want %q", cfg.Type, "decision")
				}
				if cfg.Session != "abc123" {
					t.Errorf("Session = %q, want %q", cfg.Session, "abc123")
				}
				if cfg.Tag != "security" {
					t.Errorf("Tag = %q, want %q", cfg.Tag, "security")
				}
				if cfg.Since != "1h" {
					t.Errorf("Since = %q, want %q", cfg.Since, "1h")
				}
				if cfg.Until != "30m" {
					t.Errorf("Until = %q, want %q", cfg.Until, "30m")
				}
				if cfg.File != "main.go" {
					t.Errorf("File = %q, want %q", cfg.File, "main.go")
				}
				if cfg.Limit != 5 {
					t.Errorf("Limit = %d, want 5", cfg.Limit)
				}
				if cfg.SearchTerm != "search term" {
					t.Errorf("SearchTerm = %q, want %q", cfg.SearchTerm, "search term")
				}
			},
		},
		{
			name:    "no search term",
			args:    []string{},
			wantErr: true,
		},
		{
			name:    "only flags no search term",
			args:    []string{"--type", "decision"},
			wantErr: true,
		},
		{
			name:    "invalid flag",
			args:    []string{"--bogus", "val", "term"},
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			cfg, err := ParseQueryArgs(tt.args)
			if tt.wantErr {
				if err == nil {
					t.Fatal("expected error, got nil")
				}
				return
			}
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
			if tt.check != nil {
				tt.check(t, cfg)
			}
		})
	}
}

func TestParseDuration(t *testing.T) {
	now := time.Date(2026, 3, 15, 12, 0, 0, 0, time.UTC)

	tests := []struct {
		name    string
		input   string
		want    time.Time
		wantErr bool
	}{
		{
			name:  "minutes",
			input: "30m",
			want:  now.Add(-30 * time.Minute),
		},
		{
			name:  "hours",
			input: "2h",
			want:  now.Add(-2 * time.Hour),
		},
		{
			name:  "days",
			input: "7d",
			want:  now.AddDate(0, 0, -7),
		},
		{
			name:  "ISO 8601",
			input: "2026-03-14T10:00:00Z",
			want:  time.Date(2026, 3, 14, 10, 0, 0, 0, time.UTC),
		},
		{
			name:  "empty string",
			input: "",
			want:  time.Time{},
		},
		{
			name:    "invalid",
			input:   "abc",
			wantErr: true,
		},
		{
			name:    "unknown unit",
			input:   "5x",
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := parseDuration(tt.input, now)
			if tt.wantErr {
				if err == nil {
					t.Fatal("expected error, got nil")
				}
				return
			}
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
			if !got.Equal(tt.want) {
				t.Errorf("got %v, want %v", got, tt.want)
			}
		})
	}
}

func TestFilterEntries(t *testing.T) {
	now := time.Date(2026, 3, 15, 12, 0, 0, 0, time.UTC)

	entries := []store.Entry{
		{
			ID:        "1",
			Timestamp: now.Add(-1 * time.Hour),
			SessionID: "sess-aaa",
			Type:      "decision",
			Title:     "Use JWT for auth",
			Tags:      []string{"security", "auth"},
			FileRefs:  []string{"auth.go"},
		},
		{
			ID:        "2",
			Timestamp: now.Add(-2 * time.Hour),
			SessionID: "sess-bbb",
			Type:      "assumption",
			Title:     "Database supports JSONB",
			Tags:      []string{"database"},
			FileRefs:  []string{"db.go"},
		},
		{
			ID:        "3",
			Timestamp: now.Add(-5 * time.Hour),
			SessionID: "sess-aaa",
			Type:      "question",
			Title:     "Which auth provider?",
			Tags:      []string{"auth"},
			FileRefs:  []string{"auth.go", "config.go"},
		},
	}

	tests := []struct {
		name    string
		cfg     *QueryConfig
		wantIDs []string
		wantErr bool
	}{
		{
			name:    "no filters",
			cfg:     &QueryConfig{Limit: 20},
			wantIDs: []string{"1", "2", "3"},
		},
		{
			name:    "filter by type",
			cfg:     &QueryConfig{Type: "decision", Limit: 20},
			wantIDs: []string{"1"},
		},
		{
			name:    "filter by session",
			cfg:     &QueryConfig{Session: "sess-aaa", Limit: 20},
			wantIDs: []string{"1", "3"},
		},
		{
			name:    "filter by tag",
			cfg:     &QueryConfig{Tag: "auth", Limit: 20},
			wantIDs: []string{"1", "3"},
		},
		{
			name:    "filter by file",
			cfg:     &QueryConfig{File: "config.go", Limit: 20},
			wantIDs: []string{"3"},
		},
		{
			name:    "filter by since (3h ago)",
			cfg:     &QueryConfig{Since: "3h", Limit: 20},
			wantIDs: []string{"1", "2"},
		},
		{
			name:    "filter by until (90m ago)",
			cfg:     &QueryConfig{Until: "90m", Limit: 20},
			wantIDs: []string{"2", "3"},
		},
		{
			name:    "combined filters",
			cfg:     &QueryConfig{Session: "sess-aaa", Tag: "auth", Limit: 20},
			wantIDs: []string{"1", "3"},
		},
		{
			name:    "limit",
			cfg:     &QueryConfig{Limit: 1},
			wantIDs: []string{"1"},
		},
		{
			name:    "no matches",
			cfg:     &QueryConfig{Type: "deferred", Limit: 20},
			wantIDs: nil,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := FilterEntries(entries, tt.cfg, now)
			if tt.wantErr {
				if err == nil {
					t.Fatal("expected error, got nil")
				}
				return
			}
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
			var gotIDs []string
			for _, e := range got {
				gotIDs = append(gotIDs, e.ID)
			}
			if len(gotIDs) != len(tt.wantIDs) {
				t.Fatalf("got %d results %v, want %d %v", len(gotIDs), gotIDs, len(tt.wantIDs), tt.wantIDs)
			}
			for i, id := range gotIDs {
				if id != tt.wantIDs[i] {
					t.Errorf("result[%d] = %q, want %q", i, id, tt.wantIDs[i])
				}
			}
		})
	}
}

func TestFormatResults(t *testing.T) {
	t.Run("no results", func(t *testing.T) {
		var buf bytes.Buffer
		FormatResults(&buf, nil, "test")
		if !strings.Contains(buf.String(), "No results found.") {
			t.Errorf("expected 'No results found.' in output, got: %s", buf.String())
		}
	})

	t.Run("single result", func(t *testing.T) {
		var buf bytes.Buffer
		entries := []store.Entry{
			{
				ID:        "abc12345-1234-1234-1234-123456789012",
				Timestamp: time.Date(2026, 3, 15, 10, 30, 0, 0, time.UTC),
				SessionID: "session-1234-5678",
				Type:      "decision",
				Title:     "Use JWT tokens",
				Body:      "We decided to use JWT tokens for authentication.",
			},
		}
		FormatResults(&buf, entries, "JWT")
		output := buf.String()

		if !strings.Contains(output, "2026-03-15 10:30:00") {
			t.Error("expected timestamp in output")
		}
		if !strings.Contains(output, "decision") {
			t.Error("expected type in output")
		}
		if !strings.Contains(output, "session-") {
			t.Error("expected session ID prefix in output")
		}
		if !strings.Contains(output, "1 result(s)") {
			t.Error("expected result count in output")
		}
	})
}

func TestHighlightTerms(t *testing.T) {
	tests := []struct {
		name       string
		text       string
		searchTerm string
		wantBold   string
	}{
		{
			name:       "single word",
			text:       "Use JWT for auth",
			searchTerm: "JWT",
			wantBold:   "\033[1mJWT\033[0m",
		},
		{
			name:       "case insensitive",
			text:       "Use jwt for auth",
			searchTerm: "JWT",
			wantBold:   "\033[1mjwt\033[0m",
		},
		{
			name:       "empty search",
			text:       "some text",
			searchTerm: "",
			wantBold:   "",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := highlightTerms(tt.text, tt.searchTerm)
			if tt.wantBold != "" && !strings.Contains(got, tt.wantBold) {
				t.Errorf("expected %q in result %q", tt.wantBold, got)
			}
			if tt.wantBold == "" && got != tt.text {
				t.Errorf("expected unchanged text %q, got %q", tt.text, got)
			}
		})
	}
}

func TestMakeSnippet(t *testing.T) {
	t.Run("short body", func(t *testing.T) {
		got := makeSnippet("short text", "short", 120)
		if got != "short text" {
			t.Errorf("got %q, want %q", got, "short text")
		}
	})

	t.Run("long body with match", func(t *testing.T) {
		body := strings.Repeat("a", 100) + "MATCH" + strings.Repeat("b", 100)
		got := makeSnippet(body, "MATCH", 50)
		if !strings.Contains(got, "MATCH") {
			t.Error("expected snippet to contain the match")
		}
		if len(got) > 60 { // 50 + ellipsis overhead
			t.Errorf("snippet too long: %d chars", len(got))
		}
	})

	t.Run("newlines replaced", func(t *testing.T) {
		got := makeSnippet("line1\nline2\nline3", "line2", 120)
		if strings.Contains(got, "\n") {
			t.Error("expected newlines to be replaced")
		}
	})
}

func TestSessionIDTruncation(t *testing.T) {
	var buf bytes.Buffer
	entries := []store.Entry{
		{
			ID:        "id1",
			Timestamp: time.Date(2026, 1, 1, 0, 0, 0, 0, time.UTC),
			SessionID: "abcdefghijklmnop",
			Type:      "decision",
			Title:     "Test",
		},
	}
	FormatResults(&buf, entries, "test")
	output := buf.String()

	// Should show first 8 chars of session ID
	if !strings.Contains(output, "abcdefgh") {
		t.Error("expected truncated session ID in output")
	}
	if strings.Contains(output, "abcdefghijklmnop") {
		t.Error("expected session ID to be truncated")
	}
}

package daemon

import (
	"encoding/json"
	"fmt"
	"strings"
	"testing"
	"time"

	"github.com/byronxlg/agentlog/internal/store"
)

func seedExportEntries(t *testing.T, d *Daemon) string {
	t.Helper()
	sessionID := d.createSession()

	entries := []store.Entry{
		{ID: "exp1", Timestamp: time.Date(2026, 3, 10, 10, 0, 0, 0, time.UTC), SessionID: sessionID, Type: store.EntryTypeDecision, Title: "Use SQLite for indexing", Body: "Lightweight embedded DB.", Tags: []string{"database"}, FileRefs: []string{"internal/index/index.go"}},
		{ID: "exp2", Timestamp: time.Date(2026, 3, 10, 11, 0, 0, 0, time.UTC), SessionID: sessionID, Type: store.EntryTypeAssumption, Title: "Single writer is sufficient", Body: "No concurrent writes expected.", Tags: []string{"architecture"}, FileRefs: []string{"internal/store/store.go"}},
		{ID: "exp3", Timestamp: time.Date(2026, 3, 11, 9, 0, 0, 0, time.UTC), SessionID: sessionID, Type: store.EntryTypeDeferred, Title: "Add compression support", Body: "Not needed for v1.", Tags: []string{"performance"}},
		{ID: "exp4", Timestamp: time.Date(2026, 3, 11, 14, 0, 0, 0, time.UTC), SessionID: sessionID, Type: store.EntryTypeAttemptFailed, Title: "Tried protobuf encoding", Body: "Too complex for simple log entries.", Tags: []string{"encoding"}},
		{ID: "exp5", Timestamp: time.Date(2026, 3, 12, 8, 0, 0, 0, time.UTC), SessionID: sessionID, Type: store.EntryTypeQuestion, Title: "Should we add gRPC?", Body: "Would simplify SDK integration.", Tags: []string{"architecture"}, FileRefs: []string{"internal/daemon/daemon.go"}},
	}

	for _, e := range entries {
		if err := d.store.Append(e); err != nil {
			t.Fatalf("append: %v", err)
		}
		if err := d.index.Insert(e); err != nil {
			t.Fatalf("insert: %v", err)
		}
	}

	return sessionID
}

func TestHandleExport_DefaultMarkdown(t *testing.T) {
	d := newTestDaemon(t)
	defer func() { _ = d.index.Close() }()
	seedExportEntries(t, d)

	params, _ := json.Marshal(ExportParams{})
	resp := d.handleRequest(Request{Method: "export", Params: params})
	if !resp.OK {
		t.Fatalf("export failed: %s", resp.Error)
	}

	var output string
	if err := json.Unmarshal(resp.Result, &output); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}

	if !strings.Contains(output, "# Decision Log Export") {
		t.Error("missing markdown header")
	}
	if !strings.Contains(output, "Use SQLite for indexing") {
		t.Error("missing entry title")
	}
	if !strings.Contains(output, "**Type:** decision") {
		t.Error("missing type field")
	}
}

func TestHandleExport_JSONFormat(t *testing.T) {
	d := newTestDaemon(t)
	defer func() { _ = d.index.Close() }()
	seedExportEntries(t, d)

	params, _ := json.Marshal(ExportParams{Format: "json"})
	resp := d.handleRequest(Request{Method: "export", Params: params})
	if !resp.OK {
		t.Fatalf("export failed: %s", resp.Error)
	}

	var output string
	if err := json.Unmarshal(resp.Result, &output); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}

	var entries []store.Entry
	if err := json.Unmarshal([]byte(output), &entries); err != nil {
		t.Fatalf("JSON format should produce valid JSON array: %v", err)
	}
	if len(entries) != 5 {
		t.Fatalf("expected 5 entries, got %d", len(entries))
	}
}

func TestHandleExport_TextFormat(t *testing.T) {
	d := newTestDaemon(t)
	defer func() { _ = d.index.Close() }()
	seedExportEntries(t, d)

	params, _ := json.Marshal(ExportParams{Format: "text"})
	resp := d.handleRequest(Request{Method: "export", Params: params})
	if !resp.OK {
		t.Fatalf("export failed: %s", resp.Error)
	}

	var output string
	if err := json.Unmarshal(resp.Result, &output); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}

	if !strings.Contains(output, "[decision] Use SQLite for indexing") {
		t.Error("missing text format entry")
	}
	if !strings.Contains(output, "Tags: database") {
		t.Error("missing tags in text format")
	}
}

func TestHandleExport_FilterBySession(t *testing.T) {
	d := newTestDaemon(t)
	defer func() { _ = d.index.Close() }()
	sid := seedExportEntries(t, d)

	// Seed another session with different entries.
	otherSession := d.createSession()
	other := store.Entry{ID: "other1", Timestamp: time.Date(2026, 3, 12, 10, 0, 0, 0, time.UTC), SessionID: otherSession, Type: store.EntryTypeDecision, Title: "Other session entry"}
	_ = d.store.Append(other)
	_ = d.index.Insert(other)

	params, _ := json.Marshal(ExportParams{SessionID: sid, Format: "json"})
	resp := d.handleRequest(Request{Method: "export", Params: params})
	if !resp.OK {
		t.Fatalf("export failed: %s", resp.Error)
	}

	var output string
	_ = json.Unmarshal(resp.Result, &output)
	var entries []store.Entry
	_ = json.Unmarshal([]byte(output), &entries)

	if len(entries) != 5 {
		t.Fatalf("expected 5 entries for session, got %d", len(entries))
	}
	for _, e := range entries {
		if e.SessionID != sid {
			t.Errorf("entry %s has wrong session %s", e.ID, e.SessionID)
		}
	}
}

func TestHandleExport_FilterByType(t *testing.T) {
	d := newTestDaemon(t)
	defer func() { _ = d.index.Close() }()
	seedExportEntries(t, d)

	params, _ := json.Marshal(ExportParams{Type: "decision", Format: "json"})
	resp := d.handleRequest(Request{Method: "export", Params: params})
	if !resp.OK {
		t.Fatalf("export failed: %s", resp.Error)
	}

	var output string
	_ = json.Unmarshal(resp.Result, &output)
	var entries []store.Entry
	_ = json.Unmarshal([]byte(output), &entries)

	if len(entries) != 1 {
		t.Fatalf("expected 1 decision entry, got %d", len(entries))
	}
	if entries[0].Title != "Use SQLite for indexing" {
		t.Errorf("unexpected title: %s", entries[0].Title)
	}
}

func TestHandleExport_FilterByTag(t *testing.T) {
	d := newTestDaemon(t)
	defer func() { _ = d.index.Close() }()
	seedExportEntries(t, d)

	params, _ := json.Marshal(ExportParams{Tag: "architecture", Format: "json"})
	resp := d.handleRequest(Request{Method: "export", Params: params})
	if !resp.OK {
		t.Fatalf("export failed: %s", resp.Error)
	}

	var output string
	_ = json.Unmarshal(resp.Result, &output)
	var entries []store.Entry
	_ = json.Unmarshal([]byte(output), &entries)

	if len(entries) != 2 {
		t.Fatalf("expected 2 architecture-tagged entries, got %d", len(entries))
	}
}

func TestHandleExport_FilterByFile(t *testing.T) {
	d := newTestDaemon(t)
	defer func() { _ = d.index.Close() }()
	seedExportEntries(t, d)

	params, _ := json.Marshal(ExportParams{FilePath: "internal/index/index.go", Format: "json"})
	resp := d.handleRequest(Request{Method: "export", Params: params})
	if !resp.OK {
		t.Fatalf("export failed: %s", resp.Error)
	}

	var output string
	_ = json.Unmarshal(resp.Result, &output)
	var entries []store.Entry
	_ = json.Unmarshal([]byte(output), &entries)

	if len(entries) != 1 {
		t.Fatalf("expected 1 entry for file, got %d", len(entries))
	}
	if entries[0].ID != "exp1" {
		t.Errorf("expected exp1, got %s", entries[0].ID)
	}
}

func TestHandleExport_FilterBySince(t *testing.T) {
	d := newTestDaemon(t)
	defer func() { _ = d.index.Close() }()
	seedExportEntries(t, d)

	params, _ := json.Marshal(ExportParams{Since: "2026-03-11T00:00:00Z", Format: "json"})
	resp := d.handleRequest(Request{Method: "export", Params: params})
	if !resp.OK {
		t.Fatalf("export failed: %s", resp.Error)
	}

	var output string
	_ = json.Unmarshal(resp.Result, &output)
	var entries []store.Entry
	_ = json.Unmarshal([]byte(output), &entries)

	if len(entries) != 3 {
		t.Fatalf("expected 3 entries after 2026-03-11, got %d", len(entries))
	}
}

func TestHandleExport_FilterByUntil(t *testing.T) {
	d := newTestDaemon(t)
	defer func() { _ = d.index.Close() }()
	seedExportEntries(t, d)

	params, _ := json.Marshal(ExportParams{Until: "2026-03-10T23:59:59Z", Format: "json"})
	resp := d.handleRequest(Request{Method: "export", Params: params})
	if !resp.OK {
		t.Fatalf("export failed: %s", resp.Error)
	}

	var output string
	_ = json.Unmarshal(resp.Result, &output)
	var entries []store.Entry
	_ = json.Unmarshal([]byte(output), &entries)

	if len(entries) != 2 {
		t.Fatalf("expected 2 entries before 2026-03-11, got %d", len(entries))
	}
}

func TestHandleExport_CombinedFilters(t *testing.T) {
	d := newTestDaemon(t)
	defer func() { _ = d.index.Close() }()
	sid := seedExportEntries(t, d)

	// Filter by session + type + since.
	params, _ := json.Marshal(ExportParams{
		SessionID: sid,
		Type:      "decision",
		Since:     "2026-03-10T00:00:00Z",
		Format:    "json",
	})
	resp := d.handleRequest(Request{Method: "export", Params: params})
	if !resp.OK {
		t.Fatalf("export failed: %s", resp.Error)
	}

	var output string
	_ = json.Unmarshal(resp.Result, &output)
	var entries []store.Entry
	_ = json.Unmarshal([]byte(output), &entries)

	if len(entries) != 1 {
		t.Fatalf("expected 1 entry matching combined filters, got %d", len(entries))
	}
}

func TestHandleExport_EmptyResult(t *testing.T) {
	d := newTestDaemon(t)
	defer func() { _ = d.index.Close() }()

	// No entries seeded.
	params, _ := json.Marshal(ExportParams{Format: "markdown"})
	resp := d.handleRequest(Request{Method: "export", Params: params})
	if !resp.OK {
		t.Fatalf("export failed: %s", resp.Error)
	}

	var output string
	_ = json.Unmarshal(resp.Result, &output)
	if output != "No entries found." {
		t.Errorf("expected empty message, got %q", output)
	}

	// JSON format empty.
	params, _ = json.Marshal(ExportParams{Format: "json"})
	resp = d.handleRequest(Request{Method: "export", Params: params})
	if !resp.OK {
		t.Fatalf("export json failed: %s", resp.Error)
	}
	_ = json.Unmarshal(resp.Result, &output)
	if output != "[]" {
		t.Errorf("expected empty JSON array, got %q", output)
	}

	// Text format empty.
	params, _ = json.Marshal(ExportParams{Format: "text"})
	resp = d.handleRequest(Request{Method: "export", Params: params})
	if !resp.OK {
		t.Fatalf("export text failed: %s", resp.Error)
	}
	_ = json.Unmarshal(resp.Result, &output)
	if output != "No entries found." {
		t.Errorf("expected empty text message, got %q", output)
	}
}

func TestHandleExport_InvalidFormat(t *testing.T) {
	d := newTestDaemon(t)
	defer func() { _ = d.index.Close() }()

	params, _ := json.Marshal(ExportParams{Format: "csv"})
	resp := d.handleRequest(Request{Method: "export", Params: params})
	if resp.OK {
		t.Fatal("expected error for invalid format")
	}
	if !strings.Contains(resp.Error, "invalid format") {
		t.Errorf("unexpected error: %s", resp.Error)
	}
}

func TestHandleExport_InvalidTemplate(t *testing.T) {
	d := newTestDaemon(t)
	defer func() { _ = d.index.Close() }()

	params, _ := json.Marshal(ExportParams{Template: "invalid"})
	resp := d.handleRequest(Request{Method: "export", Params: params})
	if resp.OK {
		t.Fatal("expected error for invalid template")
	}
	if !strings.Contains(resp.Error, "invalid template") {
		t.Errorf("unexpected error: %s", resp.Error)
	}
}

func TestHandleExport_TemplatePR(t *testing.T) {
	d := newTestDaemon(t)
	defer func() { _ = d.index.Close() }()
	seedExportEntries(t, d)

	params, _ := json.Marshal(ExportParams{Template: "pr"})
	resp := d.handleRequest(Request{Method: "export", Params: params})
	if !resp.OK {
		t.Fatalf("export failed: %s", resp.Error)
	}

	var output string
	_ = json.Unmarshal(resp.Result, &output)

	if !strings.Contains(output, "## What changed") {
		t.Error("missing PR template header")
	}
	if !strings.Contains(output, "Use SQLite for indexing") {
		t.Error("missing decision entry in PR template")
	}
	if !strings.Contains(output, "## Notes") {
		t.Error("missing notes section for non-decision entries")
	}
}

func TestHandleExport_TemplateRetro(t *testing.T) {
	d := newTestDaemon(t)
	defer func() { _ = d.index.Close() }()
	seedExportEntries(t, d)

	params, _ := json.Marshal(ExportParams{Template: "retro"})
	resp := d.handleRequest(Request{Method: "export", Params: params})
	if !resp.OK {
		t.Fatalf("export failed: %s", resp.Error)
	}

	var output string
	_ = json.Unmarshal(resp.Result, &output)

	if !strings.Contains(output, "# Retrospective") {
		t.Error("missing retro header")
	}
	if !strings.Contains(output, "## Decisions made") {
		t.Error("missing decisions section")
	}
	if !strings.Contains(output, "## Deferred items") {
		t.Error("missing deferred section")
	}
	if !strings.Contains(output, "## Assumptions") {
		t.Error("missing assumptions section")
	}
	if !strings.Contains(output, "## Failed attempts") {
		t.Error("missing failed attempts section")
	}
	if !strings.Contains(output, "## Open questions") {
		t.Error("missing questions section")
	}
}

func TestHandleExport_TemplateHandoff(t *testing.T) {
	d := newTestDaemon(t)
	defer func() { _ = d.index.Close() }()
	seedExportEntries(t, d)

	params, _ := json.Marshal(ExportParams{FilePath: "internal/index/index.go", Template: "handoff"})
	resp := d.handleRequest(Request{Method: "export", Params: params})
	if !resp.OK {
		t.Fatalf("export failed: %s", resp.Error)
	}

	var output string
	_ = json.Unmarshal(resp.Result, &output)

	if !strings.Contains(output, "# Handoff Document") {
		t.Error("missing handoff header")
	}
	if !strings.Contains(output, "Use SQLite for indexing") {
		t.Error("missing entry in handoff")
	}
}

func TestHandleExport_TemplateEmptyResult(t *testing.T) {
	d := newTestDaemon(t)
	defer func() { _ = d.index.Close() }()

	templates := []struct {
		name     string
		expected string
	}{
		{"pr", "No decisions to summarize."},
		{"retro", "No entries for retrospective."},
		{"handoff", "No decisions found for these files."},
	}

	for _, tc := range templates {
		t.Run(tc.name, func(t *testing.T) {
			params, _ := json.Marshal(ExportParams{Template: tc.name})
			resp := d.handleRequest(Request{Method: "export", Params: params})
			if !resp.OK {
				t.Fatalf("export failed: %s", resp.Error)
			}

			var output string
			_ = json.Unmarshal(resp.Result, &output)
			if output != tc.expected {
				t.Errorf("expected %q, got %q", tc.expected, output)
			}
		})
	}
}

func TestHandleExport_Routing(t *testing.T) {
	d := newTestDaemon(t)
	defer func() { _ = d.index.Close() }()

	resp := d.handleRequest(Request{Method: "export"})
	// Should not return "unknown method", just invalid params.
	if resp.Error == "unknown method: export" {
		t.Error("export method not routed")
	}
}

func TestHandleExport_RelativeTime(t *testing.T) {
	d := newTestDaemon(t)
	defer func() { _ = d.index.Close() }()

	sessionID := d.createSession()
	now := time.Now().UTC()

	entries := []store.Entry{
		{ID: "rt1", Timestamp: now.Add(-2 * time.Hour), SessionID: sessionID, Type: store.EntryTypeDecision, Title: "Two hours ago"},
		{ID: "rt2", Timestamp: now.Add(-30 * time.Minute), SessionID: sessionID, Type: store.EntryTypeDecision, Title: "Thirty minutes ago"},
		{ID: "rt3", Timestamp: now.Add(-5 * time.Minute), SessionID: sessionID, Type: store.EntryTypeDecision, Title: "Five minutes ago"},
	}
	for _, e := range entries {
		_ = d.store.Append(e)
		_ = d.index.Insert(e)
	}

	params, _ := json.Marshal(ExportParams{Since: "1h", Format: "json"})
	resp := d.handleRequest(Request{Method: "export", Params: params})
	if !resp.OK {
		t.Fatalf("export failed: %s", resp.Error)
	}

	var output string
	_ = json.Unmarshal(resp.Result, &output)
	var result []store.Entry
	_ = json.Unmarshal([]byte(output), &result)

	if len(result) != 2 {
		t.Fatalf("expected 2 entries within last hour, got %d (entries: %v)", len(result), titlesOf(result))
	}
}

func titlesOf(entries []store.Entry) []string {
	titles := make([]string, len(entries))
	for i, e := range entries {
		titles[i] = fmt.Sprintf("%s (%s)", e.Title, e.Timestamp.Format(time.RFC3339))
	}
	return titles
}

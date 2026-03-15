package index

import (
	"fmt"
	"path/filepath"
	"testing"
	"time"

	"github.com/byronxlg/agentlog/internal/store"
)

func testIndex(t *testing.T) *Index {
	t.Helper()
	dbPath := filepath.Join(t.TempDir(), "test.db")
	idx, err := Open(dbPath)
	if err != nil {
		t.Fatalf("open index: %v", err)
	}
	t.Cleanup(func() { _ = idx.Close() })
	return idx
}

func testEntry(id string, ts time.Time) store.Entry {
	return store.Entry{
		ID:        id,
		Timestamp: ts,
		SessionID: "session-1",
		Type:      store.EntryTypeDecision,
		Title:     "Test decision " + id,
		Body:      "Body for " + id,
		Tags:      []string{"go", "sqlite"},
		FileRefs:  []string{"internal/index/index.go"},
	}
}

func TestInsertAndRetrieve(t *testing.T) {
	idx := testIndex(t)
	ts := time.Date(2026, 1, 15, 10, 0, 0, 0, time.UTC)
	entry := testEntry("e1", ts)

	if err := idx.Insert(entry); err != nil {
		t.Fatalf("insert: %v", err)
	}

	results, err := idx.QueryBySession("session-1")
	if err != nil {
		t.Fatalf("query by session: %v", err)
	}
	if len(results) != 1 {
		t.Fatalf("expected 1 result, got %d", len(results))
	}

	got := results[0]
	if got.ID != "e1" {
		t.Errorf("id: got %q, want %q", got.ID, "e1")
	}
	if !got.Timestamp.Equal(ts) {
		t.Errorf("timestamp: got %v, want %v", got.Timestamp, ts)
	}
	if got.Title != entry.Title {
		t.Errorf("title: got %q, want %q", got.Title, entry.Title)
	}
	if got.Body != entry.Body {
		t.Errorf("body: got %q, want %q", got.Body, entry.Body)
	}
	if len(got.Tags) != 2 || got.Tags[0] != "go" || got.Tags[1] != "sqlite" {
		t.Errorf("tags: got %v, want [go sqlite]", got.Tags)
	}
	if len(got.FileRefs) != 1 || got.FileRefs[0] != "internal/index/index.go" {
		t.Errorf("file refs: got %v, want [internal/index/index.go]", got.FileRefs)
	}
}

func TestQueryByTimeRange(t *testing.T) {
	idx := testIndex(t)
	t1 := time.Date(2026, 1, 10, 0, 0, 0, 0, time.UTC)
	t2 := time.Date(2026, 1, 15, 0, 0, 0, 0, time.UTC)
	t3 := time.Date(2026, 1, 20, 0, 0, 0, 0, time.UTC)

	for i, ts := range []time.Time{t1, t2, t3} {
		e := testEntry(fmt.Sprintf("e%d", i+1), ts)
		if err := idx.Insert(e); err != nil {
			t.Fatalf("insert: %v", err)
		}
	}

	results, err := idx.QueryByTimeRange(
		time.Date(2026, 1, 12, 0, 0, 0, 0, time.UTC),
		time.Date(2026, 1, 18, 0, 0, 0, 0, time.UTC),
	)
	if err != nil {
		t.Fatalf("query by time range: %v", err)
	}
	if len(results) != 1 {
		t.Fatalf("expected 1 result, got %d", len(results))
	}
	if results[0].ID != "e2" {
		t.Errorf("expected e2, got %q", results[0].ID)
	}
}

func TestQueryByType(t *testing.T) {
	idx := testIndex(t)
	ts := time.Date(2026, 1, 15, 0, 0, 0, 0, time.UTC)

	e1 := testEntry("e1", ts)
	e1.Type = store.EntryTypeDecision

	e2 := testEntry("e2", ts.Add(time.Hour))
	e2.Type = store.EntryTypeQuestion

	for _, e := range []store.Entry{e1, e2} {
		if err := idx.Insert(e); err != nil {
			t.Fatalf("insert: %v", err)
		}
	}

	results, err := idx.QueryByType(store.EntryTypeQuestion)
	if err != nil {
		t.Fatalf("query by type: %v", err)
	}
	if len(results) != 1 {
		t.Fatalf("expected 1 result, got %d", len(results))
	}
	if results[0].ID != "e2" {
		t.Errorf("expected e2, got %q", results[0].ID)
	}
}

func TestQueryBySession(t *testing.T) {
	idx := testIndex(t)
	ts := time.Date(2026, 1, 15, 0, 0, 0, 0, time.UTC)

	e1 := testEntry("e1", ts)
	e1.SessionID = "sess-a"

	e2 := testEntry("e2", ts.Add(time.Hour))
	e2.SessionID = "sess-b"

	for _, e := range []store.Entry{e1, e2} {
		if err := idx.Insert(e); err != nil {
			t.Fatalf("insert: %v", err)
		}
	}

	results, err := idx.QueryBySession("sess-b")
	if err != nil {
		t.Fatalf("query by session: %v", err)
	}
	if len(results) != 1 {
		t.Fatalf("expected 1 result, got %d", len(results))
	}
	if results[0].ID != "e2" {
		t.Errorf("expected e2, got %q", results[0].ID)
	}
}

func TestQueryByTags(t *testing.T) {
	idx := testIndex(t)
	ts := time.Date(2026, 1, 15, 0, 0, 0, 0, time.UTC)

	e1 := testEntry("e1", ts)
	e1.Tags = []string{"go", "sqlite", "backend"}

	e2 := testEntry("e2", ts.Add(time.Hour))
	e2.Tags = []string{"go", "frontend"}

	e3 := testEntry("e3", ts.Add(2*time.Hour))
	e3.Tags = []string{"sqlite", "backend"}

	for _, e := range []store.Entry{e1, e2, e3} {
		if err := idx.Insert(e); err != nil {
			t.Fatalf("insert: %v", err)
		}
	}

	results, err := idx.QueryByTags([]string{"go", "sqlite"})
	if err != nil {
		t.Fatalf("query by tags: %v", err)
	}
	if len(results) != 1 {
		t.Fatalf("expected 1 result (AND semantics), got %d", len(results))
	}
	if results[0].ID != "e1" {
		t.Errorf("expected e1, got %q", results[0].ID)
	}
}

func TestQueryByTagsEmpty(t *testing.T) {
	idx := testIndex(t)
	results, err := idx.QueryByTags(nil)
	if err != nil {
		t.Fatalf("query by empty tags: %v", err)
	}
	if results != nil {
		t.Errorf("expected nil results for empty tags, got %v", results)
	}
}

func TestQueryByFilePath(t *testing.T) {
	idx := testIndex(t)
	ts := time.Date(2026, 1, 15, 0, 0, 0, 0, time.UTC)

	e1 := testEntry("e1", ts)
	e1.FileRefs = []string{"cmd/main.go", "internal/index/index.go"}

	e2 := testEntry("e2", ts.Add(time.Hour))
	e2.FileRefs = []string{"cmd/main.go"}

	for _, e := range []store.Entry{e1, e2} {
		if err := idx.Insert(e); err != nil {
			t.Fatalf("insert: %v", err)
		}
	}

	results, err := idx.QueryByFilePath("internal/index/index.go")
	if err != nil {
		t.Fatalf("query by file path: %v", err)
	}
	if len(results) != 1 {
		t.Fatalf("expected 1 result, got %d", len(results))
	}
	if results[0].ID != "e1" {
		t.Errorf("expected e1, got %q", results[0].ID)
	}
}

func TestQueryRecent(t *testing.T) {
	idx := testIndex(t)
	base := time.Date(2026, 1, 15, 0, 0, 0, 0, time.UTC)

	for i := 0; i < 5; i++ {
		e := testEntry(fmt.Sprintf("e%d", i+1), base.Add(time.Duration(i)*time.Hour))
		if err := idx.Insert(e); err != nil {
			t.Fatalf("insert: %v", err)
		}
	}

	// Most recent first, limit 3
	results, err := idx.QueryRecent(3, 0)
	if err != nil {
		t.Fatalf("query recent: %v", err)
	}
	if len(results) != 3 {
		t.Fatalf("expected 3 results, got %d", len(results))
	}
	if results[0].ID != "e5" {
		t.Errorf("first result should be most recent: got %q, want e5", results[0].ID)
	}
	if results[2].ID != "e3" {
		t.Errorf("third result: got %q, want e3", results[2].ID)
	}

	// With offset
	results, err = idx.QueryRecent(2, 3)
	if err != nil {
		t.Fatalf("query recent with offset: %v", err)
	}
	if len(results) != 2 {
		t.Fatalf("expected 2 results, got %d", len(results))
	}
	if results[0].ID != "e2" {
		t.Errorf("first result after offset: got %q, want e2", results[0].ID)
	}
	if results[1].ID != "e1" {
		t.Errorf("second result after offset: got %q, want e1", results[1].ID)
	}
}

func TestSearch(t *testing.T) {
	idx := testIndex(t)
	ts := time.Date(2026, 1, 15, 0, 0, 0, 0, time.UTC)

	e1 := store.Entry{
		ID:        "e1",
		Timestamp: ts,
		SessionID: "session-1",
		Type:      store.EntryTypeDecision,
		Title:     "Choose SQLite for indexing",
		Body:      "We decided to use SQLite because it is embedded and requires no server.",
	}
	e2 := store.Entry{
		ID:        "e2",
		Timestamp: ts.Add(time.Hour),
		SessionID: "session-1",
		Type:      store.EntryTypeDecision,
		Title:     "Use JSONL for log storage",
		Body:      "JSONL is append-friendly and easy to parse.",
	}

	for _, e := range []store.Entry{e1, e2} {
		if err := idx.Insert(e); err != nil {
			t.Fatalf("insert: %v", err)
		}
	}

	results, err := idx.Search("SQLite")
	if err != nil {
		t.Fatalf("search: %v", err)
	}
	if len(results) != 1 {
		t.Fatalf("expected 1 result, got %d", len(results))
	}
	if results[0].ID != "e1" {
		t.Errorf("expected e1, got %q", results[0].ID)
	}
}

func TestInsertDuplicateUpdates(t *testing.T) {
	idx := testIndex(t)
	ts := time.Date(2026, 1, 15, 0, 0, 0, 0, time.UTC)

	entry := testEntry("e1", ts)
	if err := idx.Insert(entry); err != nil {
		t.Fatalf("insert: %v", err)
	}

	entry.Title = "Updated title"
	entry.Tags = []string{"updated"}
	if err := idx.Insert(entry); err != nil {
		t.Fatalf("insert duplicate: %v", err)
	}

	results, err := idx.QueryBySession("session-1")
	if err != nil {
		t.Fatalf("query: %v", err)
	}
	if len(results) != 1 {
		t.Fatalf("expected 1 result after update, got %d", len(results))
	}
	if results[0].Title != "Updated title" {
		t.Errorf("title not updated: got %q", results[0].Title)
	}
	if len(results[0].Tags) != 1 || results[0].Tags[0] != "updated" {
		t.Errorf("tags not updated: got %v", results[0].Tags)
	}
}

func TestRebuild(t *testing.T) {
	dir := t.TempDir()
	s := store.New(dir)

	ts := time.Date(2026, 1, 15, 0, 0, 0, 0, time.UTC)
	entries := []store.Entry{
		{
			ID:        "e1",
			Timestamp: ts,
			SessionID: "session-1",
			Type:      store.EntryTypeDecision,
			Title:     "First decision",
			Tags:      []string{"alpha"},
		},
		{
			ID:        "e2",
			Timestamp: ts.Add(time.Hour),
			SessionID: "session-1",
			Type:      store.EntryTypeQuestion,
			Title:     "Open question",
			Tags:      []string{"beta"},
		},
	}
	for _, e := range entries {
		if err := s.Append(e); err != nil {
			t.Fatalf("append to store: %v", err)
		}
	}

	dbPath := filepath.Join(dir, "index.db")
	idx, err := Open(dbPath)
	if err != nil {
		t.Fatalf("open index: %v", err)
	}
	defer func() { _ = idx.Close() }()

	stale := store.Entry{
		ID:        "stale",
		Timestamp: ts,
		SessionID: "session-old",
		Type:      store.EntryTypeDeferred,
		Title:     "Stale entry",
	}
	if err := idx.Insert(stale); err != nil {
		t.Fatalf("insert stale: %v", err)
	}

	if err := idx.Rebuild(s); err != nil {
		t.Fatalf("rebuild: %v", err)
	}

	results, err := idx.QueryBySession("session-old")
	if err != nil {
		t.Fatalf("query stale session: %v", err)
	}
	if len(results) != 0 {
		t.Errorf("expected 0 stale results, got %d", len(results))
	}

	results, err = idx.QueryBySession("session-1")
	if err != nil {
		t.Fatalf("query rebuilt session: %v", err)
	}
	if len(results) != 2 {
		t.Fatalf("expected 2 rebuilt results, got %d", len(results))
	}
}

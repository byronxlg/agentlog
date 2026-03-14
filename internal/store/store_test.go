package store

import (
	"fmt"
	"os"
	"path/filepath"
	"sync"
	"testing"
	"time"
)

func newTestEntry(id, sessionID string, entryType EntryType) Entry {
	return Entry{
		ID:        id,
		Timestamp: time.Date(2026, 3, 14, 12, 0, 0, 0, time.UTC),
		SessionID: sessionID,
		Type:      entryType,
		Title:     "Test decision",
		Body:      "Some reasoning here",
		Tags:      []string{"test", "example"},
		FileRefs:  []string{"main.go", "store.go"},
	}
}

func TestAppendAndReadSession_RoundTrip(t *testing.T) {
	dir := t.TempDir()
	s := New(dir)

	entry := newTestEntry("entry-1", "session-1", EntryTypeDecision)
	if err := s.Append(entry); err != nil {
		t.Fatalf("Append: %v", err)
	}

	entries, err := s.ReadSession("session-1")
	if err != nil {
		t.Fatalf("ReadSession: %v", err)
	}
	if len(entries) != 1 {
		t.Fatalf("expected 1 entry, got %d", len(entries))
	}

	got := entries[0]
	if got.ID != entry.ID {
		t.Errorf("ID = %q, want %q", got.ID, entry.ID)
	}
	if got.SessionID != entry.SessionID {
		t.Errorf("SessionID = %q, want %q", got.SessionID, entry.SessionID)
	}
	if got.Type != entry.Type {
		t.Errorf("Type = %q, want %q", got.Type, entry.Type)
	}
	if got.Title != entry.Title {
		t.Errorf("Title = %q, want %q", got.Title, entry.Title)
	}
	if got.Body != entry.Body {
		t.Errorf("Body = %q, want %q", got.Body, entry.Body)
	}
	if !got.Timestamp.Equal(entry.Timestamp) {
		t.Errorf("Timestamp = %v, want %v", got.Timestamp, entry.Timestamp)
	}
	if len(got.Tags) != 2 || got.Tags[0] != "test" || got.Tags[1] != "example" {
		t.Errorf("Tags = %v, want %v", got.Tags, entry.Tags)
	}
	if len(got.FileRefs) != 2 || got.FileRefs[0] != "main.go" || got.FileRefs[1] != "store.go" {
		t.Errorf("FileRefs = %v, want %v", got.FileRefs, entry.FileRefs)
	}
}

func TestAppendMultipleEntries_SameSession(t *testing.T) {
	dir := t.TempDir()
	s := New(dir)

	for i := range 5 {
		entry := newTestEntry(
			"entry-"+string(rune('a'+i)),
			"session-1",
			EntryTypeDecision,
		)
		entry.Title = "Decision " + string(rune('A'+i))
		if err := s.Append(entry); err != nil {
			t.Fatalf("Append %d: %v", i, err)
		}
	}

	entries, err := s.ReadSession("session-1")
	if err != nil {
		t.Fatalf("ReadSession: %v", err)
	}
	if len(entries) != 5 {
		t.Fatalf("expected 5 entries, got %d", len(entries))
	}
}

func TestReadAll_MultipleSessions(t *testing.T) {
	dir := t.TempDir()
	s := New(dir)

	sessions := []string{"sess-a", "sess-b", "sess-c"}
	for _, sid := range sessions {
		entry := newTestEntry("id-"+sid, sid, EntryTypeAssumption)
		if err := s.Append(entry); err != nil {
			t.Fatalf("Append: %v", err)
		}
	}

	all, err := s.ReadAll()
	if err != nil {
		t.Fatalf("ReadAll: %v", err)
	}
	if len(all) != 3 {
		t.Fatalf("expected 3 entries, got %d", len(all))
	}
}

func TestListSessions(t *testing.T) {
	dir := t.TempDir()
	s := New(dir)

	sessions := []string{"alpha", "beta"}
	for _, sid := range sessions {
		if err := s.Append(newTestEntry("id-"+sid, sid, EntryTypeDecision)); err != nil {
			t.Fatalf("Append: %v", err)
		}
	}

	got, err := s.ListSessions()
	if err != nil {
		t.Fatalf("ListSessions: %v", err)
	}
	if len(got) != 2 {
		t.Fatalf("expected 2 sessions, got %d", len(got))
	}
}

func TestAppend_RejectsInvalidType(t *testing.T) {
	dir := t.TempDir()
	s := New(dir)

	entry := newTestEntry("e1", "s1", EntryType("invalid"))
	if err := s.Append(entry); err == nil {
		t.Fatal("expected error for invalid entry type, got nil")
	}
}

func TestAppend_RejectsEmptyID(t *testing.T) {
	dir := t.TempDir()
	s := New(dir)

	entry := newTestEntry("", "s1", EntryTypeDecision)
	if err := s.Append(entry); err == nil {
		t.Fatal("expected error for empty ID, got nil")
	}
}

func TestAppend_RejectsEmptySessionID(t *testing.T) {
	dir := t.TempDir()
	s := New(dir)

	entry := newTestEntry("e1", "", EntryTypeDecision)
	if err := s.Append(entry); err == nil {
		t.Fatal("expected error for empty session ID, got nil")
	}
}

func TestAppend_RejectsEmptyTitle(t *testing.T) {
	dir := t.TempDir()
	s := New(dir)

	entry := newTestEntry("e1", "s1", EntryTypeDecision)
	entry.Title = ""
	if err := s.Append(entry); err == nil {
		t.Fatal("expected error for empty title, got nil")
	}
}

func TestReadSession_NonexistentSession_ReturnsError(t *testing.T) {
	dir := t.TempDir()
	s := New(dir)

	_, err := s.ReadSession("does-not-exist")
	if err == nil {
		t.Fatal("expected error for nonexistent session, got nil")
	}
}

func TestReadAll_EmptyStore_ReturnsEmpty(t *testing.T) {
	dir := t.TempDir()
	s := New(dir)

	entries, err := s.ReadAll()
	if err != nil {
		t.Fatalf("ReadAll: %v", err)
	}
	if len(entries) != 0 {
		t.Fatalf("expected 0 entries, got %d", len(entries))
	}
}

func TestAppend_CreatesLogDirectory(t *testing.T) {
	dir := t.TempDir()
	logDir := filepath.Join(dir, "log")
	s := New(dir)

	// Confirm log dir does not exist yet.
	if _, err := os.Stat(logDir); !os.IsNotExist(err) {
		t.Fatal("log directory should not exist before first write")
	}

	if err := s.Append(newTestEntry("e1", "s1", EntryTypeDecision)); err != nil {
		t.Fatalf("Append: %v", err)
	}

	info, err := os.Stat(logDir)
	if err != nil {
		t.Fatalf("log directory not created: %v", err)
	}
	if !info.IsDir() {
		t.Fatal("log path is not a directory")
	}
}

func TestAppend_AllEntryTypes(t *testing.T) {
	dir := t.TempDir()
	s := New(dir)

	types := []EntryType{
		EntryTypeDecision,
		EntryTypeAttemptFailed,
		EntryTypeDeferred,
		EntryTypeAssumption,
		EntryTypeQuestion,
	}

	for i, et := range types {
		entry := newTestEntry("id-"+string(rune('0'+i)), "s1", et)
		if err := s.Append(entry); err != nil {
			t.Fatalf("Append type %q: %v", et, err)
		}
	}

	entries, err := s.ReadSession("s1")
	if err != nil {
		t.Fatalf("ReadSession: %v", err)
	}
	if len(entries) != len(types) {
		t.Fatalf("expected %d entries, got %d", len(types), len(entries))
	}
	for i, entry := range entries {
		if entry.Type != types[i] {
			t.Errorf("entry %d: Type = %q, want %q", i, entry.Type, types[i])
		}
	}
}

func TestAppend_OmitsEmptyOptionalFields(t *testing.T) {
	dir := t.TempDir()
	s := New(dir)

	entry := Entry{
		ID:        "e1",
		Timestamp: time.Now(),
		SessionID: "s1",
		Type:      EntryTypeDecision,
		Title:     "Minimal entry",
	}
	if err := s.Append(entry); err != nil {
		t.Fatalf("Append: %v", err)
	}

	// Read the raw file to verify omitempty works.
	raw, err := os.ReadFile(s.sessionPath("s1"))
	if err != nil {
		t.Fatalf("ReadFile: %v", err)
	}
	content := string(raw)
	if contains(content, `"body"`) {
		t.Error("empty body should be omitted from JSON")
	}
	if contains(content, `"tags"`) {
		t.Error("nil tags should be omitted from JSON")
	}
	if contains(content, `"file_refs"`) {
		t.Error("nil file_refs should be omitted from JSON")
	}
}

func TestConcurrentAppend_NoCorruption(t *testing.T) {
	dir := t.TempDir()
	s := New(dir)

	const goroutines = 20
	var wg sync.WaitGroup
	wg.Add(goroutines)

	for i := range goroutines {
		go func(n int) {
			defer wg.Done()
			entry := Entry{
				ID:        fmt.Sprintf("e-%d", n),
				Timestamp: time.Now(),
				SessionID: "shared-session",
				Type:      EntryTypeDecision,
				Title:     fmt.Sprintf("Concurrent decision %d", n),
			}
			if err := s.Append(entry); err != nil {
				t.Errorf("Append goroutine %d: %v", n, err)
			}
		}(i)
	}
	wg.Wait()

	entries, err := s.ReadSession("shared-session")
	if err != nil {
		t.Fatalf("ReadSession: %v", err)
	}
	if len(entries) != goroutines {
		t.Fatalf("expected %d entries, got %d (possible corruption)", goroutines, len(entries))
	}
}

func TestValidateEntryType_ValidTypes(t *testing.T) {
	valid := []EntryType{
		EntryTypeDecision,
		EntryTypeAttemptFailed,
		EntryTypeDeferred,
		EntryTypeAssumption,
		EntryTypeQuestion,
	}
	for _, et := range valid {
		if err := ValidateEntryType(et); err != nil {
			t.Errorf("ValidateEntryType(%q) returned error: %v", et, err)
		}
	}
}

func TestValidateEntryType_InvalidType(t *testing.T) {
	if err := ValidateEntryType("bogus"); err == nil {
		t.Error("ValidateEntryType(\"bogus\") should return error")
	}
}

func contains(s, substr string) bool {
	return len(s) >= len(substr) && searchString(s, substr)
}

func searchString(s, substr string) bool {
	for i := 0; i <= len(s)-len(substr); i++ {
		if s[i:i+len(substr)] == substr {
			return true
		}
	}
	return false
}

package daemon

import (
	"bufio"
	"context"
	"encoding/json"
	"fmt"
	"net"
	"path/filepath"
	"testing"
	"time"

	"github.com/byronxlg/agentlog/internal/store"
)

func newTestDaemon(t *testing.T) *Daemon {
	t.Helper()
	dir := t.TempDir()
	cfg := Config{
		Dir:        dir,
		SocketPath: filepath.Join(dir, "test.sock"),
		PIDPath:    filepath.Join(dir, "test.pid"),
		LogPath:    filepath.Join(dir, "test.log"),
	}
	d, err := New(cfg)
	if err != nil {
		t.Fatalf("create daemon: %v", err)
	}
	return d
}

func TestHandleRequest_UnknownMethod(t *testing.T) {
	d := newTestDaemon(t)
	defer func() { _ = d.index.Close() }()

	resp := d.handleRequest(Request{Method: "nonexistent"})
	if resp.OK {
		t.Fatal("expected error response for unknown method")
	}
	if resp.Error != "unknown method: nonexistent" {
		t.Fatalf("unexpected error: %s", resp.Error)
	}
}

func TestHandleRequest_Routing(t *testing.T) {
	d := newTestDaemon(t)
	defer func() { _ = d.index.Close() }()

	methods := []string{"write", "query", "search", "get_session", "list_sessions", "create_session", "blame"}
	for _, m := range methods {
		resp := d.handleRequest(Request{Method: m})
		// All methods should be routed (not return "unknown method")
		if resp.Error != "" && resp.Error == "unknown method: "+m {
			t.Errorf("method %q was not routed", m)
		}
	}
}

func TestSessionCreateAndList(t *testing.T) {
	d := newTestDaemon(t)
	defer func() { _ = d.index.Close() }()

	if len(d.listActiveSessions()) != 0 {
		t.Fatal("expected no sessions initially")
	}

	id1 := d.createSession()
	id2 := d.createSession()

	if id1 == id2 {
		t.Fatal("session IDs should be unique")
	}

	sessions := d.listActiveSessions()
	if len(sessions) != 2 {
		t.Fatalf("expected 2 sessions, got %d", len(sessions))
	}

	found := make(map[string]bool)
	for _, s := range sessions {
		found[s] = true
	}
	if !found[id1] || !found[id2] {
		t.Fatal("missing session ID in list")
	}
}

func TestCreateSessionViaProtocol(t *testing.T) {
	d := newTestDaemon(t)
	defer func() { _ = d.index.Close() }()

	resp := d.handleRequest(Request{Method: "create_session"})
	if !resp.OK {
		t.Fatalf("create_session failed: %s", resp.Error)
	}

	var result map[string]string
	if err := json.Unmarshal(resp.Result, &result); err != nil {
		t.Fatalf("unmarshal result: %v", err)
	}
	if result["session_id"] == "" {
		t.Fatal("expected non-empty session_id")
	}
}

func TestHandleQuery_NoFilter(t *testing.T) {
	d := newTestDaemon(t)
	defer func() { _ = d.index.Close() }()

	// Create session and write entries.
	sessionID := d.createSession()
	for i := 0; i < 3; i++ {
		entry := store.Entry{
			ID:        fmt.Sprintf("e%d", i+1),
			Timestamp: time.Date(2026, 1, 15, i, 0, 0, 0, time.UTC),
			SessionID: sessionID,
			Type:      store.EntryTypeDecision,
			Title:     fmt.Sprintf("Decision %d", i+1),
		}
		if err := d.store.Append(entry); err != nil {
			t.Fatalf("append: %v", err)
		}
		if err := d.index.Insert(entry); err != nil {
			t.Fatalf("insert index: %v", err)
		}
	}

	// Query with no filters should return entries.
	params, _ := json.Marshal(QueryParams{})
	resp := d.handleRequest(Request{Method: "query", Params: params})
	if !resp.OK {
		t.Fatalf("no-filter query failed: %s", resp.Error)
	}

	var entries []store.Entry
	if err := json.Unmarshal(resp.Result, &entries); err != nil {
		t.Fatalf("unmarshal entries: %v", err)
	}
	if len(entries) != 3 {
		t.Fatalf("expected 3 entries, got %d", len(entries))
	}
	// Should be ordered most recent first.
	if entries[0].ID != "e3" {
		t.Errorf("expected most recent first (e3), got %q", entries[0].ID)
	}

	// With limit.
	params, _ = json.Marshal(QueryParams{Limit: 2})
	resp = d.handleRequest(Request{Method: "query", Params: params})
	if !resp.OK {
		t.Fatalf("limited query failed: %s", resp.Error)
	}
	if err := json.Unmarshal(resp.Result, &entries); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}
	if len(entries) != 2 {
		t.Fatalf("expected 2 entries with limit, got %d", len(entries))
	}

	// With offset.
	params, _ = json.Marshal(QueryParams{Limit: 2, Offset: 2})
	resp = d.handleRequest(Request{Method: "query", Params: params})
	if !resp.OK {
		t.Fatalf("offset query failed: %s", resp.Error)
	}
	if err := json.Unmarshal(resp.Result, &entries); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}
	if len(entries) != 1 {
		t.Fatalf("expected 1 entry with offset, got %d", len(entries))
	}
}

func sendRequest(t *testing.T, socketPath string, req Request) Response {
	t.Helper()
	conn, err := net.Dial("unix", socketPath)
	if err != nil {
		t.Fatalf("dial socket: %v", err)
	}
	defer func() { _ = conn.Close() }()

	data, err := json.Marshal(req)
	if err != nil {
		t.Fatalf("marshal request: %v", err)
	}
	_, err = conn.Write(append(data, '\n'))
	if err != nil {
		t.Fatalf("write request: %v", err)
	}

	scanner := bufio.NewScanner(conn)
	if !scanner.Scan() {
		t.Fatal("no response from daemon")
	}

	var resp Response
	if err := json.Unmarshal(scanner.Bytes(), &resp); err != nil {
		t.Fatalf("unmarshal response: %v", err)
	}
	return resp
}

func TestIntegration(t *testing.T) {
	dir := t.TempDir()
	socketPath := filepath.Join(dir, "test.sock")

	cfg := Config{
		Dir:        dir,
		SocketPath: socketPath,
		PIDPath:    filepath.Join(dir, "test.pid"),
		LogPath:    filepath.Join(dir, "test.log"),
	}

	d, err := New(cfg)
	if err != nil {
		t.Fatalf("create daemon: %v", err)
	}

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	errCh := make(chan error, 1)
	go func() {
		errCh <- d.Run(ctx)
	}()

	// Wait for socket to be available
	var conn net.Conn
	for i := 0; i < 50; i++ {
		conn, err = net.Dial("unix", socketPath)
		if err == nil {
			_ = conn.Close()
			break
		}
		time.Sleep(10 * time.Millisecond)
	}
	if err != nil {
		t.Fatalf("daemon did not start: %v", err)
	}

	// Create session
	resp := sendRequest(t, socketPath, Request{Method: "create_session"})
	if !resp.OK {
		t.Fatalf("create_session failed: %s", resp.Error)
	}
	var sessionResult map[string]string
	if err := json.Unmarshal(resp.Result, &sessionResult); err != nil {
		t.Fatalf("unmarshal session result: %v", err)
	}
	sessionID := sessionResult["session_id"]

	// Write entry
	writeParams, _ := json.Marshal(WriteParams{
		Entry: EntryParams{
			SessionID: sessionID,
			Type:      "decision",
			Title:     "use Unix sockets",
			Body:      "simpler than HTTP for local IPC",
			Tags:      []string{"architecture"},
			FileRefs:  []string{"internal/daemon/daemon.go"},
		},
	})
	resp = sendRequest(t, socketPath, Request{Method: "write", Params: writeParams})
	if !resp.OK {
		t.Fatalf("write failed: %s", resp.Error)
	}

	var writtenEntry store.Entry
	if err := json.Unmarshal(resp.Result, &writtenEntry); err != nil {
		t.Fatalf("unmarshal written entry: %v", err)
	}
	if writtenEntry.ID == "" {
		t.Fatal("expected auto-generated entry ID")
	}
	if writtenEntry.Title != "use Unix sockets" {
		t.Fatalf("unexpected title: %s", writtenEntry.Title)
	}

	// Query by session
	queryParams, _ := json.Marshal(QueryParams{SessionID: sessionID})
	resp = sendRequest(t, socketPath, Request{Method: "query", Params: queryParams})
	if !resp.OK {
		t.Fatalf("query failed: %s", resp.Error)
	}
	var entries []store.Entry
	if err := json.Unmarshal(resp.Result, &entries); err != nil {
		t.Fatalf("unmarshal entries: %v", err)
	}
	if len(entries) != 1 {
		t.Fatalf("expected 1 entry, got %d", len(entries))
	}

	// Search
	searchParams, _ := json.Marshal(SearchParams{Query: "Unix sockets"})
	resp = sendRequest(t, socketPath, Request{Method: "search", Params: searchParams})
	if !resp.OK {
		t.Fatalf("search failed: %s", resp.Error)
	}
	if err := json.Unmarshal(resp.Result, &entries); err != nil {
		t.Fatalf("unmarshal search results: %v", err)
	}
	if len(entries) != 1 {
		t.Fatalf("expected 1 search result, got %d", len(entries))
	}

	// Blame
	blameParams, _ := json.Marshal(BlameParams{FilePath: "internal/daemon/daemon.go"})
	resp = sendRequest(t, socketPath, Request{Method: "blame", Params: blameParams})
	if !resp.OK {
		t.Fatalf("blame failed: %s", resp.Error)
	}
	if err := json.Unmarshal(resp.Result, &entries); err != nil {
		t.Fatalf("unmarshal blame results: %v", err)
	}
	if len(entries) != 1 {
		t.Fatalf("expected 1 blame result, got %d", len(entries))
	}

	// List sessions (from store, not in-memory)
	resp = sendRequest(t, socketPath, Request{Method: "list_sessions"})
	if !resp.OK {
		t.Fatalf("list_sessions failed: %s", resp.Error)
	}
	var sessions []string
	if err := json.Unmarshal(resp.Result, &sessions); err != nil {
		t.Fatalf("unmarshal sessions: %v", err)
	}
	if len(sessions) != 1 {
		t.Fatalf("expected 1 session, got %d", len(sessions))
	}

	// Get session
	getSessionParams, _ := json.Marshal(GetSessionParams{SessionID: sessionID})
	resp = sendRequest(t, socketPath, Request{Method: "get_session", Params: getSessionParams})
	if !resp.OK {
		t.Fatalf("get_session failed: %s", resp.Error)
	}
	if err := json.Unmarshal(resp.Result, &entries); err != nil {
		t.Fatalf("unmarshal get_session results: %v", err)
	}
	if len(entries) != 1 {
		t.Fatalf("expected 1 entry from get_session, got %d", len(entries))
	}

	// Shutdown
	cancel()
	if err := <-errCh; err != nil {
		t.Fatalf("daemon run error: %v", err)
	}
}

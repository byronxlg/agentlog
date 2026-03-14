// Package store implements the append-only JSONL log store for decision entries.
package store

import (
	"bufio"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"sync"
)

// Store manages reading and writing decision entries as JSONL files.
// Each session gets its own file: <store_dir>/log/<session_id>.jsonl
type Store struct {
	dir string
	mu  sync.Mutex
}

// New creates a Store rooted at the given directory.
// The log subdirectory is created on first write, not at construction time.
func New(dir string) *Store {
	return &Store{dir: dir}
}

// logDir returns the path to the log subdirectory.
func (s *Store) logDir() string {
	return filepath.Join(s.dir, "log")
}

// sessionPath returns the JSONL file path for a given session ID.
func (s *Store) sessionPath(sessionID string) string {
	return filepath.Join(s.logDir(), sessionID+".jsonl")
}

// Append writes an entry to the JSONL file for its session.
// It creates the log directory and session file if they do not exist.
// Writes are serialized with a mutex to prevent concurrent corruption.
func (s *Store) Append(entry Entry) error {
	if err := ValidateEntryType(entry.Type); err != nil {
		return err
	}
	if entry.ID == "" {
		return fmt.Errorf("entry ID must not be empty")
	}
	if entry.SessionID == "" {
		return fmt.Errorf("entry session_id must not be empty")
	}
	if entry.Title == "" {
		return fmt.Errorf("entry title must not be empty")
	}

	data, err := json.Marshal(entry)
	if err != nil {
		return fmt.Errorf("marshal entry: %w", err)
	}

	s.mu.Lock()
	defer s.mu.Unlock()

	if err := os.MkdirAll(s.logDir(), 0o755); err != nil {
		return fmt.Errorf("create log directory: %w", err)
	}

	f, err := os.OpenFile(s.sessionPath(entry.SessionID), os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0o644)
	if err != nil {
		return fmt.Errorf("open session file: %w", err)
	}
	defer f.Close()

	// File-level lock to prevent corruption from other processes.
	if err := lockFile(f); err != nil {
		return fmt.Errorf("lock session file: %w", err)
	}
	defer unlockFile(f)

	data = append(data, '\n')
	if _, err := f.Write(data); err != nil {
		return fmt.Errorf("write entry: %w", err)
	}

	return nil
}

// ReadSession reads all entries from a single session's JSONL file.
func (s *Store) ReadSession(sessionID string) ([]Entry, error) {
	return readEntriesFromFile(s.sessionPath(sessionID))
}

// ReadAll reads entries from all session JSONL files in the log directory.
func (s *Store) ReadAll() ([]Entry, error) {
	pattern := filepath.Join(s.logDir(), "*.jsonl")
	files, err := filepath.Glob(pattern)
	if err != nil {
		return nil, fmt.Errorf("glob session files: %w", err)
	}

	var all []Entry
	for _, f := range files {
		entries, err := readEntriesFromFile(f)
		if err != nil {
			return nil, fmt.Errorf("read %s: %w", filepath.Base(f), err)
		}
		all = append(all, entries...)
	}
	return all, nil
}

// ListSessions returns the session IDs that have log files.
func (s *Store) ListSessions() ([]string, error) {
	pattern := filepath.Join(s.logDir(), "*.jsonl")
	files, err := filepath.Glob(pattern)
	if err != nil {
		return nil, fmt.Errorf("glob session files: %w", err)
	}

	sessions := make([]string, 0, len(files))
	for _, f := range files {
		name := filepath.Base(f)
		sessions = append(sessions, strings.TrimSuffix(name, ".jsonl"))
	}
	return sessions, nil
}

// readEntriesFromFile reads and deserializes all entries from a JSONL file.
func readEntriesFromFile(path string) ([]Entry, error) {
	f, err := os.Open(path)
	if err != nil {
		return nil, err
	}
	defer f.Close()

	var entries []Entry
	scanner := bufio.NewScanner(f)
	lineNum := 0
	for scanner.Scan() {
		lineNum++
		line := scanner.Bytes()
		if len(line) == 0 {
			continue
		}
		var entry Entry
		if err := json.Unmarshal(line, &entry); err != nil {
			return nil, fmt.Errorf("line %d: %w", lineNum, err)
		}
		entries = append(entries, entry)
	}
	if err := scanner.Err(); err != nil {
		return nil, fmt.Errorf("scan file: %w", err)
	}
	return entries, nil
}

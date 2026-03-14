package index

import (
	"fmt"
	"strings"
	"time"

	"github.com/byronxlg/agentlog/internal/store"
)

// QueryByTimeRange returns entries with timestamps in [start, end].
func (idx *Index) QueryByTimeRange(start, end time.Time) ([]store.Entry, error) {
	rows, err := idx.db.Query(
		`SELECT id, timestamp, session_id, type, title, body FROM entries WHERE timestamp >= ? AND timestamp <= ? ORDER BY timestamp`,
		start.UTC().Format(time.RFC3339Nano),
		end.UTC().Format(time.RFC3339Nano),
	)
	if err != nil {
		return nil, fmt.Errorf("query by time range: %w", err)
	}
	defer func() { _ = rows.Close() }()
	return idx.scanEntries(rows)
}

// QueryByType returns entries matching the given type.
func (idx *Index) QueryByType(entryType store.EntryType) ([]store.Entry, error) {
	rows, err := idx.db.Query(
		`SELECT id, timestamp, session_id, type, title, body FROM entries WHERE type = ? ORDER BY timestamp`,
		string(entryType),
	)
	if err != nil {
		return nil, fmt.Errorf("query by type: %w", err)
	}
	defer func() { _ = rows.Close() }()
	return idx.scanEntries(rows)
}

// QueryBySession returns entries for a given session ID.
func (idx *Index) QueryBySession(sessionID string) ([]store.Entry, error) {
	rows, err := idx.db.Query(
		`SELECT id, timestamp, session_id, type, title, body FROM entries WHERE session_id = ? ORDER BY timestamp`,
		sessionID,
	)
	if err != nil {
		return nil, fmt.Errorf("query by session: %w", err)
	}
	defer func() { _ = rows.Close() }()
	return idx.scanEntries(rows)
}

// QueryByTags returns entries that have ALL specified tags (AND semantics).
func (idx *Index) QueryByTags(tags []string) ([]store.Entry, error) {
	if len(tags) == 0 {
		return nil, nil
	}

	placeholders := make([]string, len(tags))
	args := make([]any, len(tags))
	for i, tag := range tags {
		placeholders[i] = "?"
		args[i] = tag
	}
	args = append(args, len(tags))

	query := fmt.Sprintf(
		`SELECT e.id, e.timestamp, e.session_id, e.type, e.title, e.body
		 FROM entries e
		 JOIN entry_tags t ON e.id = t.entry_id
		 WHERE t.tag IN (%s)
		 GROUP BY e.id
		 HAVING COUNT(DISTINCT t.tag) = ?
		 ORDER BY e.timestamp`,
		strings.Join(placeholders, ", "),
	)

	rows, err := idx.db.Query(query, args...)
	if err != nil {
		return nil, fmt.Errorf("query by tags: %w", err)
	}
	defer func() { _ = rows.Close() }()
	return idx.scanEntries(rows)
}

// QueryByFilePath returns entries that reference the given file path.
func (idx *Index) QueryByFilePath(filePath string) ([]store.Entry, error) {
	rows, err := idx.db.Query(
		`SELECT e.id, e.timestamp, e.session_id, e.type, e.title, e.body
		 FROM entries e
		 JOIN entry_file_refs f ON e.id = f.entry_id
		 WHERE f.file_path = ?
		 ORDER BY e.timestamp`,
		filePath,
	)
	if err != nil {
		return nil, fmt.Errorf("query by file path: %w", err)
	}
	defer func() { _ = rows.Close() }()
	return idx.scanEntries(rows)
}

// Search performs a full-text search across title and body using FTS5.
func (idx *Index) Search(query string) ([]store.Entry, error) {
	rows, err := idx.db.Query(
		`SELECT e.id, e.timestamp, e.session_id, e.type, e.title, e.body
		 FROM entries e
		 JOIN entries_fts fts ON e.rowid = fts.rowid
		 WHERE entries_fts MATCH ?
		 ORDER BY rank`,
		query,
	)
	if err != nil {
		return nil, fmt.Errorf("full-text search: %w", err)
	}
	defer func() { _ = rows.Close() }()
	return idx.scanEntries(rows)
}

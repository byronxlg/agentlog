// Package index implements the SQLite index for fast queries across decision entries.
// The index is a derived, rebuildable cache over the JSONL log store.
package index

import (
	"database/sql"
	"fmt"
	"time"

	_ "modernc.org/sqlite" // register SQLite driver

	"github.com/byronxlg/agentlog/internal/store"
)

// Index provides fast read access to decision entries via SQLite.
type Index struct {
	db *sql.DB
}

// Open creates or opens the SQLite index at dbPath and runs schema migrations.
func Open(dbPath string) (*Index, error) {
	dsn := dbPath + "?_pragma=journal_mode(wal)&_pragma=foreign_keys(on)"
	db, err := sql.Open("sqlite", dsn)
	if err != nil {
		return nil, fmt.Errorf("open index database: %w", err)
	}

	idx := &Index{db: db}
	if err := idx.migrate(); err != nil {
		_ = db.Close()
		return nil, fmt.Errorf("migrate index schema: %w", err)
	}
	return idx, nil
}

// Close closes the underlying database connection.
func (idx *Index) Close() error {
	return idx.db.Close()
}

func (idx *Index) migrate() error {
	stmts := []string{
		`CREATE TABLE IF NOT EXISTS entries (
			id         TEXT PRIMARY KEY,
			timestamp  TEXT NOT NULL,
			session_id TEXT NOT NULL,
			type       TEXT NOT NULL,
			title      TEXT NOT NULL,
			body       TEXT NOT NULL DEFAULT ''
		)`,
		`CREATE TABLE IF NOT EXISTS entry_tags (
			entry_id TEXT NOT NULL REFERENCES entries(id) ON DELETE CASCADE,
			tag      TEXT NOT NULL,
			PRIMARY KEY (entry_id, tag)
		)`,
		`CREATE TABLE IF NOT EXISTS entry_file_refs (
			entry_id  TEXT NOT NULL REFERENCES entries(id) ON DELETE CASCADE,
			file_path TEXT NOT NULL,
			PRIMARY KEY (entry_id, file_path)
		)`,
		`CREATE VIRTUAL TABLE IF NOT EXISTS entries_fts USING fts5(
			title,
			body,
			content=entries,
			content_rowid=rowid
		)`,
		`CREATE TRIGGER IF NOT EXISTS entries_ai AFTER INSERT ON entries BEGIN
			INSERT INTO entries_fts(rowid, title, body) VALUES (new.rowid, new.title, new.body);
		END`,
		`CREATE TRIGGER IF NOT EXISTS entries_ad AFTER DELETE ON entries BEGIN
			INSERT INTO entries_fts(entries_fts, rowid, title, body) VALUES ('delete', old.rowid, old.title, old.body);
		END`,
		`CREATE TRIGGER IF NOT EXISTS entries_au AFTER UPDATE ON entries BEGIN
			INSERT INTO entries_fts(entries_fts, rowid, title, body) VALUES ('delete', old.rowid, old.title, old.body);
			INSERT INTO entries_fts(rowid, title, body) VALUES (new.rowid, new.title, new.body);
		END`,
	}

	for _, stmt := range stmts {
		if _, err := idx.db.Exec(stmt); err != nil {
			return fmt.Errorf("exec %q: %w", stmt[:40], err)
		}
	}
	return nil
}

func (idx *Index) scanEntries(rows *sql.Rows) ([]store.Entry, error) {
	var entries []store.Entry
	for rows.Next() {
		var e store.Entry
		var ts string
		if err := rows.Scan(&e.ID, &ts, &e.SessionID, &e.Type, &e.Title, &e.Body); err != nil {
			return nil, fmt.Errorf("scan entry row: %w", err)
		}
		t, err := time.Parse(time.RFC3339Nano, ts)
		if err != nil {
			return nil, fmt.Errorf("parse timestamp %q: %w", ts, err)
		}
		e.Timestamp = t

		tags, err := idx.loadTags(e.ID)
		if err != nil {
			return nil, err
		}
		e.Tags = tags

		refs, err := idx.loadFileRefs(e.ID)
		if err != nil {
			return nil, err
		}
		e.FileRefs = refs

		entries = append(entries, e)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("iterate rows: %w", err)
	}
	return entries, nil
}

func (idx *Index) loadTags(entryID string) ([]string, error) {
	rows, err := idx.db.Query(`SELECT tag FROM entry_tags WHERE entry_id = ? ORDER BY tag`, entryID)
	if err != nil {
		return nil, fmt.Errorf("load tags for %q: %w", entryID, err)
	}
	defer func() { _ = rows.Close() }()

	var tags []string
	for rows.Next() {
		var tag string
		if err := rows.Scan(&tag); err != nil {
			return nil, fmt.Errorf("scan tag: %w", err)
		}
		tags = append(tags, tag)
	}
	return tags, rows.Err()
}

func (idx *Index) loadFileRefs(entryID string) ([]string, error) {
	rows, err := idx.db.Query(`SELECT file_path FROM entry_file_refs WHERE entry_id = ? ORDER BY file_path`, entryID)
	if err != nil {
		return nil, fmt.Errorf("load file refs for %q: %w", entryID, err)
	}
	defer func() { _ = rows.Close() }()

	var refs []string
	for rows.Next() {
		var ref string
		if err := rows.Scan(&ref); err != nil {
			return nil, fmt.Errorf("scan file ref: %w", err)
		}
		refs = append(refs, ref)
	}
	return refs, rows.Err()
}

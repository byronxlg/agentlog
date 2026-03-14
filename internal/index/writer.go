package index

import (
	"fmt"
	"time"

	"github.com/byronxlg/agentlog/internal/store"
)

// Insert adds an entry to the index, replacing any existing entry with the same ID.
func (idx *Index) Insert(entry store.Entry) error {
	tx, err := idx.db.Begin()
	if err != nil {
		return fmt.Errorf("begin transaction: %w", err)
	}
	defer func() { _ = tx.Rollback() }()

	_, err = tx.Exec(
		`INSERT OR REPLACE INTO entries (id, timestamp, session_id, type, title, body) VALUES (?, ?, ?, ?, ?, ?)`,
		entry.ID,
		entry.Timestamp.UTC().Format(time.RFC3339Nano),
		entry.SessionID,
		string(entry.Type),
		entry.Title,
		entry.Body,
	)
	if err != nil {
		return fmt.Errorf("insert entry: %w", err)
	}

	if _, err := tx.Exec(`DELETE FROM entry_tags WHERE entry_id = ?`, entry.ID); err != nil {
		return fmt.Errorf("clear tags: %w", err)
	}
	for _, tag := range entry.Tags {
		if _, err := tx.Exec(`INSERT INTO entry_tags (entry_id, tag) VALUES (?, ?)`, entry.ID, tag); err != nil {
			return fmt.Errorf("insert tag %q: %w", tag, err)
		}
	}

	if _, err := tx.Exec(`DELETE FROM entry_file_refs WHERE entry_id = ?`, entry.ID); err != nil {
		return fmt.Errorf("clear file refs: %w", err)
	}
	for _, fp := range entry.FileRefs {
		if _, err := tx.Exec(`INSERT INTO entry_file_refs (entry_id, file_path) VALUES (?, ?)`, entry.ID, fp); err != nil {
			return fmt.Errorf("insert file ref %q: %w", fp, err)
		}
	}

	return tx.Commit()
}

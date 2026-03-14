package index

import (
	"fmt"

	"github.com/byronxlg/agentlog/internal/store"
)

// Rebuild drops all index data and re-indexes every entry from the JSONL store.
func (idx *Index) Rebuild(s *store.Store) error {
	dropStmts := []string{
		`DROP TRIGGER IF EXISTS entries_au`,
		`DROP TRIGGER IF EXISTS entries_ad`,
		`DROP TRIGGER IF EXISTS entries_ai`,
		`DROP TABLE IF EXISTS entries_fts`,
		`DROP TABLE IF EXISTS entry_file_refs`,
		`DROP TABLE IF EXISTS entry_tags`,
		`DROP TABLE IF EXISTS entries`,
	}
	for _, stmt := range dropStmts {
		if _, err := idx.db.Exec(stmt); err != nil {
			return fmt.Errorf("drop tables: %w", err)
		}
	}

	if err := idx.migrate(); err != nil {
		return fmt.Errorf("recreate schema: %w", err)
	}

	entries, err := s.ReadAll()
	if err != nil {
		return fmt.Errorf("read all entries from store: %w", err)
	}

	for _, entry := range entries {
		if err := idx.Insert(entry); err != nil {
			return fmt.Errorf("index entry %q: %w", entry.ID, err)
		}
	}
	return nil
}

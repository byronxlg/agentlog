// Package store implements the append-only JSONL log store for decision entries.
package store

import (
	"fmt"
	"time"
)

// EntryType represents the kind of decision being logged.
type EntryType string

const (
	EntryTypeDecision      EntryType = "decision"
	EntryTypeAttemptFailed EntryType = "attempt_failed"
	EntryTypeDeferred      EntryType = "deferred"
	EntryTypeAssumption    EntryType = "assumption"
	EntryTypeQuestion      EntryType = "question"
)

// validEntryTypes is the set of allowed entry types.
var validEntryTypes = map[EntryType]bool{
	EntryTypeDecision:      true,
	EntryTypeAttemptFailed: true,
	EntryTypeDeferred:      true,
	EntryTypeAssumption:    true,
	EntryTypeQuestion:      true,
}

// ValidateEntryType returns an error if the given type is not one of the allowed values.
func ValidateEntryType(t EntryType) error {
	if !validEntryTypes[t] {
		return fmt.Errorf("invalid entry type %q: must be one of decision, attempt_failed, deferred, assumption, question", t)
	}
	return nil
}

// Entry represents a single decision log entry.
type Entry struct {
	ID        string    `json:"id"`
	Timestamp time.Time `json:"timestamp"`
	SessionID string    `json:"session_id"`
	Type      EntryType `json:"type"`
	Title     string    `json:"title"`
	Body      string    `json:"body,omitempty"`
	Tags      []string  `json:"tags,omitempty"`
	FileRefs  []string  `json:"file_refs,omitempty"`
}

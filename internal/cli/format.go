package cli

import (
	"fmt"
	"strings"
	"time"

	"github.com/byronxlg/agentlog/internal/store"
)

// FormatTimestamp formats a time value for display.
func FormatTimestamp(t time.Time) string {
	return t.Format("2006-01-02 15:04:05")
}

// ShortSession returns the first 8 characters of a session ID.
func ShortSession(sessionID string) string {
	if len(sessionID) <= 8 {
		return sessionID
	}
	return sessionID[:8]
}

// FormatEntry formats a single entry for display.
// When verbose is true, the body is included indented below the header line.
func FormatEntry(entry store.Entry, verbose bool) string {
	line := fmt.Sprintf("%s [%s] (%s) %s",
		FormatTimestamp(entry.Timestamp),
		entry.Type,
		ShortSession(entry.SessionID),
		entry.Title,
	)

	if verbose && entry.Body != "" {
		indented := indentBody(entry.Body, "    ")
		line += "\n" + indented
	}

	return line
}

func indentBody(body, prefix string) string {
	lines := strings.Split(body, "\n")
	for i, l := range lines {
		lines[i] = prefix + l
	}
	return strings.Join(lines, "\n")
}

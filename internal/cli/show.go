package cli

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"

	"github.com/byronxlg/agentlog/internal/daemon"
	"github.com/byronxlg/agentlog/internal/store"
)

// ShowOptions holds the configuration for the show command.
type ShowOptions struct {
	Dir       string
	SessionID string
}

// Show displays all entries for a session, supporting partial session ID matching.
func Show(opts ShowOptions) error {
	socketPath := filepath.Join(opts.Dir, "agentlogd.sock")

	listReq := daemon.Request{Method: "list_sessions"}
	listResp, err := SendRequest(socketPath, listReq)
	if err != nil {
		return fmt.Errorf("daemon is not running (could not connect to %s)", socketPath)
	}
	if !listResp.OK {
		return fmt.Errorf("daemon error: %s", listResp.Error)
	}

	var sessions []string
	if err := json.Unmarshal(listResp.Result, &sessions); err != nil {
		return fmt.Errorf("parse sessions: %w", err)
	}

	fullID, err := matchSession(opts.SessionID, sessions)
	if err != nil {
		return err
	}

	params, err := json.Marshal(daemon.GetSessionParams{SessionID: fullID})
	if err != nil {
		return fmt.Errorf("marshal params: %w", err)
	}

	getReq := daemon.Request{Method: "get_session", Params: params}
	getResp, err := SendRequest(socketPath, getReq)
	if err != nil {
		return fmt.Errorf("send get_session request: %w", err)
	}
	if !getResp.OK {
		return fmt.Errorf("daemon error: %s", getResp.Error)
	}

	var entries []store.Entry
	if err := json.Unmarshal(getResp.Result, &entries); err != nil {
		return fmt.Errorf("parse entries: %w", err)
	}

	sort.Slice(entries, func(i, j int) bool {
		return entries[i].Timestamp.Before(entries[j].Timestamp)
	})

	printSessionOutput(entries, fullID)
	return nil
}

func printSessionOutput(entries []store.Entry, sessionID string) {
	started := "N/A"
	if len(entries) > 0 {
		started = FormatTimestamp(entries[0].Timestamp)
	}

	_, _ = fmt.Fprintf(os.Stdout, "Session: %s\n", sessionID)
	_, _ = fmt.Fprintf(os.Stdout, "Started: %s\n", started)
	_, _ = fmt.Fprintf(os.Stdout, "Entries: %d\n", len(entries))
	_, _ = fmt.Fprintln(os.Stdout)

	for _, entry := range entries {
		_, _ = fmt.Fprintln(os.Stdout, FormatEntryDetail(entry))
		_, _ = fmt.Fprintln(os.Stdout)
	}
}

// matchSession finds a single session ID matching the given prefix.
// Returns an error if zero or multiple sessions match.
func matchSession(prefix string, sessions []string) (string, error) {
	var matches []string
	for _, s := range sessions {
		if strings.HasPrefix(s, prefix) {
			matches = append(matches, s)
		}
	}

	switch len(matches) {
	case 0:
		return "", fmt.Errorf("no session found matching %q", prefix)
	case 1:
		return matches[0], nil
	default:
		return "", fmt.Errorf("ambiguous session ID %q, matches: %s", prefix, strings.Join(matches, ", "))
	}
}

// FormatEntryDetail formats a single entry with all fields for detailed display.
func FormatEntryDetail(entry store.Entry) string {
	var b strings.Builder

	fmt.Fprintf(&b, "%s [%s] %s", FormatTimestamp(entry.Timestamp), entry.Type, entry.Title)

	if entry.Body != "" {
		b.WriteString("\n")
		b.WriteString(indentBody(entry.Body, "  "))
	}

	if len(entry.Tags) > 0 {
		fmt.Fprintf(&b, "\n  Tags: %s", strings.Join(entry.Tags, ", "))
	}

	if len(entry.FileRefs) > 0 {
		fmt.Fprintf(&b, "\n  Files: %s", strings.Join(entry.FileRefs, ", "))
	}

	return b.String()
}

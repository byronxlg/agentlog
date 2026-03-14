package cli

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"sort"

	"github.com/byronxlg/agentlog/internal/daemon"
	"github.com/byronxlg/agentlog/internal/store"
)

// BlameOptions holds the configuration for the blame command.
type BlameOptions struct {
	Dir     string
	File    string
	Verbose bool
}

// Blame queries the daemon for all entries referencing a file and prints them.
func Blame(opts BlameOptions) error {
	absPath, err := filepath.Abs(opts.File)
	if err != nil {
		return fmt.Errorf("resolve file path: %w", err)
	}

	params, err := json.Marshal(daemon.BlameParams{FilePath: absPath})
	if err != nil {
		return fmt.Errorf("marshal params: %w", err)
	}

	req := daemon.Request{
		Method: "blame",
		Params: params,
	}

	socketPath := filepath.Join(opts.Dir, "agentlogd.sock")
	resp, err := SendRequest(socketPath, req)
	if err != nil {
		return fmt.Errorf("daemon is not running (could not connect to %s)", socketPath)
	}

	if !resp.OK {
		return fmt.Errorf("daemon error: %s", resp.Error)
	}

	var entries []store.Entry
	if err := json.Unmarshal(resp.Result, &entries); err != nil {
		return fmt.Errorf("parse response: %w", err)
	}

	sort.Slice(entries, func(i, j int) bool {
		return entries[i].Timestamp.After(entries[j].Timestamp)
	})

	if len(entries) == 0 {
		_, _ = fmt.Fprintf(os.Stdout, "no decisions reference %s\n", absPath)
		return nil
	}

	for _, entry := range entries {
		_, _ = fmt.Fprintln(os.Stdout, FormatEntry(entry, opts.Verbose))
	}

	return nil
}

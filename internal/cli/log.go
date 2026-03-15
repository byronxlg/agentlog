package cli

import (
	"encoding/json"
	"flag"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"sort"
	"time"

	"github.com/byronxlg/agentlog/internal/daemon"
	"github.com/byronxlg/agentlog/internal/store"
)

// LogOptions holds parsed flags for the log command.
type LogOptions struct {
	Dir     string
	Type    string
	Session string
	Tag     string
	Since   string
	Until   string
	File    string
	Verbose bool
	Limit   int
	Offset  int
}

// ParseLogArgs parses command-line arguments for the log subcommand.
func ParseLogArgs(dir string, args []string) (LogOptions, error) {
	fs := flag.NewFlagSet("log", flag.ContinueOnError)
	fs.SetOutput(io.Discard)

	opts := LogOptions{Dir: dir}
	fs.StringVar(&opts.Type, "type", "", "filter by entry type")
	fs.StringVar(&opts.Session, "session", "", "filter by session ID")
	fs.StringVar(&opts.Tag, "tag", "", "filter by tag")
	fs.StringVar(&opts.Since, "since", "", "show entries after this time (RFC3339 or relative like 1h, 7d)")
	fs.StringVar(&opts.Until, "until", "", "show entries before this time (RFC3339 or relative like 1h, 7d)")
	fs.StringVar(&opts.File, "file", "", "filter by referenced file path")
	fs.BoolVar(&opts.Verbose, "verbose", false, "show entry body text inline")
	fs.IntVar(&opts.Limit, "limit", 50, "maximum number of entries to display")
	fs.IntVar(&opts.Offset, "offset", 0, "number of entries to skip")

	if err := fs.Parse(args); err != nil {
		return LogOptions{}, err
	}

	if opts.Limit < 0 {
		return LogOptions{}, fmt.Errorf("--limit must be non-negative")
	}
	if opts.Offset < 0 {
		return LogOptions{}, fmt.Errorf("--offset must be non-negative")
	}

	return opts, nil
}

// Log queries the daemon for entries matching the given filters and prints them.
func Log(opts LogOptions) error {
	now := time.Now()

	sinceTime, err := ParseTimeFlag(opts.Since, now)
	if err != nil {
		return fmt.Errorf("invalid --since value: %w", err)
	}

	untilTime, err := ParseTimeFlag(opts.Until, now)
	if err != nil {
		return fmt.Errorf("invalid --until value: %w", err)
	}

	qp := daemon.QueryParams{
		Type:      opts.Type,
		SessionID: opts.Session,
		FilePath:  opts.File,
	}

	if opts.Tag != "" {
		qp.Tags = []string{opts.Tag}
	}

	if !sinceTime.IsZero() {
		qp.Start = sinceTime.Format(time.RFC3339)
	}
	if !untilTime.IsZero() {
		qp.End = untilTime.Format(time.RFC3339)
	}

	paramsJSON, err := json.Marshal(qp)
	if err != nil {
		return fmt.Errorf("marshal params: %w", err)
	}

	req := daemon.Request{
		Method: "query",
		Params: paramsJSON,
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

	// Apply pagination
	if opts.Offset > 0 {
		if opts.Offset >= len(entries) {
			entries = nil
		} else {
			entries = entries[opts.Offset:]
		}
	}
	if opts.Limit > 0 && len(entries) > opts.Limit {
		entries = entries[:opts.Limit]
	}

	if len(entries) == 0 {
		_, _ = fmt.Fprintln(os.Stdout, "no entries found")
		return nil
	}

	for _, entry := range entries {
		_, _ = fmt.Fprintln(os.Stdout, FormatEntry(entry, opts.Verbose))
	}

	return nil
}

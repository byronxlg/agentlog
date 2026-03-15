package cli

import (
	"encoding/json"
	"flag"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"

	"github.com/byronxlg/agentlog/internal/daemon"
	"github.com/byronxlg/agentlog/internal/store"
)

// ContextOptions holds parsed flags for the context command.
type ContextOptions struct {
	Dir   string
	Files []string
	Topic string
	Limit int
	JSON  bool
}

// filesFlag implements flag.Value for repeatable --files flags.
type filesFlag []string

func (f *filesFlag) String() string { return strings.Join(*f, ",") }
func (f *filesFlag) Set(val string) error {
	*f = append(*f, val)
	return nil
}

// ParseContextArgs parses command-line arguments for the context subcommand.
func ParseContextArgs(dir string, args []string) (ContextOptions, error) {
	fs := flag.NewFlagSet("context", flag.ContinueOnError)
	fs.SetOutput(io.Discard)

	opts := ContextOptions{Dir: dir}
	var files filesFlag
	fs.Var(&files, "files", "file path to look up decisions for (repeatable)")
	fs.StringVar(&opts.Topic, "topic", "", "search string for full-text search")
	fs.IntVar(&opts.Limit, "limit", 10, "maximum entries to return")
	fs.BoolVar(&opts.JSON, "json", false, "output raw JSON instead of formatted text")

	if err := fs.Parse(args); err != nil {
		return ContextOptions{}, err
	}

	opts.Files = []string(files)

	if len(opts.Files) == 0 && opts.Topic == "" {
		return ContextOptions{}, fmt.Errorf("at least one of --files or --topic is required")
	}

	if opts.Limit < 0 {
		return ContextOptions{}, fmt.Errorf("--limit must be non-negative")
	}

	return opts, nil
}

// FormatContext formats entries as markdown-style text for prompt injection.
func FormatContext(entries []store.Entry) string {
	if len(entries) == 0 {
		return "No relevant decisions found."
	}

	var b strings.Builder
	b.WriteString("# Relevant decisions\n")

	for _, e := range entries {
		fmt.Fprintf(&b, "\n## [%s] %s (%s)\n", e.Type, e.Title, e.Timestamp.Format("2006-01-02T15:04:05Z"))
		if e.Body != "" {
			b.WriteString(e.Body)
			b.WriteString("\n")
		}
		if len(e.FileRefs) > 0 {
			fmt.Fprintf(&b, "Files: %s\n", strings.Join(e.FileRefs, ", "))
		}
		if len(e.Tags) > 0 {
			fmt.Fprintf(&b, "Tags: %s\n", strings.Join(e.Tags, ", "))
		}
	}

	return b.String()
}

// Context queries the daemon for contextual decisions and prints them.
func Context(opts ContextOptions) error {
	cp := daemon.ContextParams{
		Files: opts.Files,
		Topic: opts.Topic,
		Limit: opts.Limit,
	}

	params, err := json.Marshal(cp)
	if err != nil {
		return fmt.Errorf("marshal params: %w", err)
	}

	req := daemon.Request{
		Method: "context",
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

	if opts.JSON {
		enc := json.NewEncoder(os.Stdout)
		enc.SetIndent("", "  ")
		return enc.Encode(entries)
	}

	fmt.Print(FormatContext(entries))
	return nil
}

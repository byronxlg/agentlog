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
)

var validEntryTypes = map[string]bool{
	"decision":       true,
	"attempt_failed": true,
	"deferred":       true,
	"assumption":     true,
	"question":       true,
}

// WriteOptions holds parsed flags for the write command.
type WriteOptions struct {
	Dir     string
	Type    string
	Title   string
	Body    string
	Tags    string
	Files   string
	Session string
}

// ParseWriteArgs parses command-line arguments for the write subcommand.
// args should not include the "write" subcommand itself.
func ParseWriteArgs(dir string, args []string) (WriteOptions, error) {
	fs := flag.NewFlagSet("write", flag.ContinueOnError)
	fs.SetOutput(io.Discard)

	opts := WriteOptions{Dir: dir}
	fs.StringVar(&opts.Type, "type", "", "entry type (decision, attempt_failed, deferred, assumption, question)")
	fs.StringVar(&opts.Title, "title", "", "entry title")
	fs.StringVar(&opts.Body, "body", "", "entry body")
	fs.StringVar(&opts.Tags, "tags", "", "comma-separated tags")
	fs.StringVar(&opts.Files, "files", "", "comma-separated file paths")
	fs.StringVar(&opts.Session, "session", "", "session ID (creates new session if omitted)")

	if err := fs.Parse(args); err != nil {
		return WriteOptions{}, err
	}

	if opts.Type == "" {
		return WriteOptions{}, fmt.Errorf("--type is required")
	}
	if !validEntryTypes[opts.Type] {
		return WriteOptions{}, fmt.Errorf("invalid type %q: must be one of decision, attempt_failed, deferred, assumption, question", opts.Type)
	}
	if opts.Title == "" {
		return WriteOptions{}, fmt.Errorf("--title is required")
	}

	return opts, nil
}

// splitCSV splits a comma-separated string into trimmed, non-empty values.
func splitCSV(s string) []string {
	if s == "" {
		return nil
	}
	parts := strings.Split(s, ",")
	var result []string
	for _, p := range parts {
		p = strings.TrimSpace(p)
		if p != "" {
			result = append(result, p)
		}
	}
	return result
}

// createSession calls the daemon to create a new session and returns the session ID.
func createSession(socketPath string) (string, error) {
	req := daemon.Request{Method: "create_session"}
	resp, err := SendRequest(socketPath, req)
	if err != nil {
		return "", fmt.Errorf("daemon is not running (could not connect to %s)", socketPath)
	}
	if !resp.OK {
		return "", fmt.Errorf("daemon error: %s", resp.Error)
	}
	var result map[string]string
	if err := json.Unmarshal(resp.Result, &result); err != nil {
		return "", fmt.Errorf("parse session response: %w", err)
	}
	id, ok := result["session_id"]
	if !ok || id == "" {
		return "", fmt.Errorf("daemon returned no session_id")
	}
	return id, nil
}

// Write validates options, sends a write request to the daemon, and prints the entry ID.
func Write(opts WriteOptions) error {
	socketPath := filepath.Join(opts.Dir, "agentlogd.sock")

	sessionID := opts.Session
	if sessionID == "" {
		id, err := createSession(socketPath)
		if err != nil {
			return err
		}
		sessionID = id
		fmt.Fprintln(os.Stderr, sessionID)
	}

	entry := daemon.EntryParams{
		SessionID: sessionID,
		Type:      opts.Type,
		Title:     opts.Title,
		Body:      opts.Body,
		Tags:      splitCSV(opts.Tags),
		FileRefs:  splitCSV(opts.Files),
	}

	params, err := json.Marshal(daemon.WriteParams{Entry: entry})
	if err != nil {
		return fmt.Errorf("marshal params: %w", err)
	}

	req := daemon.Request{
		Method: "write",
		Params: params,
	}

	resp, err := SendRequest(socketPath, req)
	if err != nil {
		return fmt.Errorf("daemon is not running (could not connect to %s)", socketPath)
	}

	if !resp.OK {
		return fmt.Errorf("daemon error: %s", resp.Error)
	}

	var result daemon.EntryParams
	if err := json.Unmarshal(resp.Result, &result); err != nil {
		return fmt.Errorf("parse response: %w", err)
	}

	fmt.Println(result.ID)
	return nil
}

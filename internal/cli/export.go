package cli

import (
	"encoding/json"
	"flag"
	"fmt"
	"io"
	"path/filepath"

	"github.com/byronxlg/agentlog/internal/daemon"
)

// ExportOptions holds parsed flags for the export command.
type ExportOptions struct {
	Dir       string
	SessionID string
	Since     string
	Until     string
	FilePath  string
	Tag       string
	Type      string
	Format    string
	Template  string
}

// ParseExportArgs parses command-line arguments for the export subcommand.
func ParseExportArgs(dir string, args []string) (ExportOptions, error) {
	fs := flag.NewFlagSet("export", flag.ContinueOnError)
	fs.SetOutput(io.Discard)

	opts := ExportOptions{Dir: dir}
	fs.StringVar(&opts.SessionID, "session", "", "filter by session ID")
	fs.StringVar(&opts.Since, "since", "", "show entries after this time (RFC3339 or relative like 1h, 7d)")
	fs.StringVar(&opts.Until, "until", "", "show entries before this time (RFC3339 or relative like 1h, 7d)")
	fs.StringVar(&opts.FilePath, "file", "", "filter by referenced file path")
	fs.StringVar(&opts.Tag, "tag", "", "filter by tag")
	fs.StringVar(&opts.Type, "type", "", "filter by entry type")
	fs.StringVar(&opts.Format, "format", "markdown", "output format: markdown, json, text")
	fs.StringVar(&opts.Template, "template", "", "use a built-in template: pr, retro, handoff")

	if err := fs.Parse(args); err != nil {
		return ExportOptions{}, err
	}

	validFormats := map[string]bool{"markdown": true, "json": true, "text": true}
	if !validFormats[opts.Format] {
		return ExportOptions{}, fmt.Errorf("invalid format %q: must be one of markdown, json, text", opts.Format)
	}

	if opts.Template != "" {
		validTemplates := map[string]bool{"pr": true, "retro": true, "handoff": true}
		if !validTemplates[opts.Template] {
			return ExportOptions{}, fmt.Errorf("invalid template %q: must be one of pr, retro, handoff", opts.Template)
		}
	}

	return opts, nil
}

// Export sends an export request to the daemon and prints the formatted output.
func Export(opts ExportOptions) error {
	ep := daemon.ExportParams{
		SessionID: opts.SessionID,
		Since:     opts.Since,
		Until:     opts.Until,
		FilePath:  opts.FilePath,
		Tag:       opts.Tag,
		Type:      opts.Type,
		Format:    opts.Format,
		Template:  opts.Template,
	}

	params, err := json.Marshal(ep)
	if err != nil {
		return fmt.Errorf("marshal params: %w", err)
	}

	req := daemon.Request{
		Method: "export",
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

	var output string
	if err := json.Unmarshal(resp.Result, &output); err != nil {
		return fmt.Errorf("parse response: %w", err)
	}

	fmt.Print(output)
	return nil
}

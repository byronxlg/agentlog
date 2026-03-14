// Package cli implements the agentlog CLI commands.
package cli

import (
	"encoding/json"
	"flag"
	"fmt"
	"io"
	"os"
	"regexp"
	"strings"
	"time"

	"github.com/byronxlg/agentlog/internal/client"
	"github.com/byronxlg/agentlog/internal/daemon"
	"github.com/byronxlg/agentlog/internal/store"
)

// QueryConfig holds parsed flags and arguments for the query command.
type QueryConfig struct {
	SearchTerm string
	Type       string
	Session    string
	Tag        string
	Since      string
	Until      string
	File       string
	Limit      int
	SocketPath string
}

// ParseQueryArgs parses command-line arguments for the query subcommand.
// args should not include the "query" subcommand itself.
func ParseQueryArgs(args []string) (*QueryConfig, error) {
	fs := flag.NewFlagSet("query", flag.ContinueOnError)
	fs.SetOutput(io.Discard)

	cfg := &QueryConfig{}
	fs.StringVar(&cfg.Type, "type", "", "filter by entry type")
	fs.StringVar(&cfg.Session, "session", "", "filter by session ID")
	fs.StringVar(&cfg.Tag, "tag", "", "filter by tag")
	fs.StringVar(&cfg.Since, "since", "", "show entries after this time (ISO 8601 or duration like 1h, 7d)")
	fs.StringVar(&cfg.Until, "until", "", "show entries before this time (ISO 8601 or duration like 1h, 7d)")
	fs.StringVar(&cfg.File, "file", "", "filter by file reference")
	fs.IntVar(&cfg.Limit, "limit", 20, "maximum number of results")
	fs.StringVar(&cfg.SocketPath, "socket", "", "daemon socket path")

	if err := fs.Parse(args); err != nil {
		return nil, err
	}

	if fs.NArg() < 1 {
		return nil, fmt.Errorf("usage: agentlog query [flags] <search_term>")
	}
	cfg.SearchTerm = strings.Join(fs.Args(), " ")

	if cfg.Limit < 1 {
		return nil, fmt.Errorf("--limit must be at least 1")
	}

	return cfg, nil
}

// parseDuration parses a relative duration string like "1h", "7d", "30m"
// and returns the absolute time by subtracting from now.
func parseDuration(s string, now time.Time) (time.Time, error) {
	if s == "" {
		return time.Time{}, nil
	}

	// Try ISO 8601 first
	if t, err := time.Parse(time.RFC3339, s); err == nil {
		return t, nil
	}
	if t, err := time.Parse(time.RFC3339Nano, s); err == nil {
		return t, nil
	}

	// Try relative duration: number followed by unit
	if len(s) < 2 {
		return time.Time{}, fmt.Errorf("invalid time or duration: %q", s)
	}

	unit := s[len(s)-1]
	numStr := s[:len(s)-1]
	var n int
	if _, err := fmt.Sscanf(numStr, "%d", &n); err != nil {
		return time.Time{}, fmt.Errorf("invalid time or duration: %q", s)
	}
	if n < 0 {
		return time.Time{}, fmt.Errorf("invalid time or duration: %q (negative value)", s)
	}

	switch unit {
	case 'm':
		return now.Add(-time.Duration(n) * time.Minute), nil
	case 'h':
		return now.Add(-time.Duration(n) * time.Hour), nil
	case 'd':
		return now.AddDate(0, 0, -n), nil
	default:
		return time.Time{}, fmt.Errorf("invalid time or duration: %q (unknown unit %q, use m/h/d)", s, string(unit))
	}
}

// FilterEntries applies client-side filters to a slice of entries.
func FilterEntries(entries []store.Entry, cfg *QueryConfig, now time.Time) ([]store.Entry, error) {
	var sinceTime, untilTime time.Time
	var err error

	if cfg.Since != "" {
		sinceTime, err = parseDuration(cfg.Since, now)
		if err != nil {
			return nil, err
		}
	}
	if cfg.Until != "" {
		untilTime, err = parseDuration(cfg.Until, now)
		if err != nil {
			return nil, err
		}
	}

	var result []store.Entry
	for _, e := range entries {
		if cfg.Type != "" && string(e.Type) != cfg.Type {
			continue
		}
		if cfg.Session != "" && e.SessionID != cfg.Session {
			continue
		}
		if cfg.Tag != "" && !containsTag(e.Tags, cfg.Tag) {
			continue
		}
		if cfg.File != "" && !containsFile(e.FileRefs, cfg.File) {
			continue
		}
		if !sinceTime.IsZero() && e.Timestamp.Before(sinceTime) {
			continue
		}
		if !untilTime.IsZero() && e.Timestamp.After(untilTime) {
			continue
		}
		result = append(result, e)
	}

	if cfg.Limit > 0 && len(result) > cfg.Limit {
		result = result[:cfg.Limit]
	}

	return result, nil
}

func containsTag(tags []string, tag string) bool {
	for _, t := range tags {
		if t == tag {
			return true
		}
	}
	return false
}

func containsFile(refs []string, file string) bool {
	for _, r := range refs {
		if r == file {
			return true
		}
	}
	return false
}

// FormatResults formats search results for terminal display.
// searchTerm is used to highlight matching text with ANSI bold.
func FormatResults(w io.Writer, entries []store.Entry, searchTerm string) {
	if len(entries) == 0 {
		_, _ = fmt.Fprintln(w, "No results found.")
		return
	}

	for _, e := range entries {
		sessionShort := e.SessionID
		if len(sessionShort) > 8 {
			sessionShort = sessionShort[:8]
		}

		ts := e.Timestamp.Format("2006-01-02 15:04:05")
		title := highlightTerms(e.Title, searchTerm)
		_, _ = fmt.Fprintf(w, "%s  %-16s  %s  %s\n", ts, e.Type, sessionShort, title)

		if e.Body != "" {
			snippet := makeSnippet(e.Body, searchTerm, 120)
			snippet = highlightTerms(snippet, searchTerm)
			_, _ = fmt.Fprintf(w, "    %s\n", snippet)
		}
	}

	_, _ = fmt.Fprintf(w, "\n%d result(s)\n", len(entries))
}

// highlightTerms wraps occurrences of the search terms with ANSI bold codes.
func highlightTerms(text, searchTerm string) string {
	if searchTerm == "" {
		return text
	}

	// Split search term into words and highlight each
	words := strings.Fields(searchTerm)
	for _, word := range words {
		pattern := regexp.MustCompile("(?i)" + regexp.QuoteMeta(word))
		text = pattern.ReplaceAllStringFunc(text, func(match string) string {
			return "\033[1m" + match + "\033[0m"
		})
	}
	return text
}

// makeSnippet extracts a relevant snippet from body text around the search term.
func makeSnippet(body, searchTerm string, maxLen int) string {
	if len(body) <= maxLen {
		return strings.ReplaceAll(body, "\n", " ")
	}

	// Find the first occurrence of any search word
	bodyLower := strings.ToLower(body)
	words := strings.Fields(strings.ToLower(searchTerm))
	bestIdx := -1
	for _, word := range words {
		idx := strings.Index(bodyLower, word)
		if idx >= 0 && (bestIdx < 0 || idx < bestIdx) {
			bestIdx = idx
		}
	}

	if bestIdx < 0 {
		// No match found in body, just take the beginning
		snippet := body[:maxLen]
		return strings.ReplaceAll(snippet, "\n", " ") + "..."
	}

	// Center the snippet around the match
	start := bestIdx - maxLen/2
	if start < 0 {
		start = 0
	}
	end := start + maxLen
	if end > len(body) {
		end = len(body)
		start = end - maxLen
		if start < 0 {
			start = 0
		}
	}

	snippet := body[start:end]
	snippet = strings.ReplaceAll(snippet, "\n", " ")

	if start > 0 {
		snippet = "..." + snippet
	}
	if end < len(body) {
		snippet += "..."
	}

	return snippet
}

// RunQuery executes the query command with the given configuration.
func RunQuery(cfg *QueryConfig) int {
	c := client.NewClient(cfg.SocketPath)

	params := daemon.SearchParams{Query: cfg.SearchTerm}
	resp, err := c.Send("search", params)
	if err != nil {
		fmt.Fprintf(os.Stderr, "error: daemon is not running (start it with \"agentlog start\")\n")
		return 1
	}

	if !resp.OK {
		fmt.Fprintf(os.Stderr, "error: %s\n", resp.Error)
		return 1
	}

	var entries []store.Entry
	if err := json.Unmarshal(resp.Result, &entries); err != nil {
		fmt.Fprintf(os.Stderr, "error: failed to parse results: %v\n", err)
		return 1
	}

	filtered, err := FilterEntries(entries, cfg, time.Now())
	if err != nil {
		fmt.Fprintf(os.Stderr, "error: %v\n", err)
		return 1
	}

	FormatResults(os.Stdout, filtered, cfg.SearchTerm)
	return 0
}

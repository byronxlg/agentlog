package daemon

import (
	"encoding/json"
	"fmt"
	"sort"
	"strings"
	"time"

	"github.com/byronxlg/agentlog/internal/store"
)

func (d *Daemon) handleExport(params json.RawMessage) Response {
	var p ExportParams
	if err := json.Unmarshal(params, &p); err != nil {
		return errResponse("invalid export params: " + err.Error())
	}

	if p.Format == "" {
		p.Format = "markdown"
	}
	if !isValidExportFormat(p.Format) {
		return errResponse(fmt.Sprintf("invalid format %q: must be one of markdown, json, text", p.Format))
	}
	if p.Template != "" && !isValidTemplate(p.Template) {
		return errResponse(fmt.Sprintf("invalid template %q: must be one of pr, retro, handoff", p.Template))
	}

	entries, err := d.queryExportEntries(p)
	if err != nil {
		return errResponse("export query failed: " + err.Error())
	}

	sort.Slice(entries, func(i, j int) bool {
		return entries[i].Timestamp.Before(entries[j].Timestamp)
	})

	var output string
	if p.Template != "" {
		output = formatTemplate(entries, p.Template)
	} else {
		output = formatExport(entries, p.Format)
	}

	return okResponse(output)
}

// queryExportEntries retrieves entries matching the export filters.
// It uses the most selective index query, then filters the rest in memory.
func (d *Daemon) queryExportEntries(p ExportParams) ([]store.Entry, error) {
	var entries []store.Entry
	var err error

	// Use the most selective index query available.
	switch {
	case p.SessionID != "":
		entries, err = d.index.QueryBySession(p.SessionID)
	case p.FilePath != "":
		entries, err = d.index.QueryByFilePath(p.FilePath)
	case p.Tag != "":
		entries, err = d.index.QueryByTags([]string{p.Tag})
	case p.Type != "":
		entries, err = d.index.QueryByType(store.EntryType(p.Type))
	default:
		entries, err = d.index.QueryRecent(10000, 0)
	}
	if err != nil {
		return nil, err
	}

	// Apply remaining filters in memory.
	now := time.Now()
	var sinceTime, untilTime time.Time

	if p.Since != "" {
		sinceTime, err = parseExportTime(p.Since, now)
		if err != nil {
			return nil, fmt.Errorf("invalid since: %w", err)
		}
	}
	if p.Until != "" {
		untilTime, err = parseExportTime(p.Until, now)
		if err != nil {
			return nil, fmt.Errorf("invalid until: %w", err)
		}
	}

	var filtered []store.Entry
	for _, e := range entries {
		if p.SessionID != "" && e.SessionID != p.SessionID {
			continue
		}
		if p.Type != "" && string(e.Type) != p.Type {
			continue
		}
		if p.Tag != "" && !hasTag(e.Tags, p.Tag) {
			continue
		}
		if p.FilePath != "" && !hasFile(e.FileRefs, p.FilePath) {
			continue
		}
		if !sinceTime.IsZero() && e.Timestamp.Before(sinceTime) {
			continue
		}
		if !untilTime.IsZero() && e.Timestamp.After(untilTime) {
			continue
		}
		filtered = append(filtered, e)
	}

	return filtered, nil
}

func isValidExportFormat(f string) bool {
	return f == "markdown" || f == "json" || f == "text"
}

func isValidTemplate(t string) bool {
	return t == "pr" || t == "retro" || t == "handoff"
}

func hasTag(tags []string, tag string) bool {
	for _, t := range tags {
		if t == tag {
			return true
		}
	}
	return false
}

func hasFile(refs []string, file string) bool {
	for _, r := range refs {
		if r == file {
			return true
		}
	}
	return false
}

// parseExportTime parses time values for export filters.
// Supports RFC3339, date-only, and relative durations (1h, 7d, 2w).
func parseExportTime(value string, now time.Time) (time.Time, error) {
	if value == "" {
		return time.Time{}, nil
	}

	if t, err := time.Parse(time.RFC3339, value); err == nil {
		return t, nil
	}
	if t, err := time.Parse("2006-01-02T15:04:05", value); err == nil {
		return t, nil
	}
	if t, err := time.Parse("2006-01-02", value); err == nil {
		return t, nil
	}

	return parseExportRelativeTime(value, now)
}

func parseExportRelativeTime(value string, now time.Time) (time.Time, error) {
	if len(value) < 2 {
		return time.Time{}, fmt.Errorf("invalid time format %q", value)
	}

	suffix := value[len(value)-1:]
	numStr := value[:len(value)-1]

	var num int
	if _, err := fmt.Sscanf(numStr, "%d", &num); err != nil {
		return time.Time{}, fmt.Errorf("invalid time format %q", value)
	}
	if num <= 0 {
		return time.Time{}, fmt.Errorf("invalid time format %q: duration must be positive", value)
	}

	switch strings.ToLower(suffix) {
	case "m":
		return now.Add(-time.Duration(num) * time.Minute), nil
	case "h":
		return now.Add(-time.Duration(num) * time.Hour), nil
	case "d":
		return now.AddDate(0, 0, -num), nil
	case "w":
		return now.AddDate(0, 0, -num*7), nil
	default:
		return time.Time{}, fmt.Errorf("invalid time unit %q in %q", suffix, value)
	}
}

// formatExport formats entries in the specified format.
func formatExport(entries []store.Entry, format string) string {
	if len(entries) == 0 {
		return formatEmptyExport(format)
	}

	switch format {
	case "json":
		return formatExportJSON(entries)
	case "text":
		return formatExportText(entries)
	default:
		return formatExportMarkdown(entries)
	}
}

func formatEmptyExport(format string) string {
	switch format {
	case "json":
		return "[]"
	case "text":
		return "No entries found."
	default:
		return "No entries found."
	}
}

func formatExportJSON(entries []store.Entry) string {
	data, err := json.MarshalIndent(entries, "", "  ")
	if err != nil {
		return "[]"
	}
	return string(data)
}

func formatExportText(entries []store.Entry) string {
	var b strings.Builder
	for i, e := range entries {
		if i > 0 {
			b.WriteString("\n")
		}
		fmt.Fprintf(&b, "%s [%s] %s\n", e.Timestamp.Format("2006-01-02 15:04:05"), e.Type, e.Title)
		if e.Body != "" {
			b.WriteString(e.Body)
			b.WriteString("\n")
		}
		if len(e.Tags) > 0 {
			fmt.Fprintf(&b, "Tags: %s\n", strings.Join(e.Tags, ", "))
		}
		if len(e.FileRefs) > 0 {
			fmt.Fprintf(&b, "Files: %s\n", strings.Join(e.FileRefs, ", "))
		}
	}
	return b.String()
}

func formatExportMarkdown(entries []store.Entry) string {
	var b strings.Builder
	b.WriteString("# Decision Log Export\n\n")

	for _, e := range entries {
		fmt.Fprintf(&b, "## %s\n\n", e.Title)
		fmt.Fprintf(&b, "- **Type:** %s\n", e.Type)
		fmt.Fprintf(&b, "- **Time:** %s\n", e.Timestamp.Format("2006-01-02 15:04:05"))
		fmt.Fprintf(&b, "- **Session:** %s\n", shortSession(e.SessionID))
		if len(e.Tags) > 0 {
			fmt.Fprintf(&b, "- **Tags:** %s\n", strings.Join(e.Tags, ", "))
		}
		if len(e.FileRefs) > 0 {
			fmt.Fprintf(&b, "- **Files:** %s\n", strings.Join(e.FileRefs, ", "))
		}
		if e.Body != "" {
			fmt.Fprintf(&b, "\n%s\n", e.Body)
		}
		b.WriteString("\n")
	}

	return b.String()
}

func shortSession(id string) string {
	if len(id) <= 8 {
		return id
	}
	return id[:8]
}

// formatTemplate formats entries using a built-in template.
func formatTemplate(entries []store.Entry, template string) string {
	switch template {
	case "pr":
		return formatTemplatePR(entries)
	case "retro":
		return formatTemplateRetro(entries)
	case "handoff":
		return formatTemplateHandoff(entries)
	default:
		return formatExportMarkdown(entries)
	}
}

func formatTemplatePR(entries []store.Entry) string {
	if len(entries) == 0 {
		return "No decisions to summarize."
	}

	var b strings.Builder
	b.WriteString("## What changed\n\n")

	for _, e := range entries {
		if e.Type != store.EntryTypeDecision {
			continue
		}
		fmt.Fprintf(&b, "- **%s**", e.Title)
		if e.Body != "" {
			// Use the first sentence or line of the body as reasoning.
			reason := firstSentence(e.Body)
			fmt.Fprintf(&b, " - %s", reason)
		}
		b.WriteString("\n")
	}

	// Include non-decision entries if any.
	var other []store.Entry
	for _, e := range entries {
		if e.Type != store.EntryTypeDecision {
			other = append(other, e)
		}
	}
	if len(other) > 0 {
		b.WriteString("\n## Notes\n\n")
		for _, e := range other {
			fmt.Fprintf(&b, "- [%s] %s\n", e.Type, e.Title)
		}
	}

	return b.String()
}

func formatTemplateRetro(entries []store.Entry) string {
	if len(entries) == 0 {
		return "No entries for retrospective."
	}

	grouped := make(map[store.EntryType][]store.Entry)
	for _, e := range entries {
		grouped[e.Type] = append(grouped[e.Type], e)
	}

	typeOrder := []struct {
		typ   store.EntryType
		label string
	}{
		{store.EntryTypeDecision, "Decisions made"},
		{store.EntryTypeDeferred, "Deferred items"},
		{store.EntryTypeAssumption, "Assumptions"},
		{store.EntryTypeAttemptFailed, "Failed attempts"},
		{store.EntryTypeQuestion, "Open questions"},
	}

	var b strings.Builder
	b.WriteString("# Retrospective\n\n")

	for _, to := range typeOrder {
		group := grouped[to.typ]
		if len(group) == 0 {
			continue
		}
		fmt.Fprintf(&b, "## %s\n\n", to.label)
		for _, e := range group {
			fmt.Fprintf(&b, "- **%s**", e.Title)
			if e.Body != "" {
				fmt.Fprintf(&b, ": %s", firstSentence(e.Body))
			}
			fmt.Fprintf(&b, " (%s)\n", e.Timestamp.Format("2006-01-02"))
		}
		b.WriteString("\n")
	}

	return b.String()
}

func formatTemplateHandoff(entries []store.Entry) string {
	if len(entries) == 0 {
		return "No decisions found for these files."
	}

	var b strings.Builder
	b.WriteString("# Handoff Document\n\n")

	for _, e := range entries {
		fmt.Fprintf(&b, "### %s [%s] (%s)\n\n", e.Title, e.Type, e.Timestamp.Format("2006-01-02 15:04:05"))
		if e.Body != "" {
			b.WriteString(e.Body)
			b.WriteString("\n\n")
		}
		if len(e.FileRefs) > 0 {
			fmt.Fprintf(&b, "**Files:** %s\n\n", strings.Join(e.FileRefs, ", "))
		}
	}

	return b.String()
}

func firstSentence(s string) string {
	// Take the first line.
	if idx := strings.IndexByte(s, '\n'); idx >= 0 {
		s = s[:idx]
	}
	// Take up to the first period.
	if idx := strings.IndexByte(s, '.'); idx >= 0 {
		return s[:idx+1]
	}
	// Truncate if too long.
	if len(s) > 120 {
		return s[:120] + "..."
	}
	return s
}

package cli

import (
	"fmt"
	"strconv"
	"strings"
	"time"
)

// ParseTimeFlag parses a time flag value into a time.Time.
// It accepts empty string (returns zero time), RFC3339/ISO 8601 timestamps,
// or relative durations like "30m", "1h", "7d", "2w" (subtracted from now).
func ParseTimeFlag(value string, now time.Time) (time.Time, error) {
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

	return parseRelativeTime(value, now)
}

func parseRelativeTime(value string, now time.Time) (time.Time, error) {
	if len(value) < 2 {
		return time.Time{}, fmt.Errorf("invalid time format %q: use RFC3339 or relative like 1h, 7d, 2w", value)
	}

	suffix := value[len(value)-1:]
	numStr := value[:len(value)-1]

	num, err := strconv.Atoi(numStr)
	if err != nil {
		return time.Time{}, fmt.Errorf("invalid time format %q: use RFC3339 or relative like 1h, 7d, 2w", value)
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
		return time.Time{}, fmt.Errorf("invalid time unit %q in %q: use m (minutes), h (hours), d (days), or w (weeks)", suffix, value)
	}
}

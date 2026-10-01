// Package dates parses human date/time expressions used by the CLI's
// --since/--until filters and presets like "today".
package dates

import (
	"fmt"
	"strconv"
	"strings"
	"time"
)

// ParseInstant converts an expression into a concrete time. When end is true
// a day-granular expression resolves to the end of that day, so "today"
// works as both a lower and an upper bound.
func ParseInstant(expr string, now time.Time, end bool) (time.Time, error) {
	s := strings.ToLower(strings.TrimSpace(expr))
	if s == "" {
		return time.Time{}, fmt.Errorf("dates: empty expression")
	}
	switch s {
	case "now":
		return now, nil
	case "today":
		return dayBound(now, end), nil
	case "yesterday":
		return dayBound(now.AddDate(0, 0, -1), end), nil
	case "tomorrow":
		return dayBound(now.AddDate(0, 0, 1), end), nil
	}

	for _, u := range []struct {
		suffix string
		unit   time.Duration
	}{
		{" minutes ago", time.Minute}, {" minute ago", time.Minute},
		{" hours ago", time.Hour}, {" hour ago", time.Hour},
		{" days ago", 24 * time.Hour}, {" day ago", 24 * time.Hour},
		{" weeks ago", 7 * 24 * time.Hour}, {" week ago", 7 * 24 * time.Hour},
	} {
		if strings.HasSuffix(s, u.suffix) {
			n, err := strconv.Atoi(strings.TrimSpace(strings.TrimSuffix(s, u.suffix)))
			if err != nil {
				return time.Time{}, fmt.Errorf("dates: bad amount in %q", expr)
			}
			return now.Add(-time.Duration(n) * u.unit), nil
		}
	}

	for _, layout := range []string{
		time.RFC3339, "2006-01-02T15:04", "2006-01-02 15:04", "2006-01-02",
		"02/01/2006", "2 january 2006", "2 jan 2006", "january 2 2006", "jan 2 2006",
	} {
		if t, err := time.ParseInLocation(layout, s, now.Location()); err == nil {
			if !strings.ContainsAny(layout, "15:04") {
				return dayBound(t, end), nil
			}
			return t, nil
		}
	}
	return time.Time{}, fmt.Errorf("dates: cannot parse %q (try 2006-01-02, today, yesterday, 3 days ago)", expr)
}

func dayBound(t time.Time, end bool) time.Time {
	y, m, d := t.Date()
	if end {
		return time.Date(y, m, d, 23, 59, 59, 0, t.Location())
	}
	return time.Date(y, m, d, 0, 0, 0, 0, t.Location())
}

// Range resolves a --since/--until pair plus optional presets.
func Range(now time.Time, preset, since, until string) (from, to time.Time, err error) {
	switch strings.ToLower(preset) {
	case "today":
		from, to = dayBound(now, false), dayBound(now, true)
	case "yesterday":
		y := now.AddDate(0, 0, -1)
		from, to = dayBound(y, false), dayBound(y, true)
	case "week":
		from = dayBound(now.AddDate(0, 0, -int((now.Weekday()+6)%7)), false)
		to = dayBound(now, true)
	case "month":
		from = time.Date(now.Year(), now.Month(), 1, 0, 0, 0, 0, now.Location())
		to = dayBound(now, true)
	case "":
	default:
		return from, to, fmt.Errorf("dates: unknown preset %q", preset)
	}
	if since != "" {
		if from, err = ParseInstant(since, now, false); err != nil {
			return from, to, err
		}
	}
	if until != "" {
		if to, err = ParseInstant(until, now, true); err != nil {
			return from, to, err
		}
	}
	return from, to, nil
}

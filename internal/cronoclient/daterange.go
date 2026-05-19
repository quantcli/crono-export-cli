// Package cronoclient wraps the internal MIT-licensed cronoapi client
// with a small session helper, typed export methods that return
// JSON-ready Go values, and shared Cobra flag plumbing for date-range
// selection.
package cronoclient

import (
	"fmt"
	"strings"
	"time"

	"github.com/spf13/cobra"
)

const dateLayout = "2006-01-02"

// DateRange is an inclusive [Start, End] window.  Only the calendar date
// (YYYY-MM-DD) of each endpoint is sent to Cronometer's export endpoints,
// so the time-of-day and zone on these values don't round-trip — but the
// calendar date is resolved in the user's local zone so that "today"
// matches the day the user sees in the Cronometer UI.
type DateRange struct {
	Start time.Time
	End   time.Time
}

// AddDateRangeFlags binds --since and --until on cmd.  Each subcommand calls
// this so they all share the same flag vocabulary, per the quantcli shared
// contract: https://github.com/quantcli/common/blob/main/CONTRACT.md#3-date-flags.
func AddDateRangeFlags(cmd *cobra.Command) {
	cmd.Flags().String("since", "",
		"Filter on or after date (today, yesterday, YYYY-MM-DD, or Nd/Nw/Nm/Ny; default 7d)")
	cmd.Flags().String("until", "",
		"Filter through date, inclusive (today, yesterday, YYYY-MM-DD, or Nd/Nw/Nm/Ny; default today)")
}

// ParseDateRangeFromFlags reads --since/--until off cmd and resolves them
// into a concrete DateRange.  Default when neither flag is set: the last
// 7 days ending today.  All values are interpreted in the user's local
// calendar.
func ParseDateRangeFromFlags(cmd *cobra.Command) (DateRange, error) {
	sinceStr, _ := cmd.Flags().GetString("since")
	untilStr, _ := cmd.Flags().GetString("until")
	return resolveDateRange(sinceStr, untilStr, time.Now())
}

func resolveDateRange(sinceStr, untilStr string, ref time.Time) (DateRange, error) {
	y, m, d := ref.Date()
	today := time.Date(y, m, d, 0, 0, 0, 0, ref.Location())

	since, err := parseDateValue(sinceStr, today)
	if err != nil {
		return DateRange{}, fmt.Errorf("bad --since: %w", err)
	}
	until, err := parseDateValue(untilStr, today)
	if err != nil {
		return DateRange{}, fmt.Errorf("bad --until: %w", err)
	}

	if since.IsZero() && until.IsZero() {
		// Default window: last 7 days ending today.
		return DateRange{Start: today.AddDate(0, 0, -6), End: today}, nil
	}
	if until.IsZero() {
		until = today
	}
	if since.IsZero() {
		since = until
	}
	if until.Before(since) {
		return DateRange{}, fmt.Errorf("--until (%s) is before --since (%s)",
			until.Format(dateLayout), since.Format(dateLayout))
	}
	return DateRange{Start: since, End: until}, nil
}

// parseDateValue parses a --since or --until value per the shared contract:
// "today", "yesterday", absolute YYYY-MM-DD, or relative Nd/Nw/Nm/Ny.
// Returns local midnight for the target day; empty string yields the zero
// time. The today reference is passed in for testability.
func parseDateValue(s string, today time.Time) (time.Time, error) {
	if s == "" {
		return time.Time{}, nil
	}
	switch strings.ToLower(s) {
	case "today":
		return today, nil
	case "yesterday":
		return today.AddDate(0, 0, -1), nil
	}
	if t, err := time.ParseInLocation(dateLayout, s, today.Location()); err == nil {
		return t, nil
	}
	if len(s) < 2 {
		return time.Time{}, fmt.Errorf("invalid date %q (use YYYY-MM-DD, today, yesterday, or Nd/Nw/Nm/Ny)", s)
	}
	n := 0
	if _, err := fmt.Sscanf(s[:len(s)-1], "%d", &n); err != nil {
		return time.Time{}, fmt.Errorf("invalid date %q (use YYYY-MM-DD, today, yesterday, or Nd/Nw/Nm/Ny)", s)
	}
	switch s[len(s)-1] {
	case 'd':
		return today.AddDate(0, 0, -n), nil
	case 'w':
		return today.AddDate(0, 0, -n*7), nil
	case 'm':
		return today.AddDate(0, -n, 0), nil
	case 'y':
		return today.AddDate(-n, 0, 0), nil
	default:
		return time.Time{}, fmt.Errorf("invalid date unit %q: use d, w, m, or y", string(s[len(s)-1]))
	}
}

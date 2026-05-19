package cronoclient

import (
	"testing"
	"time"
)

func TestResolveDateRange(t *testing.T) {
	ref := time.Date(2026, 5, 18, 12, 0, 0, 0, time.UTC)
	day := func(y int, m time.Month, d int) time.Time { return time.Date(y, m, d, 0, 0, 0, 0, time.UTC) }

	cases := []struct {
		name              string
		since, until      string
		wantStart, wantEnd time.Time
		wantEmpty         bool
	}{
		{
			name:      "both empty defaults to 7d ending today",
			wantStart: day(2026, 5, 12),
			wantEnd:   day(2026, 5, 18),
		},
		{
			name:      "since only defaults until to today",
			since:     "2026-05-15",
			wantStart: day(2026, 5, 15),
			wantEnd:   day(2026, 5, 18),
		},
		{
			name:      "until only defaults since to 7d before until (#18)",
			until:     "2026-05-15",
			wantStart: day(2026, 5, 9),
			wantEnd:   day(2026, 5, 15),
		},
		{
			name:      "today / yesterday",
			since:     "yesterday",
			until:     "today",
			wantStart: day(2026, 5, 17),
			wantEnd:   day(2026, 5, 18),
		},
		{
			name:      "relative 7d",
			since:     "7d",
			wantStart: day(2026, 5, 11),
			wantEnd:   day(2026, 5, 18),
		},
		{
			name:      "inverted range returns empty (#21)",
			since:     "today",
			until:     "yesterday",
			wantStart: day(2026, 5, 18),
			wantEnd:   day(2026, 5, 17),
			wantEmpty: true,
		},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			got, err := resolveDateRange(c.since, c.until, ref)
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
			if !got.Start.Equal(c.wantStart) || !got.End.Equal(c.wantEnd) {
				t.Errorf("got Start=%s End=%s, want Start=%s End=%s",
					got.Start.Format(dateLayout), got.End.Format(dateLayout),
					c.wantStart.Format(dateLayout), c.wantEnd.Format(dateLayout))
			}
			if got.IsEmpty() != c.wantEmpty {
				t.Errorf("IsEmpty()=%v, want %v", got.IsEmpty(), c.wantEmpty)
			}
		})
	}
}

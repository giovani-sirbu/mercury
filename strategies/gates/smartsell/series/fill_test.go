package series

import (
	"testing"
	"time"
)

// A stamp's bar is its UTC milliseconds floored to the hour, whatever zone the
// stamp carries; a stamp that is not a real time — zero, or before the epoch —
// has no bar, and no bar is ever zero.
func TestBarOpenFloorsTheStampToItsUTCHour(t *testing.T) {
	open := time.Date(2021, time.October, 10, 0, 0, 0, 0, time.UTC)
	zone := time.FixedZone("UTC+3", 3*60*60)
	for name, tc := range map[string]struct {
		at   time.Time
		want int64
	}{
		"on the open":          {open, open.UnixMilli()},
		"inside the hour":      {open.Add(55*time.Minute + 47*time.Second), open.UnixMilli()},
		"a millisecond before": {open.Add(time.Hour - time.Millisecond), open.UnixMilli()},
		"the next open":        {open.Add(time.Hour), open.Add(time.Hour).UnixMilli()},
		"a stamp in a zone":    {open.Add(20 * time.Minute).In(zone), open.UnixMilli()},
		"a zero time":          {time.Time{}, 0},
		"before the epoch":     {time.Date(1969, time.December, 31, 23, 30, 0, 0, time.UTC), 0},
		"the epoch itself":     {time.Unix(0, 0), 0},
	} {
		if got := BarOpen(tc.at); got != tc.want {
			t.Errorf("%s: BarOpen = %d, want %d", name, got, tc.want)
		}
	}
	if BarMs != int64(time.Hour/time.Millisecond) {
		t.Errorf("BarMs is %d, the 1h bar sophos serves is %d", BarMs, int64(time.Hour/time.Millisecond))
	}
}

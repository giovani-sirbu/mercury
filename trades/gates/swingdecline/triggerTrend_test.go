package swingdecline

import (
	"slices"
	"testing"
)

// Trigger needs the trend beside the shape: a staircase that holds triggers
// under a trend that meets SwingDeclineMinSmcTrendDown and names it last, and does
// not under one a row short of it or not read — a window that misses names the
// trend after what missed, and a ladder that read nothing names why and then the
// trend. With the condition off no trend fails a ladder, and the test is skipped.
func TestTriggerNeedsTheTrendBesideTheShape(t *testing.T) {
	needsTheTrend(t)
	series := staircase(4, staircaseTurns...)
	fills := []Fill{fillIn(0, 100), fillIn(8, 99), fillIn(16, 98), fillIn(32, 97)}
	if reading, holds := Trigger(fills, series, bearishTrend); !holds || reading.Reasons[len(reading.Reasons)-1] != bearishTrendReason {
		t.Fatalf("a staircase under a trend that holds must trigger and name the trend last, got holds %v, %+v", holds, reading)
	}
	frame := "depth 1 to 4, 1h bars 2021-10-01 00:00 to 2021-10-02 08:00 UTC"
	for name, tc := range map[string]struct {
		trend Trend
		want  string
	}{
		"short of the need": {shortTrend, shortTrendBreak},
		"not read":          {Trend{}, "smc trend direction not read"},
	} {
		reading, holds := Trigger(fills, series, tc.trend)
		if holds || reading.Holds || reading.Reasons != nil || !slices.Equal(reading.Breaks, []string{frame, tc.want}) {
			t.Errorf("%s: want the frame and the trend as breaks, got holds %v, %+v", name, holds, reading)
		}
	}

	flush := []Fill{fillIn(0, 130), fillIn(0, 128), fillIn(16, 88), fillIn(24, 85), fillIn(32, 82)}
	reading, holds := Trigger(flush, staircase(4, flushTurns...), shortTrend)
	want := []string{"depth 1 to 5, 1h bars 2021-10-01 00:00 to 2021-10-02 08:00 UTC", "largest leg 63% of the fall over 60%", shortTrendBreak}
	if holds || !slices.Equal(reading.Breaks, want) {
		t.Errorf("a window that misses names the trend after what missed, got holds %v, %q", holds, reading.Breaks)
	}
	reading, holds = Trigger(fills, Series{}, Trend{})
	if want := []string{unreadableBreak(Series{}, 4), "smc trend direction not read"}; holds || !slices.Equal(reading.Breaks, want) {
		t.Errorf("a ladder that read nothing names why, then the trend, got holds %v, %q", holds, reading.Breaks)
	}
}

// A trend that fails names the window that held on its shape — the longest of the
// windows that held, not the longest one read: the flush's window from the second
// fill holds where the one from the first misses, and of two that hold the first
// fill's answers. Under a trend that holds the same window opens the reasons.
func TestTriggerAFailingTrendNamesTheWindowThatHeld(t *testing.T) {
	needsTheTrend(t)
	for name, tc := range map[string]struct {
		series Series
		fills  []Fill
		from   int
		frame  string
	}{
		"the second fill's window, a longer one missing": {staircase(4, flushTurns...), []Fill{fillIn(0, 130), fillIn(8, 90), fillIn(16, 88), fillIn(24, 85), fillIn(32, 82)}, 2, "depth 2 to 5, 1h bars 2021-10-01 08:00 to 2021-10-02 08:00 UTC"},
		"the first fill's window, a shorter one holding": {staircase(4, staircaseTurns...), []Fill{fillIn(0, 100), fillIn(12, 98), fillIn(20, 95), fillIn(24, 93), fillIn(32, 88)}, 1, "depth 1 to 5, 1h bars 2021-10-01 00:00 to 2021-10-02 08:00 UTC"},
	} {
		if shape, holds := Trigger(tc.fills, tc.series, bearishTrend); !holds || shape.FromDepth != tc.from || shape.Reasons[0] != tc.frame || shape.Reasons[len(shape.Reasons)-1] != bearishTrendReason {
			t.Fatalf("%s: fixture drifted, the window from depth %d must hold and open the reasons, got holds %v, %q", name, tc.from, holds, shape.Reasons)
		}
		for trend, text := range map[Trend]string{shortTrend: shortTrendBreak, {}: "smc trend direction not read"} {
			reading, holds := Trigger(tc.fills, tc.series, trend)
			if holds || reading.Reasons != nil || reading.FromDepth != tc.from || !slices.Equal(reading.Breaks, []string{tc.frame, text}) {
				t.Errorf("%s, trend %+v: want the frame of the window that held, then %q, got holds %v, from depth %d, %q", name, trend, text, holds, reading.FromDepth, reading.Breaks)
			}
		}
	}
}

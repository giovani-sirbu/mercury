package swingdecline

import (
	"slices"
	"testing"

	"github.com/giovani-sirbu/mercury/strategies/gates/smarttakeloss/series"
)

// Trigger needs the trend beside the shape: a staircase that holds triggers
// under a trend that meets MinSmcTrendDown and names it last, and does
// not under one a row short of it or not read — a window that misses names the
// trend after what missed, and a ladder that read nothing names why and then the
// trend. With the condition off no trend fails a ladder, and the test is skipped.
func TestTriggerNeedsTheTrendBesideTheShape(t *testing.T) {
	needsTheTrend(t)
	tape := staircase(legBars, staircaseTurns...)
	fills := stairFills()
	if reading, holds := Trigger(fills, tape, bearishTrend); !holds || reading.Reasons[len(reading.Reasons)-1] != bearishTrendReason {
		t.Fatalf("a staircase under a trend that holds must trigger and name the trend last, got holds %v, %+v", holds, reading)
	}
	frame := frameOf(1, 4, 0, lastTurn)
	for name, tc := range map[string]struct {
		trend series.Trend
		want  string
	}{
		"short of the need": {shortTrend, shortTrendBreak},
		"not read":          {series.Trend{}, "smc trend direction not read"},
	} {
		reading, holds := Trigger(fills, tape, tc.trend)
		if holds || reading.Holds || reading.Reasons != nil || !slices.Equal(reading.Breaks, []string{frame, tc.want}) {
			t.Errorf("%s: want the frame and the trend as breaks, got holds %v, %+v", name, holds, reading)
		}
	}

	rallied := staircase(legBars, rallyTurns...)
	rallyEnd := (len(rallyTurns) - 1) * legBars
	rally := []series.Fill{fillIn(0, 100), fillIn(0, 99), fillIn(2*legBars, 92), fillIn(4*legBars, 90), fillIn(rallyEnd, 86)}
	window, _ := Read(rallied, barOpenAt(0), barOpenAt(rallyEnd), 1, 5)
	reading, holds := Trigger(rally, rallied, shortTrend)
	if want := append(slices.Clone(window.Breaks), shortTrendBreak); holds || !slices.Equal(reading.Breaks, want) {
		t.Errorf("a window that misses names the trend after what missed, got holds %v, %q", holds, reading.Breaks)
	}
	reading, holds = Trigger(fills, series.Series{}, series.Trend{})
	if want := []string{unreadableBreak(series.Series{}, 4), "smc trend direction not read"}; holds || !slices.Equal(reading.Breaks, want) {
		t.Errorf("a ladder that read nothing names why, then the trend, got holds %v, %q", holds, reading.Breaks)
	}
}

// TriggerAtLastBar folds the trend in as Trigger does, and it is the tick's: the
// decline that holds triggers under a trend that meets MinSmcTrendDown,
// its window ending at the last bar and the trend named last, and under one a row
// short, or not read, names that window and the trend alone.
func TestTriggerAtLastBarNeedsTheTrendOfTheTick(t *testing.T) {
	needsTheTrend(t)
	tape, fills := wholeDecline()
	frame := frameOf(1, len(fills), leadOf(legBars), len(tape.Opens)-1)
	reading, holds := TriggerAtLastBar(fills, tape, bearishTrend)
	if !holds || reading.Reasons[0] != frame || reading.Reasons[len(reading.Reasons)-1] != bearishTrendReason {
		t.Fatalf("the decline under a trend that holds must trigger and name the trend last, got holds %v, %q", holds, reading.Reasons)
	}
	for trend, text := range map[series.Trend]string{shortTrend: shortTrendBreak, {}: trendUnreadBreak} {
		reading, holds := TriggerAtLastBar(fills, tape, trend)
		if holds || reading.Reasons != nil || !slices.Equal(reading.Breaks, []string{frame, text}) {
			t.Errorf("trend %+v: want the frame and %q as breaks, got holds %v, %+v", trend, text, holds, reading)
		}
	}
}

// A trend that fails names the window that held on its shape — the longest of the
// windows that held, not the longest one read: on bars that start after the first
// fill the second fill's window holds where the first's is unreadable, and of two
// that hold the first fill's answers. Under a trend that holds the same window
// opens the reasons.
func TestTriggerAFailingTrendNamesTheWindowThatHeld(t *testing.T) {
	needsTheTrend(t)
	tape := staircase(legBars, staircaseTurns...)
	for name, tc := range map[string]struct {
		tape  series.Series
		fills []series.Fill
		from  int
		frame string
	}{
		"the second fill's window, a longer one unreadable": {cut(tape, 1, len(tape.Opens)), []series.Fill{fillIn(0, 130), fillIn(2*legBars, 92), fillIn(4*legBars, 86), fillIn(6*legBars, 81), fillIn(lastTurn, 77)}, 2, frameOf(2, 5, 2*legBars, lastTurn)},
		"the first fill's window, a shorter one holding":    {tape, []series.Fill{fillIn(0, 100), fillIn(3*legBars, 94), fillIn(5*legBars, 88), fillIn(6*legBars, 81), fillIn(lastTurn, 77)}, 1, frameOf(1, 5, 0, lastTurn)},
	} {
		if shape, holds := Trigger(tc.fills, tc.tape, bearishTrend); !holds || shape.FromDepth != tc.from || shape.Reasons[0] != tc.frame || shape.Reasons[len(shape.Reasons)-1] != bearishTrendReason {
			t.Fatalf("%s: fixture drifted, the window from depth %d must hold and open the reasons, got holds %v, %q", name, tc.from, holds, shape.Reasons)
		}
		for trend, text := range map[series.Trend]string{shortTrend: shortTrendBreak, {}: "smc trend direction not read"} {
			reading, holds := Trigger(tc.fills, tc.tape, trend)
			if holds || reading.Reasons != nil || reading.FromDepth != tc.from || !slices.Equal(reading.Breaks, []string{tc.frame, text}) {
				t.Errorf("%s, trend %+v: want the frame of the window that held, then %q, got holds %v, from depth %d, %q", name, trend, text, holds, reading.FromDepth, reading.Breaks)
			}
		}
	}
}

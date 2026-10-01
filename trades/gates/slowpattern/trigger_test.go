package slowpattern

import (
	"slices"
	"testing"
	"time"

	"github.com/giovani-sirbu/mercury/trades/aggragates"
)

// flushTurns is a decline whose first leg is a flush — a leg of more than the
// largest share of the whole fall — followed by the same staircase the other
// tests read: the window from its first bar fails on that share, and the
// window from the bar after the flush holds.
var flushTurns = []float64{94, 140, 90, 95, 88, 91, 85, 88, 82}

// Fewer fills than one more than SlowPatternMinDepthsBetween are never read,
// whatever the bars say: three fills over a staircase that holds at depth four
// read nothing, with the reason named.
func TestTriggerNeverReadsFewerFillsThanTheDepthsBetweenNeed(t *testing.T) {
	series := staircase(4, staircaseTurns...)
	ladder := []Fill{fillIn(0, 100), fillIn(8, 99), fillIn(16, 98), fillIn(32, 97)}
	for _, fills := range [][]Fill{nil, ladder[:1], ladder[:SlowPatternMinDepthsBetween]} {
		reading, holds := Trigger(fills, series)
		want := []string{fewFillsBreak(len(fills))}
		if holds || reading.Holds || !slices.Equal(reading.Breaks, want) || reading.ToDepth != len(fills) {
			t.Errorf("%d fills: want nothing read, got holds %v, %+v", len(fills), holds, reading)
		}
	}
	if reading, holds := Trigger(ladder, series); !holds || reading.FromDepth != 1 || reading.ToDepth != SlowPatternMinDepthsBetween+1 {
		t.Fatalf("the fourth fill must read the window from the first, got holds %v, %+v", holds, reading)
	}
}

// The windows run from every earlier fill at least SlowPatternMinDepthsBetween
// back, longest first, and the ladder triggers if ANY of them holds: here the
// window from the first fill fails on its flush and the one from the second
// holds.
func TestTriggerAnyWindowMayPass(t *testing.T) {
	series := staircase(4, flushTurns...)
	fills := []Fill{fillIn(0, 130), fillIn(8, 90), fillIn(16, 88), fillIn(24, 85), fillIn(32, 82)}
	reading, holds := Trigger(fills, series)
	if !holds || reading.FromDepth != 2 || reading.ToDepth != 5 || reading.StartAt != barOpenAt(8) || reading.EndAt != barOpenAt(32) {
		t.Fatalf("the window from the second fill must hold, got holds %v, %+v", holds, reading)
	}
	if first, readable := Read(series, barOpenAt(0), barOpenAt(32), 1, 5); !readable || first.Holds {
		t.Fatalf("the window from the first fill must be read and fail, got readable %v, %+v", readable, first)
	}
}

// Longest first: when more than one window holds, the one from the earliest
// fill is the answer.
func TestTriggerReadsTheLongestWindowFirst(t *testing.T) {
	series := staircase(4, staircaseTurns...)
	fills := []Fill{fillIn(0, 100), fillIn(12, 98), fillIn(20, 95), fillIn(24, 93), fillIn(32, 88)}
	for from, startBar := range map[int]int{1: 0, 2: 12} {
		if window, readable := Read(series, barOpenAt(startBar), barOpenAt(32), from, 5); !readable || !window.Holds {
			t.Fatalf("the window from depth %d must hold on its own, got readable %v, %+v", from, readable, window)
		}
	}
	reading, holds := Trigger(fills, series)
	if !holds || reading.FromDepth != 1 || reading.StartAt != barOpenAt(0) {
		t.Fatalf("the window from the first fill must answer first, got holds %v, %+v", holds, reading)
	}
}

// No window holding, the answer is the longest readable reading with the
// breaks that decided it, and two fills in the first bar read as one window.
func TestTriggerNoWindowHoldsAnswersTheLongestBreaks(t *testing.T) {
	series := staircase(4, flushTurns...)
	fills := []Fill{fillIn(0, 130), fillIn(0, 128), fillIn(16, 88), fillIn(24, 85), fillIn(32, 82)}
	reading, holds := Trigger(fills, series)
	if holds || reading.Holds || reading.FromDepth != 1 || reading.ToDepth != 5 {
		t.Fatalf("nothing holds from the first bar, want the window from depth 1, got holds %v, %+v", holds, reading)
	}
	if len(reading.Breaks) != 2 || reading.Breaks[1] != "largest leg 63% of the fall over 60%" {
		t.Errorf("want the frame and the flush, got %q", reading.Breaks)
	}
}

// A fill without a stamp has no bar: its window is skipped and the next fill's
// is read, and a newest fill without one ends no window at all.
func TestTriggerSkipsUnstampedFills(t *testing.T) {
	series := staircase(4, flushTurns...)
	fills := []Fill{{Price: 130}, fillIn(8, 90), fillIn(16, 88), fillIn(24, 85), fillIn(32, 82)}
	if reading, holds := Trigger(fills, series); !holds || reading.FromDepth != 2 {
		t.Fatalf("the unstamped first fill must be skipped for the second, got holds %v, %+v", holds, reading)
	}
	fills[4] = Fill{Price: 82, At: time.Time{}}
	reading, holds := Trigger(fills, series)
	if holds || !slices.Equal(reading.Breaks, []string{unstampedBreak(5)}) {
		t.Fatalf("an unstamped newest fill must read nothing, got holds %v, %+v", holds, reading)
	}
}

// A window the served bars do not hold is unreadable and the ladder is not
// triggered, with the reason named: no bars served, or the fills' bars outside
// them — older than the series' first bar, or newer than its last.
func TestTriggerFailsClosedOnBarsNotServed(t *testing.T) {
	series := staircase(4, staircaseTurns...)
	fills := []Fill{fillIn(0, 100), fillIn(8, 99), fillIn(16, 98), fillIn(32, 97)}
	old := Series{Opens: series.Opens[4:], Closes: series.Closes[4:]}
	young := Series{Opens: series.Opens[:30], Closes: series.Closes[:30]}
	for name, tc := range map[string]struct {
		series Series
		want   string
	}{
		"no bars":                        {Series{}, unreadableBreak(Series{}, 4)},
		"the first fill older than them": {old, unreadableBreak(old, 4)},
		"the newest fill not closed yet": {young, unreadableBreak(young, 4)},
	} {
		reading, holds := Trigger(fills, tc.series)
		if holds || !slices.Equal(reading.Breaks, []string{tc.want}) {
			t.Errorf("%s: want %q, got holds %v, %+v", name, tc.want, holds, reading)
		}
	}
}

// The answer is the same on every later tick: the bars after the newest
// fill's own bar never change it.
func TestTriggerIsDeterministicAsBarsGrow(t *testing.T) {
	staircased := staircase(4, staircaseTurns...)
	grown := staircase(4, append(append([]float64{}, staircaseTurns...), 99, 120, 60)...)
	fills := []Fill{fillIn(0, 100), fillIn(8, 99), fillIn(16, 98), fillIn(32, 97)}
	want, _ := Trigger(fills, staircased)
	got, _ := Trigger(fills, grown)
	if want.Weight != got.Weight || want.Holds != got.Holds || !slices.Equal(want.Reasons, got.Reasons) {
		t.Fatalf("later bars changed the answer:\nbefore %+v\nafter  %+v", want, got)
	}
}

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
}

// The series is the block's two arrays as sophos served them, and it is served
// only when they are one list of bars: both empty, only one of them, or of
// different lengths is not, and then it has no last bar and no bar is in it.
func TestSeriesIsServedOnlyAsOneListOfBars(t *testing.T) {
	block := aggragates.SmartTakeLossIndicators{SlowPatternOpens: []int64{barOpenAt(0), barOpenAt(1), barOpenAt(3)}, SlowPatternCloses: []float64{10, 11, 12}}
	served := SeriesOf(block)
	if !served.Served() || served.LastBarOpen() != barOpenAt(3) {
		t.Fatalf("three bars are served up to the last open, got %+v", served)
	}
	if index, found := served.indexOf(barOpenAt(3)); !found || index != 2 {
		t.Errorf("a bar after a gap is found by its open time, got %d, %v", index, found)
	}
	if _, found := served.indexOf(barOpenAt(2)); found {
		t.Error("a bar the exchange left out is not in the series")
	}
	for name, block := range map[string]aggragates.SmartTakeLossIndicators{
		"neither array":     {},
		"empty arrays":      {SlowPatternOpens: []int64{}, SlowPatternCloses: []float64{}},
		"only the opens":    {SlowPatternOpens: []int64{barOpenAt(0)}},
		"only the closes":   {SlowPatternCloses: []float64{10}},
		"different lengths": {SlowPatternOpens: []int64{barOpenAt(0), barOpenAt(1)}, SlowPatternCloses: []float64{10}},
	} {
		series := SeriesOf(block)
		if _, found := series.indexOf(barOpenAt(0)); series.Served() || series.LastBarOpen() != 0 || found {
			t.Errorf("%s: no series is served, got %+v", name, series)
		}
	}
}

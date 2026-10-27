package lineardecline

import (
	"github.com/giovani-sirbu/mercury/strategies/gates/smarttakeloss/series"
	"math"
	"slices"
	"testing"
	"time"
)

// change is one way to spoil a line ladder: it edits the fills and the series in place.
type change func(fills []series.Fill, tape *series.Series)

func unstamped(index int) change {
	return func(fills []series.Fill, _ *series.Series) {
		fills[index].At = time.Time{}
	}
}

func restamped(index int, afterFirst time.Duration) change {
	return func(fills []series.Fill, _ *series.Series) {
		fills[index].At = fills[0].At.Add(afterFirst)
	}
}

func repriced(index int, price float64) change {
	return func(fills []series.Fill, _ *series.Series) {
		fills[index].Price = price
	}
}

func unserved(replacement series.Series) change {
	return func(_ []series.Fill, tape *series.Series) {
		*tape = replacement
	}
}

func withoutBar(bar int) change {
	return func(_ []series.Fill, tape *series.Series) {
		tape.Opens = slices.Delete(tape.Opens, bar, bar+1)
		tape.Closes = slices.Delete(tape.Closes, bar, bar+1)
	}
}

func endingBefore(bar int) change {
	return func(_ []series.Fill, tape *series.Series) {
		tape.Opens, tape.Closes = tape.Opens[:bar], tape.Closes[:bar]
	}
}

func closesEndingBefore(bar int) change {
	return func(_ []series.Fill, tape *series.Series) {
		tape.Closes = tape.Closes[:bar]
	}
}

func closedAt(bar int, closed float64) change {
	return func(_ []series.Fill, tape *series.Series) {
		tape.Closes[bar] = closed
	}
}

// measured is whether a reading carries any of the numbers a read ladder has.
func measured(reading Reading) bool {
	return reading.Slope != 0 || reading.PreviousSlope != 0 || reading.Deviation != 0 ||
		reading.PreviousDeviation != 0 || reading.NetPct != 0
}

// A line that cannot be drawn or read never holds, and names why: a stamp missing,
// the later fill not after the first, a price or a close that is not above zero, a
// series that is not served or lacks the bar of a fill. A close outside the bars
// the chords read does not matter. Nothing is measured of an unread ladder.
func TestAChordIsNotReadWhenItsFillsOrBarsCannotDrawIt(t *testing.T) {
	depth := Window
	for _, tc := range []struct {
		name   string
		change change
		want   misses
		text   string
	}{
		{"the first fill carries no stamp", unstamped(0), missStamps, stampsBreak},
		{"the fill before the newest carries no stamp", unstamped(depth - 2), missStamps, stampsBreak},
		{"the newest fill carries no stamp", unstamped(depth - 1), missStamps, stampsBreak},
		{"the newest fill is stamped with the first's", restamped(depth-1, 0), missStamps, stampsBreak},
		{"the newest fill is stamped before the first", restamped(depth-1, -time.Hour), missStamps, stampsBreak},
		{"the fill before the newest is stamped before the first", restamped(depth-2, -time.Hour), missStamps, stampsBreak},
		{"a price at zero", repriced(depth-1, 0), missPrices, pricesBreak},
		{"a price under zero", repriced(0, -1), missPrices, pricesBreak},
		{"a price that is not a number", repriced(depth-2, math.NaN()), missPrices, pricesBreak},
		{"no bars served", unserved(series.Series{}), missBars, barsBreak},
		{"the opens longer than the closes", closesEndingBefore(1), missBars, barsBreak},
		{"the first fill's bar left out", withoutBar(leadBars), missBars, barsBreak},
		{"the bar of the fill before the newest left out", withoutBar(newestBar(depth - 1)), missBars, barsBreak},
		{"the newest fill's bar left out", withoutBar(newestBar(depth)), missBars, barsBreak},
		{"the series ends before the newest fill's bar", endingBefore(newestBar(depth)), missBars, barsBreak},
		{"a close at zero among the bars", closedAt(leadBars+1, 0), missCloses, closesBreak},
		{"a close under zero in the newest fill's bar", closedAt(newestBar(depth), -1), missCloses, closesBreak},
		{"a close that is not a number", closedAt(leadBars, math.NaN()), missCloses, closesBreak},
		{"a close at zero before the first fill's bar", closedAt(leadBars-1, 0), 0, ""},
		{"a close at zero after the newest fill's bar", closedAt(newestBar(depth)+1, 0), 0, ""},
	} {
		fills, tape := holding(depth)
		tc.change(fills, &tape)
		reading, missed := assess(fills, tape)
		if missed != tc.want {
			t.Errorf("%s: misses %b, want %b", tc.name, missed, tc.want)
			continue
		}
		if _, holds := Trigger(fills, tape, bearishTrend); holds != (tc.want == 0) {
			t.Errorf("%s: holds %v, want %v", tc.name, holds, tc.want == 0)
		}
		var wantBreaks []string
		if tc.want != 0 {
			wantBreaks = []string{tc.text}
		}
		assertBreaks(t, fills, tape, bearishTrend, wantBreaks...)
		if tc.want != 0 && measured(reading) {
			t.Errorf("%s: nothing is measured of an unread ladder, got %+v", tc.name, reading)
		}
	}
}

// Each condition misses alone, its bound included in what holds, and a number that is
// not a number misses: the readings are made by hand, so the test holds under any
// calibration of the rule.
func TestMissesNameEachConditionAlone(t *testing.T) {
	deviation, net := MaxDeviationPct, MinFallPct
	slope, previous := -(1 + MaxSlopeChange/2), -1.0
	if missed := missesOf(readingOf(deviation, deviation, slope, previous, net)); missed != 0 {
		t.Fatalf("a reading at every bound holds, got misses %b", missed)
	}
	changed := -(1 + 2*MaxSlopeChange + 0.1)
	for _, tc := range []struct {
		name    string
		reading Reading
		want    misses
	}{
		{"a deviation over", readingOf(past(deviation), deviation, slope, previous, net), missDeviation},
		{"a previous deviation over", readingOf(deviation, past(deviation), slope, previous, net), missPreviousDeviation},
		{"a deviation that is not a number", readingOf(math.NaN(), deviation, slope, previous, net), missDeviation},
		{"a newest chord at zero", readingOf(deviation, deviation, 0, previous, net), missRising},
		{"a newest chord rising", readingOf(deviation, deviation, 1, previous, net), missRising},
		{"a previous chord at zero", readingOf(deviation, deviation, slope, 0, net), missPreviousRising},
		{"a change of slope over", readingOf(deviation, deviation, changed, previous, net), missSlopeChange},
		{"a fall short", readingOf(deviation, deviation, slope, previous, past(net)), missFall},
		{"a net move that is not a number", readingOf(deviation, deviation, slope, previous, math.NaN()), missFall},
	} {
		if missed := missesOf(tc.reading); missed != tc.want {
			t.Errorf("%s: misses %b, want %b", tc.name, missed, tc.want)
		}
	}
}

// A change of slope exactly at its bound holds, the bound included. Two slopes can
// make only the changes their floating-point division gives, so a bound that is not
// one of them is never met exactly and the test is skipped.
func TestAChangeOfSlopeAtItsBoundHolds(t *testing.T) {
	deviation, previous := MaxDeviationPct, -1.0
	reading := readingOf(deviation, deviation, previous*(1+MaxSlopeChange), previous, MinFallPct)
	if slopeChange(reading) != MaxSlopeChange {
		t.Skip("MaxSlopeChange is not a change two slopes can make exactly: the bound is never met")
	}
	if missed := missesOf(reading); missed != 0 {
		t.Errorf("a change of slope at its bound holds, got misses %b", missed)
	}
}

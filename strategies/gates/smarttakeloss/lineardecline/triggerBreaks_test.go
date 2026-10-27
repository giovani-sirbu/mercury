package lineardecline

import (
	"fmt"
	"github.com/giovani-sirbu/mercury/strategies/gates/smarttakeloss/series"
	"math"
	"reflect"
	"slices"
	"testing"
)

// A close off the line over the bound on the newest chord alone — in a bar after the
// fill before the newest — names that chord's deviation at the newest depth.
func TestTriggerNamesADeviationOverOnTheNewestChordAlone(t *testing.T) {
	depth := Window
	fills, tape := holding(depth)
	bump(tape, newestBar(depth), over)
	reading := mustMiss(t, fills, tape)
	assertNear(t, "deviation", reading.Deviation, over, 1e-6)
	assertNear(t, "previous deviation", reading.PreviousDeviation, 0, 1e-6)
	assertBreaks(t, fills, tape, bearishTrend, frameOf(depth), deviationText(reading.Deviation, depth))
}

// A close off the line over the bound in a bar both chords read names the deviation
// of each at its own end depth.
func TestTriggerNamesADeviationOverOnBothChordsOverTheBarsTheyShare(t *testing.T) {
	depth := Window
	fills, tape := holding(depth)
	bump(tape, leadBars+1, over)
	reading := mustMiss(t, fills, tape)
	assertBreaks(t, fills, tape, bearishTrend,
		frameOf(depth), deviationText(reading.Deviation, depth), deviationText(reading.PreviousDeviation, depth-1))
}

// A fill before the newest paid on a shallower line than the newest's makes the
// chord to it fall slower than the chord to the newest fill: the change between the
// slopes is over its bound, and the break names both slopes at the earlier depth.
func TestTriggerNamesAChangeOfSlopeOver(t *testing.T) {
	depth := Window
	fills, tape := holding(depth)
	shallower := lineSlope / (1 + 2*MaxSlopeChange + 0.1)
	fills[depth-2].Price = linePrice(shallower, float64((depth-2)*fillGap))
	reading := mustMiss(t, fills, tape)
	assertNear(t, "slope", reading.Slope, lineSlope, 1e-9)
	assertNear(t, "previous slope", reading.PreviousSlope, shallower, 1e-9)
	changed := 100 * math.Abs(reading.Slope/reading.PreviousSlope-1)
	want := fmt.Sprintf("slope %.2f%%/h against %.2f%%/h at depth %d, changed %.0f%% over %.0f%%",
		reading.Slope, reading.PreviousSlope, depth-1, changed, 100*MaxSlopeChange)
	breaks := Breaks(fills, tape, bearishTrend)
	if len(breaks) < 2 || breaks[0] != frameOf(depth) || !slices.Contains(breaks, want) {
		t.Errorf("want the line and %q among the breaks, got %q", want, breaks)
	}
}

// A line that rises, closes and all, has two chords that do not fall and a net move
// that went up, each named at its depth: the change between the slopes is not asked
// of a slope that does not fall.
func TestTriggerNamesAChordThatDoesNotFall(t *testing.T) {
	needsAFall(t)
	depth := Window
	fills, tape := lineLadder(depth, -lineSlope)
	reading := mustMiss(t, fills, tape)
	assertBreaks(t, fills, tape, bearishTrend, frameOf(depth),
		risingText(reading.Slope, depth), risingText(reading.PreviousSlope, depth-1),
		fmt.Sprintf("up %.1f%% short of %.1f%% down", reading.NetPct, -MinFallPct))
}

// A line that falls, but by half the least the rule asks, misses on that alone.
func TestTriggerNamesAFallShort(t *testing.T) {
	needsAFall(t)
	depth := Window
	fills, tape := lineLadder(depth, 0.5*MinFallPct/float64((Window-1)*fillGap))
	reading := mustMiss(t, fills, tape)
	assertBreaks(t, fills, tape, bearishTrend, frameOf(depth),
		fmt.Sprintf("down %.1f%% short of %.1f%%", -reading.NetPct, -MinFallPct))
}

// No ladder with fewer fills than the window — the rule's own bound, never under what
// the chords need — is ever read, whatever the market did, and it says so; at that
// many fills the ladder on one line holds.
func TestTriggerNeverReadsFewerFillsThanTheRuleNeeds(t *testing.T) {
	least := Window
	fills, tape := holding(least)
	for count := 0; count < least; count++ {
		reading, holds := Trigger(fills[:count], tape, bearishTrend)
		if holds || !reflect.DeepEqual(reading, Reading{ToDepth: count}) {
			t.Errorf("%d fills: want nothing read, got holds %v, %+v", count, holds, reading)
		}
		assertBreaks(t, fills[:count], tape, bearishTrend, fmt.Sprintf("%d fills short of %d needed", count, least))
	}
	if _, holds := Trigger(fills, tape, bearishTrend); !holds {
		t.Errorf("the ladder holds at the rule's least number of fills, got breaks %q", Breaks(fills, tape, bearishTrend))
	}
}

// The trend is the tick's and is asked first: a line that holds does not trigger
// under a trend a row short of the need or not read, and the reading says nothing
// of the line — Breaks, which reads it anyway, names the line and the trend — but
// the same fills and bars trigger on the tick the trend meets the need. A line that
// misses under a trend that misses names both, the trend last.
func TestTriggerNeedsTheTrendOfTheTick(t *testing.T) {
	needsTheTrend(t)
	depth := Window
	fills, tape := holding(depth)
	off, offBars := holding(depth)
	bump(offBars, newestBar(depth), over)
	offReading, _ := assess(off, offBars)
	for _, tc := range []struct {
		name  string
		trend series.Trend
		text  string
	}{
		{"short of the need", shortTrend, trendText(shortTrend)},
		{"not read", series.Trend{}, "smc trend direction not read"},
	} {
		if reading, holds := Trigger(fills, tape, tc.trend); holds || !reflect.DeepEqual(reading, Reading{ToDepth: depth}) {
			t.Errorf("%s: want nothing read, got holds %v, %+v", tc.name, holds, reading)
		}
		assertBreaks(t, fills, tape, tc.trend, frameOf(depth), tc.text)
		assertBreaks(t, off, offBars, tc.trend, frameOf(depth), deviationText(offReading.Deviation, depth), tc.text)
	}
	if _, holds := Trigger(fills, tape, bearishTrend); !holds {
		t.Errorf("a trend that turns down later triggers the same ladder, got breaks %q", Breaks(fills, tape, bearishTrend))
	}
}

// With the condition off — the need at zero or below — no trend is asked or named,
// whatever the trend.
func TestWithTheConditionOffNoTrendIsAskedOrNamed(t *testing.T) {
	fills, tape := holding(Window)
	reading, unmet := assess(fills, tape)
	if !meets(series.Trend{}, 0) || unmet != 0 {
		t.Fatalf("with no condition every trend meets it and the line holds, got misses %b", unmet)
	}
	named, unnamed := holdReasons(reading, bearishTrend, 1), holdReasons(reading, series.Trend{}, 0)
	if !slices.Equal(unnamed, named[:len(named)-1]) {
		t.Errorf("no trend is named:\n got %q\nwant %q", unnamed, named[:len(named)-1])
	}
	if breaks := breakReasons(reading, unmet, series.Trend{}, 0); breaks != nil {
		t.Errorf("no trend is missed, got %q", breaks)
	}
}

// The trend meets the need exactly when no condition is asked (zero or below, where
// every trend does, one not served too) or sophos read a row and the rows down reach
// the need, the bound included; the exported predicate is that at
// MinSmcTrendDown.
func TestMeetsTheNeedIsServedWithEnoughRowsDown(t *testing.T) {
	for need := -1; need <= 4; need++ {
		for down := 0; down <= 3; down++ {
			for _, valid := range []int{0, down, 3} {
				trend, want := series.Trend{Down: down, Valid: valid}, need <= 0 || (valid > 0 && down >= need)
				if got := meets(trend, need); got != want {
					t.Errorf("need %d, trend %+v: meets %v, want %v", need, trend, got, want)
				}
			}
		}
	}
	for _, trend := range []series.Trend{bearishTrend, shortTrend, {}, {Down: 3}, {Valid: 3}} {
		want := MinSmcTrendDown <= 0 || (trend.Valid > 0 && trend.Down >= MinSmcTrendDown)
		if got := MeetsTheNeed(trend); got != want {
			t.Errorf("trend %+v: MeetsTheNeed %v, want %v at MinSmcTrendDown", trend, got, want)
		}
	}
}

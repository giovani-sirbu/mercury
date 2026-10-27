package lineardecline

import (
	"fmt"
	"github.com/giovani-sirbu/mercury/strategies/gates/smarttakeloss/series"
	"reflect"
	"slices"
	"testing"
	"time"
)

// measuredAt are the constants the depths of tapeLadders were read at: the replays
// that assert those depths run only while every one is what it was, and skip, naming
// the constant, as soon as one is retuned. The trend is not among them: the replays
// read the ladders under a trend that meets MinSmcTrendDown, whatever
// it is.
var measuredAt = []struct {
	name                string
	inForce, measuredAs float64
}{
	{"MinFills", float64(MinFills), 4},
	{"MaxDeviationPct", MaxDeviationPct, 8.0},
	{"MaxSlopeChange", MaxSlopeChange, 0.25},
	{"MinFallPct", MinFallPct, -8.0},
}

// needsTheMeasuredConstants skips a test that asserts the depths a ladder's line holds
// at once a constant has been retuned since they were read, naming it.
func needsTheMeasuredConstants(t *testing.T) {
	t.Helper()
	for _, constant := range measuredAt {
		if constant.inForce != constant.measuredAs {
			t.Skipf("%s is %v, the depths were read at %v", constant.name, constant.inForce, constant.measuredAs)
		}
	}
}

// holdingDepths are the depths at which a ladder's line holds, read the way the gate
// reads it: depth by depth, on the fills the ladder had by then, under a trend that
// meets its need.
func holdingDepths(fills []series.Fill, tape series.Series) []int {
	var depths []int
	for depth := 1; depth <= len(fills); depth++ {
		if _, holds := Trigger(fills[:depth], tape, bearishTrend); holds {
			depths = append(depths, depth)
		}
	}
	return depths
}

// frameAt is the frame the reasons of the line through these fills open with, written
// out from their stamps against the grammar: the bars of the window's first fill and of
// the newest.
func frameAt(fills []series.Fill) string {
	bar := func(fill series.Fill) string {
		return fill.At.Truncate(time.Hour).Format("2006-01-02 15:04")
	}
	depth := len(fills)
	return fmt.Sprintf("depth %d to %d, 1h bars %s to %s UTC", depth-Window+1, depth, bar(fills[depth-Window]), bar(fills[depth-1]))
}

// bruteSlope is the slope of the chord from the fill at start to the fill at end, read
// the long way: the net move in percent of the start fill's price, per hour between
// their stamps.
func bruteSlope(fills []series.Fill, start, end int) float64 {
	return (fills[end].Price/fills[start].Price - 1) * 100 / fills[end].At.Sub(fills[start].At).Hours()
}

// A ladder of real fills, read on the bars it fell through under a trend that meets
// the need, holds at the depths measured for it — the first where its line would latch
// the indecision direction — and at no other; one measured to hold nowhere holds
// nowhere. The reading at the first is framed by the bars of the window's first fill
// and of that depth's fill.
func TestARealLadderFirstHoldsAtTheDepthMeasured(t *testing.T) {
	needsTheMeasuredConstants(t)
	for _, ladder := range tapeLadders {
		t.Run(ladder.tape, func(t *testing.T) {
			fills, tape := ladder.read(t)
			depths := holdingDepths(fills, tape)
			if !slices.Equal(depths, ladder.holds) {
				t.Fatalf("holds at depths %v, want %v", depths, ladder.holds)
			}
			if len(ladder.holds) == 0 {
				return
			}
			first := ladder.holds[0]
			reading, _ := Trigger(fills[:first], tape, bearishTrend)
			if reading.FromDepth != first-Window+1 || reading.ToDepth != first || reading.Reasons[0] != frameAt(fills[:first]) {
				t.Errorf("the reading runs from depth %d to %d, framed %q, got %+v", first-Window+1, first, frameAt(fills[:first]), reading)
			}
		})
	}
}

// The trend is the tick's, not the bars': at the depth a real ladder first holds, a
// trend a row short of its need, or not read, leaves nothing read and names the line
// and the trend alone, and the same fills on the same bars hold under a trend that
// turns down later.
func TestTheTrendDecidesARealLadderAtItsFirstHoldingDepth(t *testing.T) {
	needsTheTrend(t)
	needsTheMeasuredConstants(t)
	for _, ladder := range tapeLadders {
		t.Run(ladder.tape, func(t *testing.T) {
			if len(ladder.holds) == 0 {
				t.Skip("the ladder holds nowhere, so no depth asks the trend")
			}
			fills, tape := ladder.read(t)
			depth := ladder.holds[0]
			held := fills[:depth]
			for _, tc := range []struct {
				name  string
				trend series.Trend
				text  string
			}{
				{"short of the need", shortTrend, trendText(shortTrend)},
				{"not read", series.Trend{}, "smc trend direction not read"},
			} {
				if reading, holds := Trigger(held, tape, tc.trend); holds || !reflect.DeepEqual(reading, Reading{ToDepth: depth}) {
					t.Errorf("%s: want nothing read, got holds %v, %+v", tc.name, holds, reading)
				}
				assertBreaks(t, held, tape, tc.trend, frameAt(held), tc.text)
			}
			if _, holds := Trigger(held, tape, bearishTrend); !holds {
				t.Errorf("a trend that turns down later holds the same ladder, got breaks %q", Breaks(held, tape, bearishTrend))
			}
		})
	}
}

// Whatever the constants, and whether the line holds or not, the numbers of a real
// ladder at every depth the rule reads are those read the long way, in hours and
// prices: the slope of each chord from the stamps and prices of its two fills, the
// net move from the window's first fill, and the largest distance of a close from each chord.
func TestTheNumbersOfARealLadderAreReadTheLongWay(t *testing.T) {
	for _, ladder := range tapeLadders {
		t.Run(ladder.tape, func(t *testing.T) {
			fills, tape := ladder.read(t)
			for depth := Window; depth <= len(fills); depth++ {
				reading, _ := Trigger(fills[:depth], tape, bearishTrend)
				name, start := fmt.Sprintf("depth %d", depth), depth-Window
				assertNear(t, name+": slope", reading.Slope, bruteSlope(fills, start, depth-1), 1e-9)
				assertNear(t, name+": previous slope", reading.PreviousSlope, bruteSlope(fills, start, depth-2), 1e-9)
				assertNear(t, name+": net move", reading.NetPct, (fills[depth-1].Price/fills[start].Price-1)*100, 1e-9)
				assertNear(t, name+": deviation", reading.Deviation, bruteDeviation(fills, tape, start, depth-1), 1e-9)
				assertNear(t, name+": previous deviation", reading.PreviousDeviation, bruteDeviation(fills, tape, start, depth-2), 1e-9)
			}
		})
	}
}

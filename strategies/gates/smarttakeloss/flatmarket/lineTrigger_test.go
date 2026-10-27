package flatmarket

import (
	"fmt"
	"github.com/giovani-sirbu/mercury/strategies/gates/smarttakeloss/series"
	"math"
	"slices"
	"strings"
	"testing"

	"github.com/giovani-sirbu/mercury/helpers"
	"github.com/giovani-sirbu/mercury/trades/aggragates"
)

// assertHolds fails unless the reading holds with no break, and assertMisses unless it misses
// and Breaks names each want.
func assertHolds(t *testing.T, bars Bars, side string, trend series.Trend) LineReading {
	t.Helper()
	reading := TriggerLine(bars, lineFixturePercentage, side, trend)
	if !reading.Holds {
		t.Fatalf("the reading must hold, got %+v; misses %q", reading, LineBreaks(bars, lineFixturePercentage, side, trend))
	}
	if misses := LineBreaks(bars, lineFixturePercentage, side, trend); len(misses) != 0 {
		t.Fatalf("a holding reading breaks nothing, got %q", misses)
	}
	return reading
}

func assertMisses(t *testing.T, bars Bars, side string, trend series.Trend, want ...string) {
	t.Helper()
	if reading := TriggerLine(bars, lineFixturePercentage, side, trend); reading.Holds {
		t.Fatalf("the reading must miss, got %+v", reading)
	}
	misses := LineBreaks(bars, lineFixturePercentage, side, trend)
	for _, text := range want {
		if !slices.ContainsFunc(misses, func(miss string) bool { return strings.Contains(miss, text) }) {
			t.Fatalf("the breaks %q must name %q", misses, text)
		}
	}
}

// A flat window holds for either side under its met trend and names the frame, the flat line,
// the closes against the line and the trend; a trend a row short, or reading the other way, misses
// on the trend alone, and the bars alone read it without the trend.
func TestAFlatWindowHoldsUnderTheTrend(t *testing.T) {
	bars := flatLineBars()
	for _, sc := range lineSideCases {
		t.Run(sc.name, func(t *testing.T) {
			reading := assertHolds(t, bars, sc.side, sc.met)
			if reading.Of != lineFixtureBars || reading.From != bars.Opens[0] || reading.To != bars.Opens[lineFixtureBars-1] || math.Abs(reading.Bound-fixtureBound) > 1e-12 {
				t.Fatalf("the reading frames the window and the bound, got %+v", reading)
			}
			want := []string{
				lineFrame(reading),
				lineReason(reading),
				fmt.Sprintf("closes within +0.00%%/+0.00%% of the line, at most %.2f%%", fixtureBound),
			}
			if !strings.HasPrefix(want[1], fmt.Sprintf("line flat, +0.00%% over the window within %.2f%%", fixtureBound)) {
				t.Fatalf("the line reason names a flat line within the bound, got %q", want[1])
			}
			if MinSmcTrend > 0 {
				want = append(want, fmt.Sprintf("smc trend direction %s %d/%d of %d needed", sc.trendWord, MinSmcTrend, lineTrendRows, MinSmcTrend))
			}
			if !slices.Equal(reading.Reasons, want) {
				t.Fatalf("reasons = %q, want %q", reading.Reasons, want)
			}
			if alone := TriggerLineOnBars(bars, lineFixturePercentage, sc.side); !alone.Holds || slices.ContainsFunc(alone.Reasons, func(r string) bool { return strings.Contains(r, "smc") }) {
				t.Fatalf("the bars alone hold without a trend reason, got %+v", alone)
			}
			needsATrend(t)
			assertMisses(t, bars, sc.side, sc.short, "smc trend direction "+sc.trendWord)
			assertMisses(t, bars, sc.side, sc.wrong, "smc trend direction "+sc.trendWord)
			assertMisses(t, bars, sc.side, series.Trend{}, trendUnreadBreak)
		})
	}
}

// A diagonal line holds only the ladder's way: down for a long, up for an inverse; a move within
// the bound is flat and holds either way.
func TestTheLineMustMoveTheLaddersWay(t *testing.T) {
	move := slopedMove()
	if move <= fixtureBound {
		t.Skipf("MaxSlopePercentage %v leaves no diagonal over the bound %.2f", MaxSlopePercentage, fixtureBound)
	}
	for _, sc := range lineSideCases {
		t.Run(sc.name, func(t *testing.T) {
			ownWay := lineBars(lineFixtureBars, sc.ownWay(move), nil, nil)
			reading := assertHolds(t, ownWay, sc.side, sc.met)
			if !strings.HasPrefix(reading.Reasons[1], fmt.Sprintf("line %s %.2f%% over the window", sc.trendWord, move)) {
				t.Fatalf("the line reason names the way, got %q", reading.Reasons[1])
			}
			against := lineBars(lineFixtureBars, -sc.ownWay(move), nil, nil)
			assertMisses(t, against, sc.side, sc.met, "over the window against the ladder")
			flatEnough := lineBars(lineFixtureBars, sc.ownWay(0.9*fixtureBound), nil, nil)
			if reading := assertHolds(t, flatEnough, sc.side, sc.met); !strings.HasPrefix(reading.Reasons[1], "line flat") {
				t.Fatalf("a move within the bound is flat, got %q", reading.Reasons[1])
			}
		})
	}
}

// A bar counts against the tolerance when its close leaves the band or its body spans more than the
// bound, wherever the close lies: as many as the tolerance allows hold and are named, one more
// misses on the count, and a wide body counts once with a stray close.
func TestBarsCountAgainstTheToleranceOnTheirCloseOrTheirBody(t *testing.T) {
	needsATolerance(t)
	allowed := allowedOver(lineFixtureBars)
	for _, sc := range lineSideCases {
		t.Run(sc.name, func(t *testing.T) {
			strays := strayOffsets(allowed, 1.5)
			onTheClose := lineBars(lineFixtureBars, 0, strays, strays)
			reading := assertHolds(t, onTheClose, sc.side, sc.met)
			if reading.Counted != allowed || reading.Wide != 0 || reading.Allowed != allowed || !slices.Contains(reading.Reasons, fmt.Sprintf("%d of %d bars over the bound (0 on their body), at most %d", allowed, lineFixtureBars, allowed)) {
				t.Fatalf("%d stray closes with their opens hold and are named, got %+v", allowed, reading)
			}
			oneMore := strayOffsets(allowed+1, 1.5)
			assertMisses(t, lineBars(lineFixtureBars, 0, oneMore, oneMore), sc.side, sc.met, "bars over the bound")

			onTheBody := lineBars(lineFixtureBars, 0, nil, strays)
			reading = assertHolds(t, onTheBody, sc.side, sc.met)
			if reading.Counted != allowed || reading.Wide != allowed || math.Abs(reading.MaxAbove) > 1e-9 || math.Abs(reading.MaxBelow) > 1e-9 {
				t.Fatalf("%d wide bodies closing on the line count on their body alone, got %+v", allowed, reading)
			}
			assertMisses(t, lineBars(lineFixtureBars, 0, nil, oneMore), sc.side, sc.met, "on their body, the widest")

			once := lineBars(lineFixtureBars, 0, strays, nil)
			if reading := assertHolds(t, once, sc.side, sc.met); reading.Counted != allowed || reading.Wide != allowed {
				t.Fatalf("a stray close off an open on the line counts once, on its close and its body, got %+v", reading)
			}
		})
	}
}

// The line may slope no steeper than the cap per hour, either way and however far it moves: a cap
// over the slope holds and names it, the slope itself as a cap still holds, a cap under it misses
// and names the slope, and a cap at zero or below reads none and names none.
func TestTheLineMaySlopeNoSteeperThanTheCap(t *testing.T) {
	const move = 10.0
	slope := move / float64(lineFixtureBars-1) * 60 / float64(helpers.IntervalToMinutes(Timeframe))
	for _, sc := range lineSideCases {
		t.Run(sc.name, func(t *testing.T) {
			bars := lineBars(lineFixtureBars, sc.ownWay(move), nil, nil)
			over, under := slope*1.05, slope*0.95

			held := lineAt(bars, lineFixturePercentage, sc.side, sc.met, MinSmcTrend, over)
			want := fmt.Sprintf("line %s %.2f%% over the window, %.2f%%/h under %.2f%%/h", sc.trendWord, move, slope, over)
			if !held.Holds || math.Abs(math.Abs(held.SlopePct)-slope) > 1e-9 || held.Reasons[1] != want {
				t.Fatalf("a cap over the slope holds and names it as %q, got %+v", want, held)
			}
			if exact := lineAt(bars, lineFixturePercentage, sc.side, sc.met, MinSmcTrend, math.Abs(held.SlopePct)); !exact.Holds {
				t.Fatalf("the slope as the cap still holds, got %+v", exact)
			}

			if missed := lineAt(bars, lineFixturePercentage, sc.side, sc.met, MinSmcTrend, under); missed.Holds {
				t.Fatalf("a cap under the slope must miss, got %+v", missed)
			}
			wantBreak := fmt.Sprintf("line slopes %.2f%%/h, over the %.2f%%/h allowed", slope, under)
			if breaks := lineBreakReasons(bars, lineFixturePercentage, sc.side, sc.met, MinSmcTrend, under); !slices.Contains(breaks, wantBreak) {
				t.Fatalf("the breaks %q must name %q", breaks, wantBreak)
			}

			for _, off := range []float64{0, -1} {
				read := lineAt(bars, lineFixturePercentage, sc.side, sc.met, MinSmcTrend, off)
				if !read.Holds || strings.Contains(read.Reasons[1], "/h") {
					t.Fatalf("a cap of %v reads none and names none, got %+v", off, read)
				}
				if breaks := lineBreakReasons(bars, lineFixturePercentage, sc.side, sc.met, MinSmcTrend, off); len(breaks) != 0 {
					t.Fatalf("a cap of %v breaks nothing here, got %q", off, breaks)
				}
			}
		})
	}
}

// The cap reads a flat line too: a move within the bound still slopes, and a cap under that slope
// misses it, while a flat window at zero slope holds under any cap above zero.
func TestTheSlopeCapReadsAFlatLineToo(t *testing.T) {
	for _, sc := range lineSideCases {
		t.Run(sc.name, func(t *testing.T) {
			if reading := lineAt(flatLineBars(), lineFixturePercentage, sc.side, sc.met, MinSmcTrend, 0.01); !reading.Holds {
				t.Fatalf("a window at zero slope holds under a small cap, got %+v", reading)
			}
			gentle := lineBars(lineFixtureBars, sc.ownWay(0.9*fixtureBound), nil, nil)
			slope := assertHolds(t, gentle, sc.side, sc.met).SlopePct
			if reading := lineAt(gentle, lineFixturePercentage, sc.side, sc.met, MinSmcTrend, math.Abs(slope)/2); reading.Holds {
				t.Fatalf("a flat line sloping %v%%/h must miss a cap of half of it, got %+v", slope, reading)
			}
		})
	}
}

// What cannot be read holds nothing: no percentage, bars not served (too few, unequal arrays, a
// price at zero) or a side the trend has no way for.
func TestWhatCannotBeReadHoldsNothing(t *testing.T) {
	bars := flatLineBars()
	met := lineSideCases[0].met
	if reading := TriggerLine(bars, 0, aggragates.SideLong, met); reading.Holds || reading.Bound != 0 {
		t.Fatalf("no percentage holds nothing, got %+v", reading)
	}
	zeroClose, zeroOpen := flatLineBars(), flatLineBars()
	zeroClose.Closes[3], zeroOpen.OpenPrices[3] = 0, 0
	for name, broken := range map[string]Bars{
		"too few":         lineBars(MinBars-1, 0, nil, nil),
		"unequal closes":  {Opens: bars.Opens, OpenPrices: bars.OpenPrices, Closes: bars.Closes[1:]},
		"unequal opens":   {Opens: bars.Opens, OpenPrices: bars.OpenPrices[1:], Closes: bars.Closes},
		"a close at zero": zeroClose,
		"an open at zero": zeroOpen,
		"none":            {},
	} {
		if broken.Served() {
			t.Fatalf("%s: the bars must not be served", name)
		}
		assertMisses(t, broken, aggragates.SideLong, met, barsBreak)
	}
	assertMisses(t, bars, "sideways", met, sideBreak)
	if misses := LineBreaks(bars, 0, aggragates.SideLong, met); misses[1] != percentageBreak {
		t.Fatalf("no percentage is named after the frame, got %q", misses)
	}
}

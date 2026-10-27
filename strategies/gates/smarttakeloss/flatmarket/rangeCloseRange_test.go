package flatmarket

import (
	"github.com/giovani-sirbu/mercury/strategies/gates/smarttakeloss/series"
	"math"
	"slices"
	"strconv"
	"strings"
	"testing"
	"time"

	"github.com/giovani-sirbu/mercury/trades/aggragates"
)

// closeRangeAsksFor are the asks of the span of the closes on a side: a window that
// spans within the bound, and the ways a reading misses on it, alone and with the
// bodies and the trend, in the order Breaks names them. None where the condition is
// off: no span is over a bound that is not there.
func closeRangeAsksFor(sc rangeSideCase) []ask {
	if MaxCloseRangePct <= 0 {
		return nil
	}
	win := func(quiet int, span float64) Bars {
		return window{quiet: quiet, movePct: holdMove, closeRangePct: span, side: sc.side}.build(fixturePercentage)
	}
	opening, need := frameText(rangeFixtureBars), quietNeeded(rangeFixtureBars)
	within, over, short := win(rangeFixtureBars, holdRange), win(rangeFixtureBars, overRange), win(need-1, overRange)
	return []ask{
		{"holds on closes spanning within the bound", within, fixturePercentage, sc.met, nil},
		{"closes spanning over the bound", over, fixturePercentage, sc.met, []string{opening, closeRangeBreakText(over)}},
		{
			"quiet bodies short and closes spanning over", short, fixturePercentage, sc.met,
			[]string{opening, quietText(need-1, rangeFixtureBars, "short of"), closeRangeBreakText(short)},
		},
		{
			"closes spanning over and a trend a row short", over, fixturePercentage, sc.short,
			[]string{opening, closeRangeBreakText(over), trendText(sc.trendName, sc.count(sc.short), sc.short, "short of")},
		},
		{
			"closes spanning over and a trend not served", over, fixturePercentage, series.Trend{},
			[]string{opening, closeRangeBreakText(over), "smc trend direction not read"},
		},
		{
			"a percentage not set and closes spanning over", over, 0, sc.met,
			[]string{opening, "percentage not set", closeRangeBreakText(over)},
		},
	}
}

// flatBars are two bars with one price each, the oldest at first and the newest at
// second, open, high, low and close alike: no body, no wick and no close between them,
// so that the span of their closes is the one thing they read, and a rise from the first
// to the second, which a long ladder reads as a fall of less than nothing, is a move
// under every bound. spanning is the pair for a ladder of the side: rising for a long
// ladder and falling for an inverse one, the market going the other way.
func flatBars(first, second float64) Bars {
	return Bars{
		Opens:      []int64{firstBar.UnixMilli(), firstBar.Add(time.Hour).UnixMilli()},
		OpenPrices: []float64{first, second},
		Highs:      []float64{first, second},
		Lows:       []float64{first, second},
		Closes:     []float64{first, second},
	}
}

func spanning(sc rangeSideCase, low, high float64) Bars {
	if sc.side == aggragates.SideShort {
		return flatBars(high, low)
	}
	return flatBars(low, high)
}

// boundText is the bound as a reason prints it: a tenth of a percent.
func boundText() string {
	return strconv.FormatFloat(float64(MaxCloseRangePct), 'f', 1, 64)
}

// The fixture the other tests lean on keeps its promises at the constants in force: the
// windows that miss on the bodies, the move or the trend alone span within the bound,
// so that they miss on that alone; a window asked for a span has exactly that span,
// holds where it is within the bound and misses on it and nothing else where it is
// over (its move reads the other way, its closes lie on the mean of the two ends, its
// bodies are the quiet ones the window was made of); and a window asked for none is not
// stretched.
func TestTheFixtureKeepsItsWindowsWithinTheSpanAndTheKnobStretchesOnlyThat(t *testing.T) {
	needsAMove(t)
	needsAQuietBoundary(t)
	needsACloseRange(t)
	need := quietNeeded(rangeFixtureBars)
	for _, sc := range rangeSideCases {
		for _, tc := range []struct {
			name  string
			w     window
			holds bool
		}{
			{"every bar quiet", window{quiet: rangeFixtureBars, movePct: holdMove, side: sc.side}, true},
			{"the fewest quiet", window{quiet: need, movePct: holdMove, side: sc.side}, true},
			{"a bar short", window{quiet: need - 1, movePct: holdMove, side: sc.side}, false},
			{"the market the other way", window{quiet: rangeFixtureBars, movePct: -holdMove, side: sc.side}, true},
			{"a move over", window{quiet: rangeFixtureBars, movePct: overMove, side: sc.side}, false},
			{"bodies short, a move over", window{quiet: need - 1, movePct: overMove, side: sc.side}, false},
			{"a stray within", window{quiet: rangeFixtureBars, movePct: holdMove, strayPct: holdStray, side: sc.side}, true},
			{"a span asked, within", window{quiet: rangeFixtureBars, movePct: holdMove, closeRangePct: holdRange, side: sc.side}, true},
		} {
			bars := tc.w.build(fixturePercentage)
			if _, _, pct := rangeOf(bars); !closeRangeBounded(pct) {
				t.Errorf("%s, %s: the closes span %v, not within the bound %v: the fixture needs recentring", sc.name, tc.name, pct, MaxCloseRangePct)
			}
			if reading := TriggerRange(bars, fixturePercentage, sc.side, sc.met); reading.Holds != tc.holds {
				t.Errorf("%s, %s: holds %v, want %v: breaks %q", sc.name, tc.name, reading.Holds, tc.holds, RangeBreaks(bars, fixturePercentage, sc.side, sc.met))
			}
			if got := spanBreaks(bars); got != nil {
				t.Errorf("%s, %s: a window that misses on something else must not miss on the span, got %q", sc.name, tc.name, got)
			}
		}

		for _, span := range []float64{holdRange, overRange} {
			bars := window{quiet: rangeFixtureBars, movePct: holdMove, closeRangePct: span, side: sc.side}.build(fixturePercentage)
			low, high, pct := rangeOf(bars)
			assertNear(t, sc.name+" span asked", pct, span, 1e-9)
			if low != slices.Min(bars.Closes) || high != slices.Max(bars.Closes) || low <= 0 {
				t.Fatalf("%s: the span %v is not of the lowest and the highest close", sc.name, span)
			}
			if got := bars.quietBodies(BodyShare * fixturePercentage); got != rangeFixtureBars {
				t.Errorf("%s, span %v: %d quiet bodies, want all %d the window is made of", sc.name, span, got, rangeFixtureBars)
			}
			if move := moveOf(bars, sc.side); !moveBounded(move) {
				t.Errorf("%s, span %v: a move of %v is over the bound, the span must be all that misses", sc.name, span, move)
			}
			_, stray, _ := closesAgainstMean(bars)
			assertNear(t, sc.name+" stray", stray, 0, 1e-9)

			reading, breaks := TriggerRange(bars, fixturePercentage, sc.side, sc.met), RangeBreaks(bars, fixturePercentage, sc.side, sc.met)
			if span == holdRange && (!reading.Holds || breaks != nil) {
				t.Errorf("%s, span %v: within the bound, want a hold, got %+v and breaks %q", sc.name, span, reading, breaks)
			}
			want := []string{frameText(rangeFixtureBars), closeRangeBreakText(bars)}
			if span == overRange && (reading.Holds || !slices.Equal(breaks, want)) {
				t.Errorf("%s, span %v: over the bound, want a miss on it alone, got %+v and breaks\n got %q\nwant %q", sc.name, span, reading, breaks, want)
			}
		}

		unasked := window{quiet: rangeFixtureBars, movePct: holdMove, side: sc.side}.build(fixturePercentage)
		if !slices.Equal(window{quiet: rangeFixtureBars, movePct: holdMove, closeRangePct: -1, side: sc.side}.build(fixturePercentage).Closes, unasked.Closes) {
			t.Errorf("%s: a span of zero or below must not stretch the window", sc.name)
		}
	}
}

// The span of the closes is the lowest close, the highest, and the highest over the
// lowest in percent of the lowest, whichever bars they belong to: the oldest and the
// newest are read like any other, the extremes may be at either end, between, or the
// same close, and nothing but the closes is read.
func TestTheCloseRangeIsTheHighestCloseOverTheLowestAnywhereInTheWindow(t *testing.T) {
	for _, tc := range []struct {
		name            string
		closes          []float64
		low, high, want float64
	}{
		{"the extremes between", []float64{100, 99, 103, 100}, 99, 103, 400.0 / 99},
		{"the lowest the oldest, the highest the newest", []float64{100, 101, 102}, 100, 102, 2},
		{"the highest the oldest, the lowest the newest", []float64{102, 101, 100}, 100, 102, 2},
		{"both extremes at the ends, the lowest the newest", []float64{110, 105, 100.5, 100}, 100, 110, 10},
		{"the lowest the oldest, the highest between", []float64{100, 108, 101, 104}, 100, 108, 8},
		{"the highest the newest, the lowest between", []float64{104, 98, 101, 106}, 98, 106, 800.0 / 98},
		{"two bars", []float64{50, 60}, 50, 60, 20},
		{"one bar", []float64{100}, 100, 100, 0},
		{"every close alike", []float64{100, 100, 100, 100}, 100, 100, 0},
		{"a share of the lowest, not the highest", []float64{200, 100, 150}, 100, 200, 100},
	} {
		bars := Bars{
			Opens:      make([]int64, len(tc.closes)),
			OpenPrices: make([]float64, len(tc.closes)),
			Highs:      make([]float64, len(tc.closes)),
			Lows:       make([]float64, len(tc.closes)),
			Closes:     tc.closes,
		}
		for index := range tc.closes {
			bars.Opens[index] = int64(index+1) * 1000
			bars.OpenPrices[index] = float64(index+1) * 1e6
			bars.Highs[index] = float64(index+1) * 2e6
			bars.Lows[index] = float64(index+1) * 1e-6
		}
		low, high, pct := bars.closeRange()
		assertNear(t, tc.name+" lowest", low, tc.low, 1e-12)
		assertNear(t, tc.name+" highest", high, tc.high, 1e-12)
		assertNear(t, tc.name+" span", pct, tc.want, 1e-9)
		if allocs := testing.AllocsPerRun(100, func() { bars.closeRange() }); allocs != 0 {
			t.Errorf("%s: the span allocated %v times, want none", tc.name, allocs)
		}
	}
}

// Bars with no close have no range, and a lowest close at or under zero has no share to
// take: the range is then zero, which is within any bound, whatever the highest is.
func TestNoCloseAndNoPositiveLowestCloseHaveNoRange(t *testing.T) {
	if low, high, pct := (Bars{}).closeRange(); low != 0 || high != 0 || pct != 0 {
		t.Errorf("no bar: the range is %v from %v to %v, want none", pct, low, high)
	}
	if low, high, pct := (Bars{Closes: []float64{}}).closeRange(); low != 0 || high != 0 || pct != 0 {
		t.Errorf("an empty close array: the range is %v from %v to %v, want none", pct, low, high)
	}
	for _, tc := range []struct {
		name      string
		closes    []float64
		low, high float64
	}{
		{"a lowest close of zero", []float64{5, 0, 7}, 0, 7},
		{"a lowest close under zero", []float64{5, -2, 7}, -2, 7},
		{"a lowest close of zero, the only one", []float64{0}, 0, 0},
	} {
		low, high, pct := Bars{Closes: tc.closes}.closeRange()
		if low != tc.low || high != tc.high || pct != 0 || !closeRangeBounded(pct) {
			t.Errorf("%s: %v from %v to %v, want the extremes read and a range of zero, within the bound", tc.name, pct, low, high)
		}
	}
}

// A range is within the bound with the bound itself included, and one that is not a
// number or is infinite is not within it. The condition is not special-cased below the
// bound: a range of nothing is within it.
func TestTheCloseRangeBoundIsIncludedAndNaNMisses(t *testing.T) {
	needsACloseRange(t)
	bound := float64(MaxCloseRangePct)
	over := math.Nextafter(bound, math.Inf(1))
	for _, tc := range []struct {
		name string
		span float64
		want bool
	}{
		{"nothing", 0, true},
		{"under the bound", math.Nextafter(bound, math.Inf(-1)), true},
		{"at the bound", bound, true},
		{"just over the bound", over, false},
		{"far over the bound", 2 * bound, false},
		{"not a number", math.NaN(), false},
		{"infinite", math.Inf(1), false},
	} {
		if got := closeRangeBounded(tc.span); got != tc.want {
			t.Errorf("%s: closeRangeBounded(%v) = %v, want %v", tc.name, tc.span, got, tc.want)
		}
	}
}

// The bound is read through the whole reading: closes spanning exactly the bound hold,
// closes a step under hold, closes a step over miss, on both sides, with the reason
// that matches, and the bound is the one the span of the closes alone is held to: the
// window has no body, no wick, no stray and no move that could miss.
func TestClosesSpanningExactlyTheBoundHoldAndAStepOverMiss(t *testing.T) {
	needsACloseRange(t)
	const low = 100.0
	bound := float64(MaxCloseRangePct)
	atBound := low * (1 + bound/100)
	if _, _, pct := rangeOf(flatBars(low, atBound)); pct != bound {
		t.Skipf("a span from %v to %v does not land on the bound %v in floating point, but on %v", low, atBound, bound, pct)
	}
	for _, sc := range rangeSideCases {
		for _, tc := range []struct {
			name  string
			high  float64
			holds bool
		}{
			{"a step under the bound", math.Nextafter(atBound, math.Inf(-1)), true},
			{"at the bound", atBound, true},
			{"a step over the bound", math.Nextafter(atBound, math.Inf(1)), false},
		} {
			bars := spanning(sc, low, tc.high)
			reading := TriggerRange(bars, fixturePercentage, sc.side, sc.met)
			breaks := RangeBreaks(bars, fixturePercentage, sc.side, sc.met)
			if reading.Holds != tc.holds || (len(breaks) == 0) != tc.holds {
				t.Errorf("%s, %s: holds %v with breaks %q, want holds %v", sc.name, tc.name, reading.Holds, breaks, tc.holds)
			}
			if tc.holds {
				if want := closeRangeHoldText(bars); len(reading.Reasons) < 5 || reading.Reasons[4] != want {
					t.Errorf("%s, %s: reasons %q, want the fifth to be %q", sc.name, tc.name, reading.Reasons, want)
				}
				continue
			}
			if want := []string{frameText(2), closeRangeBreakText(bars)}; !slices.Equal(breaks, want) {
				t.Errorf("%s, %s: breaks\n got %q\nwant %q", sc.name, tc.name, breaks, want)
			}
			assertNear(t, sc.name+" span carried", reading.CloseRangePct, (tc.high-low)/low*100, 1e-9)
		}
	}
}

// The reasons say how far apart the closes lie, from which close to which, against the
// bound printed with a tenth of a percent: the whole-number bound is an untyped constant
// and prints as a number nonetheless. The prices print as the mean's do, rounded to the
// eighth decimal in the shortest form.
func TestTheSpanReasonsNameTheLowestTheHighestAndTheBound(t *testing.T) {
	needsACloseRange(t)
	bound := boundText()
	for _, sc := range rangeSideCases {
		for _, tc := range []struct {
			name      string
			low, high float64
			text      string
			holds     bool
		}{
			{"a whole span", 100, 101, "closes span 1.0% from 100 to 101, at most " + bound + "%", true},
			{"prices rounded to the eighth decimal", 100, 100.50000000004, "closes span 0.5% from 100 to 100.5, at most " + bound + "%", true},
			{"a span over", 100, 103, "closes span 3.0% from 100 to 103 over " + bound + "%", false},
		} {
			low, high := tc.low, tc.high
			if tc.holds != (((high-low)/low)*100 <= float64(MaxCloseRangePct)) {
				continue
			}
			bars := spanning(sc, low, high)
			reading, breaks := TriggerRange(bars, fixturePercentage, sc.side, sc.met), RangeBreaks(bars, fixturePercentage, sc.side, sc.met)
			if tc.holds {
				if !reading.Holds || len(reading.Reasons) < 5 || reading.Reasons[4] != tc.text {
					t.Errorf("%s, %s: reasons %q, want a hold whose fifth reads %q", sc.name, tc.name, reading.Reasons, tc.text)
				}
				continue
			}
			if reading.Holds || !slices.Equal(breaks, []string{frameText(2), tc.text}) {
				t.Errorf("%s, %s: want a miss with breaks %q, got %+v and %q", sc.name, tc.name, []string{frameText(2), tc.text}, reading, breaks)
			}
		}
	}
}

// The span is read of the closes alone and of no side: the same bars name it alike for a
// long ladder and an inverse one, for a ladder whose side is not read, and on the bars
// alone, before the side and the trend.
func TestTheSpanOfTheClosesIsNamedAlikeOnEverySide(t *testing.T) {
	needsAMove(t)
	needsAQuietBoundary(t)
	needsACloseRange(t)
	for _, sc := range rangeSideCases {
		bars := window{quiet: rangeFixtureBars, movePct: holdMove, closeRangePct: overRange, side: sc.side}.build(fixturePercentage)
		reason := closeRangeBreakText(bars)
		for _, other := range rangeSideCases {
			if got := RangeBreaks(bars, fixturePercentage, other.side, other.met); !slices.Contains(got, reason) {
				t.Errorf("the bars of %s read as %s: breaks %q must name %q", sc.name, other.name, got, reason)
			}
			if got := RangeBreaksOnBars(bars, fixturePercentage, other.side); !slices.Contains(got, reason) {
				t.Errorf("the bars of %s read on the bars alone as %s: breaks %q must name %q", sc.name, other.name, got, reason)
			}
		}
		unread := RangeBreaks(bars, fixturePercentage, "", sc.met)
		if want := []string{frameText(rangeFixtureBars), reason, "side not read"}; !slices.Equal(unread, want) {
			t.Errorf("%s, side not read: breaks\n got %q\nwant %q", sc.name, unread, want)
		}
		low, high, pct := bars.closeRange()
		wantLow, wantHigh, wantPct := rangeOf(bars)
		assertNear(t, sc.name+" lowest", low, wantLow, 1e-12)
		assertNear(t, sc.name+" highest", high, wantHigh, 1e-12)
		assertNear(t, sc.name+" span", pct, wantPct, 1e-9)
	}
}

// A hold names the span of the closes in the fifth slot, after the closes against their
// mean and before the trend, in a list made once and exactly the size of the cap, six; on
// the bars alone the list is five in the same room, and in neither does a reason grow it.
// A miss lists the same slots in the same order: the span after the stray and before the
// side and the trend, the bars not served leaving nothing to span.
func TestTheSpanTakesItsSlotAfterTheStrayAndBeforeTheTrendAndTheCapHolds(t *testing.T) {
	needsAMove(t)
	needsAQuietBoundary(t)
	needsACloseRange(t)
	needsTheTrend(t)
	if rangeHoldReasonCap != 6 {
		t.Errorf("rangeHoldReasonCap is %d, want the six reasons of a hold: frame, bodies, move, stray, span, trend", rangeHoldReasonCap)
	}
	need := quietNeeded(rangeFixtureBars)
	for _, sc := range rangeSideCases {
		bars := holding(sc.side)
		full := TriggerRange(bars, fixturePercentage, sc.side, sc.met).Reasons
		if len(full) != 6 || cap(full) != rangeHoldReasonCap {
			t.Fatalf("%s: %d reasons in room for %d, want 6 in room for %d: %q", sc.name, len(full), cap(full), rangeHoldReasonCap, full)
		}
		for slot, opening := range []string{"range on 1h bars ", "quiet bodies ", sc.moveName + " ", "closes within ", "closes span ", trendPrefix} {
			if !strings.HasPrefix(full[slot], opening) {
				t.Errorf("%s: reason %d is %q, want it opening %q", sc.name, slot, full[slot], opening)
			}
		}
		if full[4] != closeRangeHoldText(bars) {
			t.Errorf("%s: the fifth reason is %q, want %q", sc.name, full[4], closeRangeHoldText(bars))
		}
		onBars := TriggerRangeOnBars(bars, fixturePercentage, sc.side).Reasons
		if len(onBars) != 5 || cap(onBars) != rangeHoldReasonCap || !strings.HasPrefix(onBars[4], "closes span ") {
			t.Errorf("%s: on the bars alone %d reasons in room for %d, want 5 in room for %d ending on the span: %q", sc.name, len(onBars), cap(onBars), rangeHoldReasonCap, onBars)
		}

		// quiet bodies short, a move over, a stray, a span over and a trend short: all but the side.
		every := window{quiet: need - 1, movePct: overMove, strayPct: overStray, side: sc.side}.build(fixturePercentage)
		want := []string{
			frameText(rangeFixtureBars),
			quietText(need-1, rangeFixtureBars, "short of"),
			moveText(sc.moveName, moveOf(every, sc.side), "over"),
			strayBreakText(every),
			closeRangeBreakText(every),
			trendText(sc.trendName, sc.count(sc.short), sc.short, "short of"),
		}
		if got := RangeBreaks(every, fixturePercentage, sc.side, sc.short); !slices.Equal(got, want) {
			t.Errorf("%s: breaks\n got %q\nwant %q", sc.name, got, want)
		}
		if got := RangeBreaksOnBars(every, fixturePercentage, sc.side); !slices.Equal(got, want[:5]) {
			t.Errorf("%s: breaks on the bars alone\n got %q\nwant %q", sc.name, got, want[:5])
		}
		unread := []string{want[0], want[1], want[3], want[4], "side not read"}
		if got := RangeBreaks(every, fixturePercentage, "", sc.short); !slices.Equal(got, unread) {
			t.Errorf("%s, side not read: breaks\n got %q\nwant %q", sc.name, got, unread)
		}
		if got := RangeBreaks(Bars{}, fixturePercentage, sc.side, sc.met); !slices.Equal(got, []string{"bars not served"}) {
			t.Errorf("%s: bars not served, want them alone, got %q", sc.name, got)
		}
	}
}

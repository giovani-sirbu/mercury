package flatmarket

import (
	"fmt"
	"math"
	"slices"
	"strings"
	"testing"
)

// recentred is the window's bars with every close between the first and the last
// brought onto the mean of those two, but for the middle bar's, which is brought
// onto the stray the window asks (none when zero). The four prices of a bar are
// scaled alike, so a body keeps its share of its own open and the quiet and loud
// counts stand, and the first bar and the last are never touched, so the move
// stands too: the stray is exactly the one asked, above or below, under any retune,
// and not whatever the alternating bodies happen to leave. A window of fewer than
// three bars has no close between, and nothing to ask of.
func (w window) recentred(bars Bars) Bars {
	of := len(bars.Closes)
	mean := (bars.Closes[0] + bars.Closes[of-1]) / 2
	for index := 1; index < of-1; index++ {
		target := mean
		if index == of/2 {
			target = mean * (1 + w.strayPct/100)
		}
		scale := target / bars.Closes[index]
		bars.OpenPrices[index] *= scale
		bars.Highs[index] *= scale
		bars.Lows[index] *= scale
		bars.Closes[index] *= scale
	}
	return bars
}

// closesAgainstMean is the stray of the bars written out against the rule: the mean of
// the first and the last close, the close between them farthest from it in percent of
// the mean and signed, and the index of its bar, zero when no close lies off the mean.
// A reason names the number the measurement made, not the one the window was asked,
// which the rounding of a tenth of a percent could tell apart.
func closesAgainstMean(bars Bars) (mean, pct float64, index int) {
	last := len(bars.Closes) - 1
	mean = (bars.Closes[0] + bars.Closes[last]) / 2
	for at := 1; at < last; at++ {
		if deviation := (bars.Closes[at] - mean) / mean * 100; math.Abs(deviation) > math.Abs(pct) {
			pct, index = deviation, at
		}
	}
	return mean, pct, index
}

// meanString is a mean as a reason prints it, written out against the grammar: eight
// decimals, the zeros at the end and a dot with nothing after it cut off.
func meanString(mean float64) string {
	return strings.TrimSuffix(strings.TrimRight(fmt.Sprintf("%.8f", mean), "0"), ".")
}

func strayHoldText(bars Bars) string {
	mean, pct, _ := closesAgainstMean(bars)
	return fmt.Sprintf(
		"closes within %.1f%% of the mean %s, at most %.1f%%",
		math.Abs(pct), meanString(mean), float64(MaxMovePct),
	)
}

func strayBreakText(bars Bars) string {
	mean, pct, index := closesAgainstMean(bars)
	where := "above"
	if pct < 0 {
		where = "below"
	}
	return fmt.Sprintf(
		"close at %s strays %.1f%% %s the mean %s over %.1f%%",
		stamp(index), math.Abs(pct), where, meanString(mean), float64(MaxMovePct),
	)
}

// strayAsksFor are the asks of the stray on a side: windows that stray within the
// bound, above and below, and the ways a reading misses on it, alone and with the
// others, in the order Breaks names them. A close that strays over the bound spans the
// closes at least as far (the span is the highest over the lowest, the stray the
// farthest from their mean), so the windows that stray over it name the span of their
// closes too where that is over its own bound (spanBreaks), after the stray.
func strayAsksFor(sc rangeSideCase) []ask {
	win := func(quiet int, move, stray float64) Bars {
		return window{quiet: quiet, movePct: move, strayPct: stray, side: sc.side}.build(fixturePercentage)
	}
	opening, need := frameText(rangeFixtureBars), quietNeeded(rangeFixtureBars)
	above, below := win(rangeFixtureBars, holdMove, overStray), win(rangeFixtureBars, holdMove, -overStray)
	all := win(need-1, overMove, overStray)
	quietShort := quietText(need-1, rangeFixtureBars, "short of")
	moveOver := moveText(sc.moveName, moveOf(all, sc.side), "over")
	twoBars := window{of: 2, quiet: 2, movePct: holdMove, strayPct: overStray, side: sc.side}.build(fixturePercentage)
	return []ask{
		{"holds on a stray above the mean", win(rangeFixtureBars, holdMove, holdStray), fixturePercentage, sc.met, nil},
		{"holds on a stray below the mean", win(rangeFixtureBars, holdMove, -holdStray), fixturePercentage, sc.met, nil},
		{
			"a close straying above the mean", above, fixturePercentage, sc.met,
			slices.Concat([]string{opening, strayBreakText(above)}, spanBreaks(above)),
		},
		{
			"a close straying below the mean", below, fixturePercentage, sc.met,
			slices.Concat([]string{opening, strayBreakText(below)}, spanBreaks(below)),
		},
		{
			"quiet bodies short, a move over and a close straying", all, fixturePercentage, sc.met,
			slices.Concat([]string{opening, quietShort, moveOver, strayBreakText(all)}, spanBreaks(all)),
		},
		{
			"every condition missing", all, fixturePercentage, sc.short,
			slices.Concat(
				[]string{opening, quietShort, moveOver, strayBreakText(all)}, spanBreaks(all),
				[]string{trendText(sc.trendName, sc.count(sc.short), sc.short, "short of")},
			),
		},
		{"two bars, no close between them", twoBars, fixturePercentage, sc.met, nil},
	}
}

// The stray is the close between the oldest bar and the newest that lies farthest from
// the mean of those two bars' closes, in percent of the mean and signed. Only the
// closes are read, never the first and last as a candidate, and of two as far off the
// earlier bar is the stray. The other fields of the bars are numbers it must not read.
func TestTheStrayIsTheInnerCloseFarthestFromTheMeanOfTheFirstAndLast(t *testing.T) {
	for _, tc := range []struct {
		name      string
		closes    []float64
		wantMean  float64
		wantPct   float64
		wantIndex int
	}{
		{"above the mean", []float64{100, 110, 102, 100}, 100, 10, 1},
		{"below the mean", []float64{100, 98, 90, 100}, 100, -10, 2},
		{"the farthest, on whichever side", []float64{100, 103, 94, 101, 100}, 100, -6, 2},
		{"against the mean of the two, not either alone", []float64{100, 101, 120}, 110, -900.0 / 110, 1},
		{"the first and the last are never a stray", []float64{100, 150, 200}, 150, 0, 0},
		{"two bars have no close between them", []float64{100, 120}, 110, 0, 0},
		{"a close on the mean is no stray", []float64{100, 100, 100, 100}, 100, 0, 0},
		{"a tie, the earlier below", []float64{100, 95, 105, 100}, 100, -5, 1},
		{"a tie, the earlier above", []float64{100, 105, 95, 100}, 100, 5, 1},
		{"a tie on one side", []float64{100, 105, 105, 100}, 100, 5, 1},
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
		mean, pct, at := bars.stray()
		wantAt := int64(0)
		if tc.wantIndex != 0 {
			wantAt = bars.Opens[tc.wantIndex]
		}
		assertNear(t, tc.name+" mean", mean, tc.wantMean, 1e-12)
		assertNear(t, tc.name+" stray", pct, tc.wantPct, 1e-9)
		if at != wantAt {
			t.Errorf("%s: the stray is at %d, want %d", tc.name, at, wantAt)
		}
		if allocs := testing.AllocsPerRun(100, func() { bars.stray() }); allocs != 0 {
			t.Errorf("%s: the stray allocated %v times, want none", tc.name, allocs)
		}
	}
}

// A stray is within the bound on either side of the mean, the bound itself included,
// where the move is under it and the quiet share over it. A stray that is not a number
// is not within it.
func TestTheStrayBoundIsIncludedOnBothSidesAndNaNMisses(t *testing.T) {
	bound := float64(MaxMovePct)
	over := math.Nextafter(bound, math.Inf(1))
	for _, tc := range []struct {
		name  string
		stray float64
		want  bool
	}{
		{"on the mean", 0, true},
		{"at the bound above", bound, true},
		{"at the bound below", -bound, true},
		{"just over the bound above", over, false},
		{"just over the bound below", -over, false},
		{"not a number", math.NaN(), false},
		{"infinite above", math.Inf(1), false},
		{"infinite below", math.Inf(-1), false},
	} {
		if got := strayBounded(tc.stray); got != tc.want {
			t.Errorf("%s: strayBounded(%v) = %v, want %v", tc.name, tc.stray, got, tc.want)
		}
	}
}

// A mean is printed in the shortest form of itself rounded to the eighth decimal, so
// that the noise of a division never prints.
func TestAMeanIsPrintedRoundedToTheEighthDecimalInItsShortestForm(t *testing.T) {
	for _, tc := range []struct {
		mean float64
		want string
	}{
		{16.21, "16.21"},
		{100, "100"},
		{0.1 + 0.2, "0.3"},
		{99.123456789, "99.12345679"},
		{1234567.891234567, "1234567.89123457"},
		{0.00000001, "0.00000001"},
		{2.5e-9, "0"},
	} {
		if got := meanText(tc.mean); got != tc.want {
			t.Errorf("meanText(%v) = %q, want %q", tc.mean, got, tc.want)
		}
	}
}

// A recentred window strays exactly as asked and no more, under both the quiet counts
// the window is made of, and keeps the move it was asked: every close between but the
// middle bar's lies on the mean.
func TestARecentredWindowStraysExactlyAsAskedAndKeepsItsBodiesAndMove(t *testing.T) {
	needsAMove(t)
	needsAQuietBoundary(t)
	for _, sc := range rangeSideCases {
		for _, quiet := range []int{rangeFixtureBars, quietNeeded(rangeFixtureBars)} {
			for _, asked := range []float64{0, holdStray, -holdStray, overStray, -overStray} {
				bars := window{quiet: quiet, movePct: holdMove, strayPct: asked, side: sc.side}.build(fixturePercentage)
				mean, pct, index := closesAgainstMean(bars)
				assertNear(t, sc.name+" stray", pct, asked, 1e-9)
				assertNear(t, sc.name+" move", moveOf(bars, sc.side), holdMove, 1e-9)
				if asked != 0 && index != rangeFixtureBars/2 {
					t.Errorf("%s, asked %v: the stray is at bar %d, want the middle bar %d", sc.name, asked, index, rangeFixtureBars/2)
				}
				for at := 1; at < rangeFixtureBars-1; at++ {
					if at != rangeFixtureBars/2 {
						assertNear(t, fmt.Sprintf("%s close %d off the mean", sc.name, at), (bars.Closes[at]-mean)/mean*100, 0, 1e-9)
					}
				}
				if got := bars.quietBodies(BodyShare * fixturePercentage); got != quiet {
					t.Errorf("%s, asked %v: %d quiet bodies, want the %d the window is made of", sc.name, asked, got, quiet)
				}
			}
		}
	}
}

// A reading that held carries the mean and the stray, and names how far the closes
// stray and from which mean, whichever side of the mean they stray to and whichever
// side the ladder is.
func TestAHoldCarriesTheStrayAndNamesItWithTheMean(t *testing.T) {
	needsAMove(t)
	needsAQuietBoundary(t)
	for _, sc := range rangeSideCases {
		for _, asked := range []float64{holdStray, -holdStray} {
			bars := window{quiet: rangeFixtureBars, movePct: holdMove, strayPct: asked, side: sc.side}.build(fixturePercentage)
			mean, _, index := closesAgainstMean(bars)
			reading := TriggerRange(bars, fixturePercentage, sc.side, sc.met)
			if !reading.Holds {
				t.Fatalf("%s, a stray of %v: must hold, got breaks %q", sc.name, asked, RangeBreaks(bars, fixturePercentage, sc.side, sc.met))
			}
			assertNear(t, sc.name+" mean", reading.Mean, mean, 1e-12)
			assertNear(t, sc.name+" stray", reading.StrayPct, asked, 1e-9)
			if reading.StrayAt != bars.Opens[index] {
				t.Errorf("%s, a stray of %v: at %d, want %d", sc.name, asked, reading.StrayAt, bars.Opens[index])
			}
			text, want := reading.Reasons[3], strayHoldText(bars)
			if text != want || !strings.Contains(text, meanString(mean)) || strings.Contains(text, "within 0.0%") {
				t.Errorf("%s, a stray of %v: the stray reads %q, want %q naming a real stray and the mean", sc.name, asked, text, want)
			}
		}
	}
}

// The break names the bar that strays, how far and on which side of the mean, over the
// bound, with the side of the mean in its word. It reads no side: the same bars have it
// named alike for a long ladder and an inverse one, and for a ladder whose side is not
// read, before the side. The closes of such a window span at least as far, so the span
// is named after it where it is over its own bound (spanBreaks), as the rule reads it.
func TestAStrayOverTheBoundNamesTheBarHowFarAndOnWhichSide(t *testing.T) {
	needsAMove(t)
	needsAQuietBoundary(t)
	for _, sc := range rangeSideCases {
		for _, tc := range []struct {
			asked float64
			where string
		}{{overStray, "above"}, {-overStray, "below"}} {
			bars := window{quiet: rangeFixtureBars, movePct: holdMove, strayPct: tc.asked, side: sc.side}.build(fixturePercentage)
			reading := TriggerRange(bars, fixturePercentage, sc.side, sc.met)
			mean, _, _ := closesAgainstMean(bars)
			if reading.Holds || reading.Reasons != nil || reading.StrayAt != bars.Opens[rangeFixtureBars/2] {
				t.Errorf("%s, a stray of %v: want a miss on the middle bar with no text, got %+v", sc.name, tc.asked, reading)
			}
			assertNear(t, sc.name+" stray", reading.StrayPct, tc.asked, 1e-9)
			assertNear(t, sc.name+" mean", reading.Mean, mean, 1e-12)
			breaks := RangeBreaks(bars, fixturePercentage, sc.side, sc.met)
			named := " " + tc.where + " the mean " + meanString(mean) + " over "
			wantBreaks := slices.Concat([]string{frameText(rangeFixtureBars), strayBreakText(bars)}, spanBreaks(bars))
			if !slices.Equal(breaks, wantBreaks) || !strings.Contains(breaks[1], named) {
				t.Errorf("%s, a stray of %v: breaks\n got %q\nwant the frame, a close %s the mean and then the span: %q", sc.name, tc.asked, breaks, tc.where, wantBreaks)
			}
			for _, other := range rangeSideCases {
				if got := RangeBreaks(bars, fixturePercentage, other.side, other.met); !slices.Contains(got, strayBreakText(bars)) {
					t.Errorf("the bars of %s, a stray of %v, read as %s: breaks %q must name the stray", sc.name, tc.asked, other.name, got)
				}
			}
			unread := RangeBreaks(bars, fixturePercentage, "", sc.met)
			want := slices.Concat([]string{frameText(rangeFixtureBars), strayBreakText(bars)}, spanBreaks(bars), []string{"side not read"})
			if !slices.Equal(unread, want) {
				t.Errorf("%s, a stray of %v, side not read: breaks\n got %q\nwant %q", sc.name, tc.asked, unread, want)
			}
		}
	}
}

// Three bars have one close between them and it is the stray, and two have none: the
// stray is zero at no bar and holds, the mean the two make still read and named.
func TestTheFewestBarsHaveOneCloseBetweenThemOrNone(t *testing.T) {
	needsAMove(t)
	needsAQuietBoundary(t)
	for _, sc := range rangeSideCases {
		three := window{of: 3, quiet: 3, movePct: holdMove, strayPct: -overStray, side: sc.side}.build(fixturePercentage)
		reading := TriggerRange(three, fixturePercentage, sc.side, sc.met)
		if reading.Holds || reading.StrayAt != three.Opens[1] {
			t.Errorf("%s: three bars, the one close between strays: want a miss at bar one, got %+v", sc.name, reading)
		}
		assertNear(t, sc.name+" stray of three", reading.StrayPct, -overStray, 1e-9)
		if MinBars > 2 {
			continue
		}
		two := window{of: 2, quiet: 2, movePct: holdMove, strayPct: overStray, side: sc.side}.build(fixturePercentage)
		reading = TriggerRange(two, fixturePercentage, sc.side, sc.met)
		if !reading.Holds || reading.StrayPct != 0 || reading.StrayAt != 0 {
			t.Fatalf("%s: two bars have no close to stray, want a hold at zero, got %+v", sc.name, reading)
		}
		assertNear(t, sc.name+" mean of two", reading.Mean, (two.Closes[0]+two.Closes[1])/2, 1e-12)
		reasons, named := reading.Reasons, "within 0.0% of the mean "+meanString(reading.Mean)
		if reasons[3] != strayHoldText(two) || !strings.Contains(reasons[3], named) {
			t.Errorf("%s: two bars name the mean and no stray, got %q", sc.name, reasons)
		}
	}
}

// A hold names its six reasons, the frame, the quiet bodies, the move, the stray, the span
// of the closes and the trend, in a list made once and exactly that size, so that naming
// the stray and the span too grows nothing (rangeHoldReasonCap).
func TestAHoldNamesItsSixReasonsInAListOfExactlyThatSize(t *testing.T) {
	needsAMove(t)
	needsAQuietBoundary(t)
	needsTheTrend(t)
	needsACloseRange(t)
	for _, sc := range rangeSideCases {
		reasons := TriggerRange(holding(sc.side), fixturePercentage, sc.side, sc.met).Reasons
		if len(reasons) != 6 || len(reasons) != rangeHoldReasonCap || cap(reasons) != rangeHoldReasonCap {
			t.Errorf("%s: %d reasons in room for %d, want 6 (rangeHoldReasonCap %d) in room for as many: %q", sc.name, len(reasons), cap(reasons), rangeHoldReasonCap, reasons)
		}
	}
}

// holdStray is a stray well within MaxMovePct and overStray one well over it, in percent of the
// mean, above it when positive and below it when negative: a retune moves them with the bound, as
// holdMove and overMove are moved.
var (
	holdStray = 0.5 * MaxMovePct
	overStray = 1.5 * MaxMovePct
)

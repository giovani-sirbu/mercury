package flatmarket

import (
	"fmt"
	"math"
	"slices"
	"strings"
	"testing"
	"time"
)

const (
	// userExampleStrayAt is the open time (UTC, stampLayout) of the bar of userExample
	// whose close strays, and userExampleMean the mean of the oldest close and the newest
	// the stray is measured against, as the tape's prices make it.
	userExampleStrayAt = "2022-02-17 23:00"
	userExampleMean    = 16.21
	// userExampleLow, userExampleHigh and userExampleSpanPct are the lowest close of the
	// window, the highest and the highest over the lowest in percent of the lowest, as the
	// tape's prices make them: the lowest is the close that strays (userExampleStrayAt)
	// and the highest the close of the bar two after the oldest, neither the first close
	// nor the last, which lie in between (16.46 and 15.96).
	userExampleLow     = 15.59
	userExampleHigh    = 16.57
	userExampleSpanPct = 6.29
)

// userExample is the window the stray was asked for: the lookbackBars 1h bars of LINKUSDT
// ending at the bar opening at the stamp, a market that fell from the high of its oldest
// bar to the low of its newest, quiet in all its bodies but one, whose close in the middle
// of the fall lies well below the mean of its oldest close and its newest. Measured at the
// constants rangeMeasuredAt names, to the hundredth of a percent: 23 quiet bars, a fall of 4.61
// and a stray of -3.82 (below the mean), so that it holds there, where at a bound under
// both it misses on the fall and on the stray. It is one of holdingReplays.
var userExample = replay{"linkusdt-1h-2022-02.json", "2022-02-18 09:00", 2.4, 23, 4.61, -3.82}

// quietOf is how many bodies of the bars lie under the limit, in percent of the bar's
// open, written out against the rule.
func quietOf(bars Bars, limit float64) int {
	quiet := 0
	for index, open := range bars.OpenPrices {
		if math.Abs(bars.Closes[index]-open)/open*100 < limit {
			quiet++
		}
	}
	return quiet
}

// userExampleBreaks is what Breaks names of the example window for a ladder of the side,
// written out against the rule at whatever the constants are: the frame and then, in the
// order the conditions are read, each one the window misses, and nothing when it holds.
// The span of its closes, 6.29%, is named after the stray where it is over its bound.
// The trend is met in every call, so it is never named.
func userExampleBreaks(t *testing.T, bars Bars, sc rangeSideCase) []string {
	t.Helper()
	limit := BodyShare * userExample.percentage
	var misses []string
	if quiet := quietOf(bars, limit); !(float64(quiet)/lookbackBars > QuietShare) {
		misses = append(misses, fmt.Sprintf(
			"quiet bodies %d/%d under %.2f%%, short of %.0f%%", quiet, lookbackBars, limit, float64(QuietShare)*100,
		))
	}
	if move := moveOf(bars, sc.side); !(move < MaxMovePct) {
		misses = append(misses, moveText(sc.moveName, move, "over"))
	}
	if mean, pct, _ := closesAgainstMean(bars); !(math.Abs(pct) <= MaxMovePct) {
		where := "above"
		if pct < 0 {
			where = "below"
		}
		misses = append(misses, fmt.Sprintf(
			"close at %s strays %.1f%% %s the mean %s over %.1f%%",
			userExampleStrayAt, math.Abs(pct), where, meanString(mean), float64(MaxMovePct),
		))
	}
	misses = append(misses, spanBreaks(bars)...)
	if len(misses) == 0 {
		return nil
	}
	return slices.Concat([]string{userExample.frame(t)}, misses)
}

// The closes of the example window lie below the mean of its oldest close and its newest
// by the same measure whatever the constants are retuned to: the mean, how far the
// farthest close between them lies from it, and the bar it is.
func TestTheClosesOfTheUsersExampleStrayBelowTheirMeanWhateverTheConstants(t *testing.T) {
	bars := userExample.bars(t)
	at, err := time.Parse(stampLayout, userExampleStrayAt)
	if err != nil {
		t.Fatalf("%q is not a stamp: %v", userExampleStrayAt, err)
	}
	mean, pct, index := closesAgainstMean(bars)
	assertNear(t, "mean written out", mean, userExampleMean, 1e-9)
	assertNear(t, "stray written out", pct, userExample.strayPct, 0.01)
	if bars.Opens[index] != at.UnixMilli() {
		t.Errorf("written out, the stray is at %s, want %s", barTime(bars.Opens[index]), userExampleStrayAt)
	}
	gotMean, gotPct, gotAt := bars.stray()
	assertNear(t, "mean", gotMean, mean, 1e-12)
	assertNear(t, "stray", gotPct, pct, 1e-12)
	if gotAt != at.UnixMilli() {
		t.Errorf("the stray is at %s, want %s", barTime(gotAt), userExampleStrayAt)
	}
}

// The closes of the example window span 6.29%, from the stray at 15.59 to a close two
// bars after the oldest at 16.57, whatever the constants are retuned to: neither of the
// two ends, so that the span reads every close and not the first and the last alone.
func TestTheClosesOfTheUsersExampleSpanSixPercentWhateverTheConstants(t *testing.T) {
	bars := userExample.bars(t)
	low, high, pct := rangeOf(bars)
	assertNear(t, "lowest written out", low, userExampleLow, 1e-9)
	assertNear(t, "highest written out", high, userExampleHigh, 1e-9)
	assertNear(t, "span written out", pct, userExampleSpanPct, 0.01)
	gotLow, gotHigh, gotPct := bars.closeRange()
	assertNear(t, "lowest", gotLow, low, 1e-12)
	assertNear(t, "highest", gotHigh, high, 1e-12)
	assertNear(t, "span", gotPct, pct, 1e-12)
	if end := bars.Closes[0]; end == low || end == high {
		t.Errorf("the oldest close %v is an end of the span, the example's ends are between the bars", end)
	}
	if end := bars.Closes[len(bars.Closes)-1]; end == low || end == high {
		t.Errorf("the newest close %v is an end of the span, the example's ends are between the bars", end)
	}
}

// The example window reads as the rule says on both sides of the ladder at whatever the
// constants are: it holds exactly when its bodies, its move, its stray and the span of its
// closes all meet them, and Breaks names the frame and each of them it misses, in the order
// they are read, so that the stray is named on a window the move would have let through
// and the span on one the stray would have, as its tape does at a bound under 6.29%.
func TestTheUsersExampleWindowReadsAsTheRuleSaysAtAnyTuning(t *testing.T) {
	bars := userExample.bars(t)
	spanReason := closeRangeBreakText(bars)
	for _, sc := range rangeSideCases {
		want := userExampleBreaks(t, bars, sc)
		if got := RangeBreaks(bars, userExample.percentage, sc.side, sc.met); !slices.Equal(got, want) {
			t.Errorf("%s: breaks\n got %q\nwant %q", sc.name, got, want)
		}
		if over := MaxCloseRangePct > 0 && userExampleSpanPct > float64(MaxCloseRangePct); over != slices.Contains(want, spanReason) {
			t.Errorf("%s: a span of %v against the bound %v must be named exactly when it is over: %q", sc.name, userExampleSpanPct, MaxCloseRangePct, want)
		}
		if reading := TriggerRange(bars, userExample.percentage, sc.side, sc.met); reading.Holds != (want == nil) {
			t.Errorf("%s: holds %v, want %v of a window that misses %q", sc.name, reading.Holds, want == nil, want)
		}
	}
}

// At the constants the replays are measured at, the example window holds on both sides,
// with the stray named in the reasons: the mean it is measured against, and how far the
// closes lie from it, to the tenth of a percent, below it or above it alike in the word
// of the percentage and not of the sign.
func TestTheUsersExampleWindowHoldsWithAStrayBelowTheMeanAtTheMeasuredConstants(t *testing.T) {
	rangeNeedsTheMeasuredConstants(t)
	bars := userExample.bars(t)
	at, err := time.Parse(stampLayout, userExampleStrayAt)
	if err != nil {
		t.Fatalf("%q is not a stamp: %v", userExampleStrayAt, err)
	}
	for _, tc := range []struct {
		sc         rangeSideCase
		move       float64
		moveReason string
	}{
		{rangeSideCases[0], userExample.fallPct, "fall 4.6% under 5.0%"},
		{rangeSideCases[1], -2.25, "rise -2.3% under 5.0%"},
	} {
		reading := TriggerRange(bars, userExample.percentage, tc.sc.side, tc.sc.met)
		if !reading.Holds {
			t.Fatalf("%s: does not hold, breaks %q", tc.sc.name, RangeBreaks(bars, userExample.percentage, tc.sc.side, tc.sc.met))
		}
		if reading.Of != lookbackBars || reading.Quiet != userExample.quiet {
			t.Errorf("%s: %d of %d bars quiet, want %d of %d", tc.sc.name, reading.Quiet, reading.Of, userExample.quiet, lookbackBars)
		}
		assertNear(t, tc.sc.name+" move", reading.MovePct, tc.move, 0.01)
		assertNear(t, tc.sc.name+" stray", reading.StrayPct, userExample.strayPct, 0.01)
		assertNear(t, tc.sc.name+" mean", reading.Mean, userExampleMean, 1e-9)
		if reading.StrayAt != at.UnixMilli() {
			t.Errorf("%s: the stray is at %s, want %s", tc.sc.name, barTime(reading.StrayAt), userExampleStrayAt)
		}
		want := []string{
			"range on 1h bars 2022-02-17 10:00 to 2022-02-18 09:00 UTC",
			"quiet bodies 23/24 under 1.20%, over 85%",
			tc.moveReason,
			"closes within 3.8% of the mean 16.21, at most 5.0%",
			trendText(tc.sc.trendName, tc.sc.count(tc.sc.met), tc.sc.met, "of"),
		}
		if !slices.Equal(reading.Reasons, want) {
			t.Errorf("%s: reasons\n got %q\nwant %q", tc.sc.name, reading.Reasons, want)
		}
		if got := RangeBreaks(bars, userExample.percentage, tc.sc.side, tc.sc.met); got != nil {
			t.Errorf("%s: a window that holds breaks %q", tc.sc.name, got)
		}
	}
}

// At a bound under the stray, the example window misses where it did not hold before the
// stray was read: a long ladder on its fall, which is where Trigger stops and the stray
// is left unread, and Breaks names that and the stray after it; an inverse ladder, whose
// rise is under any bound above zero, on the stray alone, which Trigger reads and Breaks
// names, the move having let the window through.
func TestTheUsersExampleWindowMissesOnTheFallAndTheStrayAtABoundUnderBoth(t *testing.T) {
	needsAMove(t)
	if MaxMovePct >= math.Abs(userExample.strayPct) {
		t.Skipf("MaxMovePct is %v, not under the stray of %v: the window holds on the stray", MaxMovePct, userExample.strayPct)
	}
	bars := userExample.bars(t)
	if quiet := quietOf(bars, BodyShare*userExample.percentage); !(float64(quiet)/lookbackBars > QuietShare) {
		t.Skipf("%d of %d bodies are quiet at this tuning, not over the share: the reading stops on the bodies, before the move", quiet, lookbackBars)
	}
	mean, pct, _ := closesAgainstMean(bars)
	strayReason := fmt.Sprintf(
		"close at %s strays %.1f%% below the mean %s over %.1f%%", userExampleStrayAt, math.Abs(pct), meanString(mean), float64(MaxMovePct),
	)
	long, inverse := rangeSideCases[0], rangeSideCases[1]

	reading := TriggerRange(bars, userExample.percentage, long.side, long.met)
	fallReason := moveText(long.moveName, moveOf(bars, long.side), "over")
	breaks := RangeBreaks(bars, userExample.percentage, long.side, long.met)
	if reading.Holds || reading.Mean != 0 || reading.StrayPct != 0 || reading.StrayAt != 0 {
		t.Errorf("long: want a miss on the fall that read no stray, got %+v", reading)
	}
	if fall, stray := slices.Index(breaks, fallReason), slices.Index(breaks, strayReason); fall < 1 || stray <= fall {
		t.Errorf("long: breaks %q, want the fall %q and then the stray %q", breaks, fallReason, strayReason)
	}

	reading = TriggerRange(bars, userExample.percentage, inverse.side, inverse.met)
	assertNear(t, "inverse stray", reading.StrayPct, pct, 1e-12)
	assertNear(t, "inverse mean", reading.Mean, mean, 1e-12)
	if reading.Holds || reading.Reasons != nil || reading.StrayAt == 0 {
		t.Errorf("inverse: want a miss on the stray with no text, got %+v", reading)
	}
	if breaks = RangeBreaks(bars, userExample.percentage, inverse.side, inverse.met); !slices.Contains(breaks, strayReason) ||
		slices.ContainsFunc(breaks, func(reason string) bool { return strings.HasPrefix(reason, inverse.moveName) }) {
		t.Errorf("inverse: breaks %q, want the stray %q and no rise", breaks, strayReason)
	}
}

package flatmarket

import (
	"github.com/giovani-sirbu/mercury/strategies/gates/smartsell/series"
	"math"
	"reflect"
	"slices"
	"strings"
	"testing"

	"github.com/giovani-sirbu/mercury/trades/aggragates"
)

// trendPrefix opens every reason that names the SMC trend.
const trendPrefix = "smc trend direction"

// reads is a trend of count rows reading the side's way out of valid rows read.
func (sc rangeSideCase) reads(count, valid int) series.Trend {
	if sc.side == aggragates.SideShort {
		return series.Trend{Up: count, Valid: valid}
	}
	return series.Trend{Down: count, Valid: valid}
}

// withoutTheTrend is breaks less the trend's reason, nothing when it was the only miss (the frame opens a list that names one).
func withoutTheTrend(breaks []string) []string {
	kept := slices.DeleteFunc(slices.Clone(breaks), func(reason string) bool { return strings.HasPrefix(reason, trendPrefix) })
	if len(kept) == 1 && strings.HasPrefix(kept[0], "range on 1h bars ") {
		return nil
	}
	return kept
}

// TriggerOnBars holds where Trigger does, with the same numbers, and where only the trend misses.
func TestTriggerOnBarsHoldsWhereTriggerDoesAndWhereOnlyTheTrendMisses(t *testing.T) {
	needsTheTrend(t)
	needsAMove(t)
	needsAQuietBoundary(t)
	for _, sc := range rangeSideCases {
		bars := holding(sc.side)
		full, on := TriggerRange(bars, fixturePercentage, sc.side, sc.met), TriggerRangeOnBars(bars, fixturePercentage, sc.side)
		if !full.Holds || !on.Holds {
			t.Fatalf("%s: both must hold, got %+v and %+v", sc.name, full, on)
		}
		want := full
		want.Trend, want.Reasons = series.Trend{}, withoutTheTrend(full.Reasons)
		if !reflect.DeepEqual(on, want) {
			t.Errorf("%s: want Trigger's reading less the trend\n got %+v\nwant %+v", sc.name, on, want)
		}
		trends := map[string]series.Trend{"not served": {}, "0 of 3 rows": sc.reads(0, rangeDashboardRows), "1 of 3 rows": sc.reads(1, rangeDashboardRows)}
		for name, trend := range trends {
			meets := trend.Valid > 0 && sc.count(trend) >= MinSmcTrend
			if got := TriggerRange(bars, fixturePercentage, sc.side, trend); got.Holds != meets {
				t.Errorf("%s, trend %s: Trigger holds %v, the rule says %v", sc.name, name, got.Holds, meets)
			}
		}
	}
}

// Every ask is answered on the bars alone: a miss on a bar condition misses, BreaksOnBars names it as Breaks does less the trend.
func TestTriggerOnBarsAnswersEveryAskOnTheBarsAlone(t *testing.T) {
	needsAMove(t)
	needsAQuietBoundary(t)
	for _, sc := range rangeSideCases {
		for _, one := range asksFor(sc) {
			want := withoutTheTrend(one.breaks)
			reading, breaks := TriggerRangeOnBars(one.bars, one.percentage, sc.side), RangeBreaksOnBars(one.bars, one.percentage, sc.side)
			if holds := len(want) == 0; reading.Holds != holds || (reading.Reasons != nil) != holds || reading.Trend != (series.Trend{}) {
				t.Errorf("%s, %s: holds %v with reasons %q and trend %+v, want holds %v", sc.name, one.name, reading.Holds, reading.Reasons, reading.Trend, holds)
			}
			if !slices.Equal(breaks, want) {
				t.Errorf("%s, %s: breaks\n got %q\nwant %q", sc.name, one.name, breaks, want)
			}
			if same := RangeBreaks(one.bars, one.percentage, sc.side, sc.met); one.trend == sc.met && !slices.Equal(breaks, same) {
				t.Errorf("%s, %s: under a met trend BreaksOnBars %q must equal Breaks %q", sc.name, one.name, breaks, same)
			}
		}
	}
}

// A hold names exactly the five bar reasons, the frame, the bodies, the move, the stray and
// the span of the closes, none the trend, and carries no trend.
func TestTriggerOnBarsNamesExactlyTheFiveBarReasonsAndReadsNoTrend(t *testing.T) {
	needsACloseRange(t)
	needsAMove(t)
	needsAQuietBoundary(t)
	for _, sc := range rangeSideCases {
		for _, quiet := range []int{rangeFixtureBars, quietNeeded(rangeFixtureBars)} {
			bars := window{quiet: quiet, movePct: holdMove, side: sc.side}.build(fixturePercentage)
			reading := TriggerRangeOnBars(bars, fixturePercentage, sc.side)
			want := []string{
				frameText(rangeFixtureBars), quietText(quiet, rangeFixtureBars, "over"),
				moveText(sc.moveName, moveOf(bars, sc.side), "under"), strayHoldText(bars), closeRangeHoldText(bars),
			}
			if !reading.Holds || !slices.Equal(reading.Reasons, want) {
				t.Fatalf("%s, %d quiet bars: reasons\n got %q\nwant %q", sc.name, quiet, reading.Reasons, want)
			}
			if reading.Trend != (series.Trend{}) || reading.Side != sc.side || strings.Contains(strings.Join(reading.Reasons, ","), trendPrefix) {
				t.Errorf("%s, %d quiet bars: no trend is read or named, got %+v", sc.name, quiet, reading)
			}
		}
	}
}

// BreaksOnBars names nothing exactly when TriggerOnBars holds, never the trend, and what Breaks names under a met trend.
func TestBreaksOnBarsNamesNothingExactlyWhenTriggerOnBarsHolds(t *testing.T) {
	needsAMove(t)
	needsAQuietBoundary(t)
	long, inverse := rangeSideCases[0], rangeSideCases[1]
	cut := holding(long.side)
	cut.Highs = cut.Highs[1:]
	holds, total := 0, 0
	for _, bars := range []Bars{
		holding(long.side), holding(inverse.side), cut, {},
		window{movePct: holdMove, side: long.side}.build(fixturePercentage),
		window{quiet: rangeFixtureBars, movePct: overMove, side: inverse.side}.build(fixturePercentage),
		window{quiet: rangeFixtureBars, movePct: holdMove, strayPct: overStray, side: long.side}.build(fixturePercentage),
		window{quiet: rangeFixtureBars, movePct: holdMove, strayPct: -holdStray, side: inverse.side}.build(fixturePercentage),
		window{quiet: rangeFixtureBars, movePct: holdMove, closeRangePct: overRange, side: long.side}.build(fixturePercentage),
		window{quiet: rangeFixtureBars, movePct: holdMove, closeRangePct: overRange, side: inverse.side}.build(fixturePercentage),
		window{quiet: rangeFixtureBars, movePct: holdMove, closeRangePct: holdRange, side: inverse.side}.build(fixturePercentage),
	} {
		for _, percentage := range []float64{fixturePercentage, 0, -1, math.NaN()} {
			for _, side := range []string{long.side, inverse.side, "", "sell"} {
				reading, breaks := TriggerRangeOnBars(bars, percentage, side), RangeBreaksOnBars(bars, percentage, side)
				if (len(breaks) == 0) != reading.Holds || (reading.Reasons != nil) != reading.Holds {
					t.Errorf("percentage %v, side %q: holds %v with reasons %q but breaks %q", percentage, side, reading.Holds, reading.Reasons, breaks)
				}
				if same := RangeBreaks(bars, percentage, side, rangeMetTrend(side)); !slices.Equal(breaks, same) || !slices.Equal(breaks, withoutTheTrend(breaks)) {
					t.Errorf("percentage %v, side %q: breaks %q, want Breaks under a met trend %q and no trend", percentage, side, breaks, same)
				}
				total++
				if reading.Holds {
					holds++
				}
			}
		}
	}
	if holds == 0 || holds == total {
		t.Errorf("the inputs must hold and miss, got %d of %d holding", holds, total)
	}
}

// A miss of TriggerOnBars formats and allocates nothing.
func TestAMissOfTriggerOnBarsAllocatesNothing(t *testing.T) {
	needsAMove(t)
	needsAQuietBoundary(t)
	for _, sc := range rangeSideCases {
		for _, one := range asksFor(sc) {
			if len(withoutTheTrend(one.breaks)) == 0 {
				continue
			}
			if allocs := testing.AllocsPerRun(100, func() { TriggerRangeOnBars(one.bars, one.percentage, sc.side) }); allocs != 0 {
				t.Errorf("%s, %s: a miss allocated %v times, want none", sc.name, one.name, allocs)
			}
		}
	}
}

// Real windows read on the bars alone as Trigger reads them under a met trend: the holds, the short bodies and the fall over the bound.
func TestTriggerOnBarsReadsTheRealWindowsAsTriggerDoesUnderAMetTrend(t *testing.T) {
	side := aggragates.SideLong
	for _, c := range slices.Concat(holdingReplays, shortReplays, overReplays) {
		t.Run(c.name(), func(t *testing.T) {
			bars := c.bars(t)
			full, on := TriggerRange(bars, c.percentage, side, rangeMetTrend(side)), TriggerRangeOnBars(bars, c.percentage, side)
			if on.Holds != full.Holds || !slices.Equal(on.Reasons, withoutTheTrend(full.Reasons)) {
				t.Errorf("holds %v with reasons %q, want %v with %q", on.Holds, on.Reasons, full.Holds, withoutTheTrend(full.Reasons))
			}
			if got, want := RangeBreaksOnBars(bars, c.percentage, side), RangeBreaks(bars, c.percentage, side, rangeMetTrend(side)); !slices.Equal(got, want) {
				t.Errorf("breaks %q, want %q", got, want)
			}
		})
	}
}

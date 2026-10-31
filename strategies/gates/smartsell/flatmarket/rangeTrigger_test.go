package flatmarket

import (
	"github.com/giovani-sirbu/mercury/strategies/gates/smartsell/series"
	"math"
	"slices"
	"testing"

	"github.com/giovani-sirbu/mercury/trades/aggragates"
)

// ask is one thing the gate is asked: the bars, the percentage and the trend of a
// tick for one side, and what Breaks must name for it, nothing when it holds.
type ask struct {
	name       string
	bars       Bars
	percentage float64
	trend      series.Trend
	breaks     []string
}

// asksFor are asks the gate answers every way on a side: windows that hold, and each
// way a reading misses, alone and together, the stray's among them (strayAsksFor) and
// the span of the closes' (closeRangeAsksFor). The windows that miss on the move, the
// bodies or the trend alone span within the bound, so that they miss on that alone;
// the windows that stray span what their closes span, and name it where it is over.
func asksFor(sc rangeSideCase) []ask {
	win := func(quiet int, move float64) Bars {
		return window{quiet: quiet, movePct: move, side: sc.side}.build(fixturePercentage)
	}
	overBars := win(rangeFixtureBars, overMove)
	opening, need, unread := frameText(rangeFixtureBars), quietNeeded(rangeFixtureBars), series.Trend{}
	holdBars, quietShort := win(rangeFixtureBars, holdMove), quietText(quietNeeded(rangeFixtureBars)-1, rangeFixtureBars, "short of")
	moveOver := moveText(sc.moveName, moveOf(overBars, sc.side), "over")
	unset := []string{opening, "percentage not set"}
	asks := []ask{
		{"holds", holdBars, fixturePercentage, sc.met, nil},
		{"holds on a move the other way", win(rangeFixtureBars, -holdMove), fixturePercentage, sc.met, nil},
		{"holds at the fewest quiet bars", win(need, holdMove), fixturePercentage, sc.met, nil},
		{"a bar short of the quiet share", win(need-1, holdMove), fixturePercentage, sc.met, []string{opening, quietShort}},
		{"a move over", overBars, fixturePercentage, sc.met, []string{opening, moveOver}},
		{
			"quiet bodies short and a move over", win(need-1, overMove), fixturePercentage, sc.met,
			[]string{opening, quietShort, moveOver},
		},
		{
			"a trend a row short", holdBars, fixturePercentage, sc.short,
			[]string{opening, trendText(sc.trendName, sc.count(sc.short), sc.short, "short of")},
		},
		{
			"a trend the other way", holdBars, fixturePercentage, sc.wrong,
			[]string{opening, trendText(sc.trendName, sc.count(sc.wrong), sc.wrong, "short of")},
		},
		{"a trend not served", holdBars, fixturePercentage, unread, []string{opening, "smc trend direction not read"}},
		{"bars not served", Bars{}, fixturePercentage, sc.met, []string{"bars not served"}},
		{"a percentage of zero", holdBars, 0, sc.met, unset},
		{"a percentage under zero", holdBars, -fixturePercentage, sc.met, unset},
		{"a percentage not a number", holdBars, math.NaN(), sc.met, unset},
	}
	return slices.Concat(asks, strayAsksFor(sc), closeRangeAsksFor(sc))
}

// Every ask is answered as the rule reads: a hold carries its reasons, a miss carries
// none and allocates nothing, and Breaks names each miss in the order the hold names
// its readings: the bodies, the move, the stray, the span of the closes, the trend.
func TestTriggerAnswersEveryAskAndAMissFormatsAndAllocatesNothing(t *testing.T) {
	needsTheTrend(t)
	needsAMove(t)
	needsAQuietBoundary(t)
	for _, sc := range rangeSideCases {
		for _, one := range asksFor(sc) {
			reading := TriggerRange(one.bars, one.percentage, sc.side, one.trend)
			if holds := one.breaks == nil; reading.Holds != holds || (reading.Reasons != nil) != holds {
				t.Errorf("%s, %s: holds %v with reasons %q, want holds %v", sc.name, one.name, reading.Holds, reading.Reasons, holds)
			}
			if got := RangeBreaks(one.bars, one.percentage, sc.side, one.trend); !slices.Equal(got, one.breaks) {
				t.Errorf("%s, %s: breaks\n got %q\nwant %q", sc.name, one.name, got, one.breaks)
			}
			if one.breaks == nil {
				continue
			}
			if allocs := testing.AllocsPerRun(100, func() { TriggerRange(one.bars, one.percentage, sc.side, one.trend) }); allocs != 0 {
				t.Errorf("%s, %s: a miss allocated %v times, want none", sc.name, one.name, allocs)
			}
		}
	}
}

// A miss returns at once, the cheapest step first, and carries what it read by then
// and nothing more: the side and the trend always, the limit of a body once the
// percentage was set, the window and its quiet bodies once the trend met its need and
// the bars were served, the move only once the quiet bodies held, the mean and the
// stray only once the move held, and the span of the closes only once the stray held.
func TestAMissReturnsAtOnceCarryingWhatItReadSoFar(t *testing.T) {
	needsTheTrend(t)
	needsAMove(t)
	needsAQuietBoundary(t)
	needsACloseRange(t)
	sc := rangeSideCases[0]
	loud := window{quiet: quietNeeded(rangeFixtureBars) - 1, movePct: overMove, side: sc.side}.build(fixturePercentage)
	over := window{quiet: rangeFixtureBars, movePct: overMove, side: sc.side}.build(fixturePercentage)
	straying := window{quiet: rangeFixtureBars, movePct: holdMove, strayPct: overStray, side: sc.side}.build(fixturePercentage)
	spanning := window{quiet: rangeFixtureBars, movePct: holdMove, closeRangePct: overRange, side: sc.side}.build(fixturePercentage)
	for _, tc := range []struct {
		name                                  string
		bars                                  Bars
		percentage                            float64
		trend                                 series.Trend
		limit, covers, moved, strayed, ranged bool
	}{
		{"a percentage not set", loud, 0, sc.met, false, false, false, false, false},
		{"a trend short", loud, fixturePercentage, sc.short, true, false, false, false, false},
		{"bars not served", Bars{}, fixturePercentage, sc.met, true, false, false, false, false},
		{"quiet bodies short", loud, fixturePercentage, sc.met, true, true, false, false, false},
		{"a move over", over, fixturePercentage, sc.met, true, true, true, false, false},
		{"a close strays", straying, fixturePercentage, sc.met, true, true, true, true, false},
		{"the closes span over", spanning, fixturePercentage, sc.met, true, true, true, true, true},
	} {
		reading := TriggerRange(tc.bars, tc.percentage, sc.side, tc.trend)
		if reading.Holds || reading.Side != sc.side || reading.Trend != tc.trend {
			t.Errorf("%s: want a miss that carries its side and trend, got %+v", tc.name, reading)
		}
		limit, covers, moved := reading.BodyMaxPct != 0, reading.Of != 0 && reading.From != 0, reading.MovePct != 0
		strayed, ranged := reading.Mean != 0, reading.CloseLow != 0 || reading.CloseHigh != 0 || reading.CloseRangePct != 0
		if limit != tc.limit || covers != tc.covers || moved != tc.moved || strayed != tc.strayed || ranged != tc.ranged {
			t.Errorf("%s: carries limit %v, window %v, move %v, mean %v, span %v, got %+v", tc.name, limit, covers, moved, strayed, ranged, reading)
		}
		if !strayed && (reading.StrayPct != 0 || reading.StrayAt != 0) {
			t.Errorf("%s: a stray not read must carry none, got %+v", tc.name, reading)
		}
		if strayed && tc.ranged {
			assertNear(t, tc.name+" stray", reading.StrayPct, 0, 1e-9)
		} else if strayed {
			assertNear(t, tc.name+" stray", reading.StrayPct, overStray, 1e-9)
		}
		if ranged {
			low, high, pct := rangeOf(tc.bars)
			assertNear(t, tc.name+" lowest close", reading.CloseLow, low, 1e-12)
			assertNear(t, tc.name+" highest close", reading.CloseHigh, high, 1e-12)
			assertNear(t, tc.name+" span", reading.CloseRangePct, pct, 1e-9)
		}
	}
}

// Trigger and Breaks never disagree: over bars that hold and bars that miss every way,
// percentages that are and are not set, sides that are and are not read and trends
// of both ways, Breaks names nothing exactly when Trigger holds, and only a hold
// carries reasons.
func TestBreaksNamesNothingExactlyWhenTriggerHolds(t *testing.T) {
	needsAMove(t)
	needsAQuietBoundary(t)
	long, inverse := rangeSideCases[0], rangeSideCases[1]
	cut := holding(long.side)
	cut.Highs = cut.Highs[1:]
	every := series.Trend{Up: trendRows, Down: trendRows, Valid: trendRows}
	holds, total := 0, 0
	for _, bars := range []Bars{
		holding(long.side), holding(inverse.side), cut, {},
		window{movePct: holdMove, side: long.side}.build(fixturePercentage),
		window{quiet: rangeFixtureBars, movePct: overMove, side: inverse.side}.build(fixturePercentage),
		window{quiet: rangeFixtureBars, movePct: holdMove, strayPct: overStray, side: long.side}.build(fixturePercentage),
		window{quiet: rangeFixtureBars, movePct: holdMove, strayPct: -holdStray, side: inverse.side}.build(fixturePercentage),
		window{quiet: rangeFixtureBars, movePct: holdMove, closeRangePct: overRange, side: long.side}.build(fixturePercentage),
		window{quiet: rangeFixtureBars, movePct: holdMove, closeRangePct: overRange, side: inverse.side}.build(fixturePercentage),
		window{quiet: rangeFixtureBars, movePct: holdMove, closeRangePct: holdRange, side: long.side}.build(fixturePercentage),
	} {
		for _, percentage := range []float64{fixturePercentage, 0, -1, math.NaN()} {
			for _, side := range []string{long.side, inverse.side, "", "sell"} {
				for _, trend := range []series.Trend{{}, long.met, inverse.met, long.short, every} {
					reading, breaks := TriggerRange(bars, percentage, side, trend), RangeBreaks(bars, percentage, side, trend)
					if (len(breaks) == 0) != reading.Holds || (reading.Reasons != nil) != reading.Holds {
						t.Errorf(
							"percentage %v, side %q, trend %+v: holds %v with reasons %q but breaks %q",
							percentage, side, trend, reading.Holds, reading.Reasons, breaks,
						)
					}
					total++
					if reading.Holds {
						holds++
					}
				}
			}
		}
	}
	if holds == 0 || holds == total {
		t.Errorf("the inputs must hold and miss, got %d of %d holding", holds, total)
	}
}

// A window that holds names the bars it covers, the quiet bodies against the share,
// the move in the word of the side, the closes against their mean, the span of all the
// closes and the trend that met its need, in that order, and carries every number it
// read: with every bar quiet, and with the fewest that hold.
func TestTriggerHoldsAndNamesTheWindowTheBodiesTheMoveAndTheTrend(t *testing.T) {
	needsAMove(t)
	needsAQuietBoundary(t)
	for _, sc := range rangeSideCases {
		for _, quiet := range []int{rangeFixtureBars, quietNeeded(rangeFixtureBars)} {
			bars := window{quiet: quiet, movePct: holdMove, side: sc.side}.build(fixturePercentage)
			reading := TriggerRange(bars, fixturePercentage, sc.side, sc.met)
			if !reading.Holds {
				t.Fatalf("%s, %d quiet bars: must hold, got breaks %q", sc.name, quiet, RangeBreaks(bars, fixturePercentage, sc.side, sc.met))
			}
			assertNear(t, sc.name+" body limit", reading.BodyMaxPct, BodyShare*fixturePercentage, 1e-12)
			assertNear(t, sc.name+" move", reading.MovePct, holdMove, 1e-9)
			covered := reading.From == bars.Opens[0] && reading.To == bars.LastBarOpen()
			covered = covered && reading.Of == rangeFixtureBars && reading.Quiet == quiet
			if !covered || reading.Side != sc.side || reading.Trend != sc.met {
				t.Errorf("%s, %d quiet bars: the reading carries its window, side and trend, got %+v", sc.name, quiet, reading)
			}
			low, high, span := rangeOf(bars)
			assertNear(t, sc.name+" lowest close", reading.CloseLow, low, 1e-12)
			assertNear(t, sc.name+" highest close", reading.CloseHigh, high, 1e-12)
			assertNear(t, sc.name+" span", reading.CloseRangePct, span, 1e-9)
			want := []string{
				frameText(rangeFixtureBars),
				quietText(quiet, rangeFixtureBars, "over"),
				moveText(sc.moveName, moveOf(bars, sc.side), "under"),
				strayHoldText(bars),
			}
			if MaxCloseRangePct > 0 {
				want = append(want, closeRangeHoldText(bars))
			}
			if MinSmcTrend > 0 {
				want = append(want, trendText(sc.trendName, sc.count(sc.met), sc.met, "of"))
			}
			if !slices.Equal(reading.Reasons, want) {
				t.Errorf("%s, %d quiet bars: reasons\n got %q\nwant %q", sc.name, quiet, reading.Reasons, want)
			}
		}
	}
}

// The trend is the count of the rows reading the ladder's way, down for a long ladder
// and up for an inverse one, served and at least the need: the other way's count
// counts for nothing, and a trend sophos did not serve meets no need.
func TestTrendMeetsReadsTheCountOfItsSide(t *testing.T) {
	needsTheTrend(t)
	both := series.Trend{Up: trendRows, Down: trendRows, Valid: trendRows}
	for _, sc := range rangeSideCases {
		for _, tc := range []struct {
			name  string
			trend series.Trend
			want  bool
		}{
			{"just enough", sc.met, true},
			{"a row short", sc.short, false},
			{"the other way", sc.wrong, false},
			{"both ways", both, true},
			{"counts without a row read", series.Trend{Up: trendRows, Down: trendRows}, false},
			{"nothing served", series.Trend{}, false},
		} {
			if got := TrendMeets(tc.trend, sc.side); got != tc.want {
				t.Errorf("%s, %s: TrendMeets %v, want %v", sc.name, tc.name, got, tc.want)
			}
		}
	}
}

// A side other than the two has no way to read: no trend meets it, with the condition
// off too, and a reading made for it misses and names the side.
func TestASideOtherThanTheTwoIsNeverMet(t *testing.T) {
	needsAMove(t)
	needsAQuietBoundary(t)
	both, bars := series.Trend{Up: trendRows, Down: trendRows, Valid: trendRows}, holding(aggragates.SideLong)
	for _, side := range []string{"", "LONG", "futures"} {
		if TrendMeets(both, side) || both.Meets(side, 0) {
			t.Errorf("side %q: a trend must not meet a side that is neither of the two", side)
		}
		if reading := TriggerRange(bars, fixturePercentage, side, both); reading.Holds || reading.Reasons != nil {
			t.Errorf("side %q: want a miss with no text, got %+v", side, reading)
		}
		want := []string{frameText(rangeFixtureBars), "side not read"}
		if got := RangeBreaks(bars, fixturePercentage, side, both); !slices.Equal(got, want) {
			t.Errorf("side %q: breaks\n got %q\nwant %q", side, got, want)
		}
	}
}

// With the condition off, the need at zero or below, no trend is asked or named: every
// trend meets it, served or not, and neither the hold nor the breaks speak of it.
func TestWithTheConditionOffNoTrendIsAskedOrNamed(t *testing.T) {
	needsAMove(t)
	needsAQuietBoundary(t)
	for _, sc := range rangeSideCases {
		bars := holding(sc.side)
		reading := TriggerRange(bars, fixturePercentage, sc.side, sc.met)
		if !reading.Holds || !(series.Trend{}).Meets(sc.side, 0) {
			t.Fatalf("%s: with no condition every trend meets it, got %+v", sc.name, reading)
		}
		named, unnamed := rangeHoldReasons(reading, 1), rangeHoldReasons(reading, 0)
		if !slices.Equal(unnamed, named[:len(named)-1]) {
			t.Errorf("%s: no trend is named:\n got %q\nwant %q", sc.name, unnamed, named[:len(named)-1])
		}
		if breaks := rangeBreakReasons(bars, fixturePercentage, sc.side, series.Trend{}, 0); breaks != nil {
			t.Errorf("%s: no trend is missed, got %q", sc.name, breaks)
		}
	}
}

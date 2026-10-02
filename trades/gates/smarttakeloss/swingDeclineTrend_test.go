package smarttakeloss

import (
	"fmt"
	"slices"
	"testing"
	"time"

	"github.com/giovani-sirbu/mercury/trades/aggragates"
	"github.com/giovani-sirbu/mercury/trades/gates/swingdecline"
)

// The Trend Direction row the fixtures serve (stairBlock, patternTape) just meets
// swingdecline.SwingDeclineMinSmcTrendDown: as many of the dashboard's
// patternTrendValid rows read down as the constant asks. shortTrendDown is a row
// short of it, so no test here leans on the constant's value. shortTrendReason is
// what the swing decline names beside the window under the second.
const (
	patternTrendValid = 3
	patternTrendDown  = swingdecline.SwingDeclineMinSmcTrendDown
	shortTrendDown    = swingdecline.SwingDeclineMinSmcTrendDown - 1
)

var shortTrendReason = fmt.Sprintf("smc trend direction down %d/%d short of %d needed", shortTrendDown, patternTrendValid, swingdecline.SwingDeclineMinSmcTrendDown)

// notReadTrendReason is what the swing decline names beside the window when
// sophos served no Trend Direction row.
const notReadTrendReason = "smc trend direction not read"

// needsTheTrend skips a test that has a trend fail a ladder when the condition is
// off: at swingdecline.SwingDeclineMinSmcTrendDown zero or below no trend is read,
// so none can fail.
func needsTheTrend(t *testing.T) {
	t.Helper()
	if swingdecline.SwingDeclineMinSmcTrendDown <= 0 {
		t.Skip("SwingDeclineMinSmcTrendDown is zero or below: the condition is off and no trend fails a ladder")
	}
}

// withSmcTrend is the block serving this Trend Direction row beside its series:
// down of the valid rows read down.
func withSmcTrend(block aggragates.SmartTakeLossIndicators, down, valid int) aggragates.SmartTakeLossIndicators {
	block.SwingDeclineSmcTrendDown, block.SwingDeclineSmcTrendValid = down, valid
	return block
}

// The staircase's shape holds at its fourth fill's bar, but the SMC trend decides
// beside it: with the Trend Direction down on a row short of the need, or with no
// row read — a stray count with no row behind it included — the ladder does not go
// pending — no row, no latch, and no sale at the band — where the same tick under
// a trend that holds writes the pending row.
func TestSwingDeclineATrendShortOfTheNeedKeepsAHoldingShapeFromGoingPending(t *testing.T) {
	needsTheTrend(t)
	holding := stairFourthBlock()
	if got := Apply(stairFourth(), "", slowDeclineBand-1, withBlock(holding)); len(got.Rows) != 1 || got.Rows[0].Event != EventPending {
		t.Fatalf("fixture drifted: under a trend that holds the staircase goes pending at bar 24, got %+v", got)
	}
	for name, trend := range map[string][2]int{"a row short of the need": {shortTrendDown, patternTrendValid}, "no row read": {0, 0}, "a count, no row read": {patternTrendValid, 0}} {
		block := withBlock(withSmcTrend(holding, trend[0], trend[1]))
		assertUntouched(t, Apply(stairFourth(), "", slowDeclineBand-1, block), "")
		if got := Apply(stairFourth(), "", slowDeclineBand+1, block); got.Reason != "" || got.Position != "" {
			t.Errorf("%s: a ladder that did not go pending sells nothing at the band, got %+v", name, got)
		}
	}
}

// A pending, latched ladder's fifth fill is judged on the trend too: the shape
// holds at its bar, the trend reads a row short of the need, and the exit is
// cancelled — the row names the window and the trend alone, under the swing
// decline, the latch stays and the band sells nothing. The ladder goes pending
// again by the trigger alone, on the next tick inside the fill's read bars, once
// the trend meets the need.
func TestSwingDeclineATrendShortOfTheNeedCancelsAJudgedFillWhoseShapeHolds(t *testing.T) {
	needsTheTrend(t)
	short := withSmcTrend(stairFifthBlock(), shortTrendDown, patternTrendValid)
	reading := stairReading(t, stairFifthFill(t), short, false)
	if len(reading.Breaks) != 2 || reading.Breaks[1] != shortTrendReason {
		t.Fatalf("fixture drifted: the shape holds and the breaks name the window and the trend alone, got %q", reading.Breaks)
	}
	cancelled, got := engineTick(stairFifthFill(t), "", slowDeclineBand+1, stairOpen(31), withBlock(short))
	assertRow(t, got, CancelledRow(stairPrices[4], quietFinding(nil), swingFinding(reading.Breaks)))
	assertNoSale(t, got, "")
	if st := rebuildState(cancelled); st.slowDeclinePending || !st.swingDeclineWatched || !st.indecision {
		t.Fatalf("watched, no longer pending and still latched, got %+v", st)
	}
	assertUntouched(t, Apply(cancelled, "", slowDeclineBand+1, withBlock(short)), "")

	met := stairFifthBlock()
	pending, again := engineTick(cancelled, "", underTheBand, stairOpen(32), withBlock(met))
	assertRow(t, again, PendingRow(stairPrices[4], swingFinding(stairReading(t, cancelled, met, true).Reasons)))
	if st := rebuildState(pending); !st.slowDeclinePending || st.slowDeclinePendingFrom != stairPrices[4] {
		t.Errorf("pending again from the fifth fill, got %+v", st)
	}
}

// The trend is the tick's own, not the fill's bar's: the fourth fill's bar closes
// with the Trend Direction a row short of the need and nothing is written, a bar
// later it meets the need — inside the read bars — and the ladder goes pending on
// that tick, its row naming the trend it met. On the last read bar it still does,
// and past the read bars the same trend writes nothing.
func TestSwingDeclineATrendThatTurnsInsideTheReadBarsStillTriggers(t *testing.T) {
	needsTheTrend(t)
	trade := stairFourth()
	assertUntouched(t, Apply(trade, "", slowDeclineBand-1, withBlock(withSmcTrend(stairFourthBlock(), shortTrendDown, patternTrendValid))), "")

	turned := withSmcTrend(stairBlock(stairTurns, 25), patternTrendDown, patternTrendValid)
	pending, got := engineTick(trade, "", slowDeclineBand-1, stairOpen(26), withBlock(turned))
	assertRow(t, got, PendingRow(stairPrices[3], indecisionFinding(nil), swingFinding(stairReading(t, trade, turned, true).Reasons)))
	if st := rebuildState(pending); !st.slowDeclinePending || st.slowDeclinePendingFrom != stairPrices[3] || !st.indecision {
		t.Errorf("pending from the fourth fill and latched, got %+v", st)
	}

	last := 24 + swingdecline.SwingDeclineReadBars
	for bar, want := range map[int]bool{last: true, last + 1: false} {
		got := Apply(trade, "", slowDeclineBand-1, withBlock(withSmcTrend(stairBlock(stairTurns, bar), patternTrendDown, patternTrendValid)))
		if (len(got.Rows) != 0) != want {
			t.Errorf("a trend that holds on the series' bar %d: pending row %v, want %v", bar, len(got.Rows) != 0, want)
		}
	}
}

// The fill that takes a pending ladder to its last depth is judged by nobody, the
// trend included: the series has closed its bar, the trend is short of the need or
// not read, and the exit stays pending, no row is written and the band sells.
func TestSwingDeclineTheLastDepthFillIsNotJudgedOnTheTrendEither(t *testing.T) {
	needsTheTrend(t)
	seventh := withRows(stairLadder(0, 9, 15, 24, 30, 36, 42), PendingRow(stairPrices[6], swingFinding(nil)))
	eighth := withFill(seventh, stairPrices[7], stairOpen(48).Add(time.Minute))
	for name, trend := range map[string][2]int{"no row read": {0, 0}, "a row short of the need": {shortTrendDown, patternTrendValid}} {
		block := withBlock(withSmcTrend(stairBlock(stairDecline, 48), trend[0], trend[1]))
		assertUntouched(t, Apply(eighth, "", underTheBand, block), "")
		got := Apply(eighth, "", slowDeclineBand+1, block)
		assertForced(t, got, reasonSellBand)
		if len(got.Rows) != 0 {
			t.Errorf("%s: the last depth's fill is judged by nobody, got %+v", name, got)
		}
	}
}

// ExitReached is Apply's own answer under the trend too: the staircase sells at the
// band under a trend that holds — on the tick that goes pending and on the one that
// judges a new fill, at the need and above it — and under one a row short of the
// need or not read neither tick does.
func TestSwingDeclineExitReachedFollowsTheTrend(t *testing.T) {
	needsTheTrend(t)
	band := slowDeclineBand + 1
	for name, tc := range map[string]struct {
		trade aggragates.Trades
		bar   int
	}{"going pending": {stairFourth(), 24}, "judging a new fill": {stairFifthFill(t), 30}} {
		for _, trend := range []struct {
			down, valid int
			sells       bool
		}{{patternTrendValid, patternTrendValid, patternTrendDown <= patternTrendValid}, {patternTrendDown, patternTrendValid, true}, {shortTrendDown, patternTrendValid, false}, {0, 0, false}} {
			block := withSmcTrend(stairBlock(stairTurns, tc.bar), trend.down, trend.valid)
			if got := Apply(tc.trade, "", band, withBlock(block)); ExitReached(tc.trade, band, block) != trend.sells || (got.Reason == reasonSellBand) != trend.sells {
				t.Errorf("%s, trend %d/%d: ExitReached %v, Apply %+v, want the sale %v", name, trend.down, trend.valid, ExitReached(tc.trade, band, block), got, trend.sells)
			}
		}
	}
}

// The trend is read at the trigger and at a new fill's judgement, never again: a
// ladder that went pending under a trend that held keeps selling at its band on
// every later tick, whatever the Trend Direction reads by then, and writes no row
// for it.
func TestSwingDeclineAPendingLadderKeepsSellingWhateverTheTrendReadsLater(t *testing.T) {
	needsTheTrend(t)
	pending := stairPending(t)
	for name, trend := range map[string][2]int{"a row short of the need": {shortTrendDown, patternTrendValid}, "no row down": {0, patternTrendValid}, "no row read": {0, 0}} {
		block := withBlock(withSmcTrend(stairBlock(stairTurns, 26), trend[0], trend[1]))
		got := Apply(pending, "", slowDeclineBand+1, block)
		assertForced(t, got, reasonSellBand)
		if len(got.Rows) != 0 {
			t.Errorf("%s: a judged ladder writes no row on a later trend, got %+v", name, got)
		}
		assertUntouched(t, Apply(pending, "", underTheBand, block), "")
	}
}

// A judgement under a trend that misses names it beside the shape, whatever the
// shape reads: a Trend Direction row not read, a count with no row read behind it,
// and a trend a row short of the need beside a shape that itself breaks — then the
// row names the shape's breaks and the trend after them. Under each the ladder is
// cancelled at its fifth fill, the latch stays, and nothing is sold at the band on
// that tick or the next.
func TestSwingDeclineAFillJudgedUnderAnyTrendThatMissesIsCancelledNamingIt(t *testing.T) {
	needsTheTrend(t)
	for name, tc := range map[string]struct {
		turns       []float64
		down, valid int
		trendReason string
	}{
		"no row read":                      {stairTurns, 0, 0, notReadTrendReason},
		"a count, no row read":             {stairTurns, patternTrendValid, 0, notReadTrendReason},
		"a shape that breaks, no row read": {stairBounce, 0, 0, notReadTrendReason},
		"a shape that breaks, a row short": {stairBounce, shortTrendDown, patternTrendValid, shortTrendReason},
	} {
		block := withSmcTrend(stairBlock(tc.turns, 30), tc.down, tc.valid)
		fifth := stairFifthFill(t)
		reading := stairReading(t, fifth, block, false)
		if last := reading.Breaks[len(reading.Breaks)-1]; last != tc.trendReason {
			t.Fatalf("%s: fixture drifted: the trend is named last, got %q", name, reading.Breaks)
		}
		cancelled, got := engineTick(fifth, "", slowDeclineBand+1, stairOpen(31), withBlock(block))
		assertRow(t, got, CancelledRow(stairPrices[4], quietFinding(nil), swingFinding(slices.Clone(reading.Breaks))))
		assertNoSale(t, got, "")
		if st := rebuildState(cancelled); st.slowDeclinePending || !st.swingDeclineWatched || !st.indecision {
			t.Errorf("%s: cancelled with the latch kept and none written again, got %+v", name, st)
		}
		assertUntouched(t, Apply(cancelled, "", slowDeclineBand+1, withBlock(block)), "")
	}
}

// A trend that misses never cancels ahead of its bar: a pending ladder's fifth fill
// whose bar the series has not closed is waited on under a trend not read as under
// one that holds — no row, nothing sold at the band — and the cancellation lands on
// the tick that has closed the bar.
func TestSwingDeclineAMissingTrendWaitsForTheFillsBarToo(t *testing.T) {
	needsTheTrend(t)
	fifth, band := stairFifthFill(t), slowDeclineBand+1
	open := withBlock(withSmcTrend(stairBlock(stairTurns, 29), 0, 0))
	assertUntouched(t, Apply(fifth, "", underTheBand, open), "")
	assertUntouched(t, Apply(fifth, "", band, open), "")
	closed := withBlock(withSmcTrend(stairBlock(stairTurns, 30), 0, 0))
	if got := Apply(fifth, "", band, closed); len(got.Rows) != 1 || got.Rows[0].Event != EventCancelled {
		t.Errorf("the tick that has closed the fill's bar must cancel, got %+v", got)
	}
}

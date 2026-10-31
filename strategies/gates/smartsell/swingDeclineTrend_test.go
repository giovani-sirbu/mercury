package smartsell

import (
	"fmt"
	"slices"
	"testing"
	"time"

	"github.com/giovani-sirbu/mercury/strategies/gates/smartsell/swingdecline"
	"github.com/giovani-sirbu/mercury/trades/aggragates"
)

// The Trend Direction row the fixtures serve (stairBlock, patternTape) just meets
// swingdecline.MinSmcTrendDown: as many of the dashboard's
// patternTrendValid rows read down as the constant asks. shortTrendDown is a row
// short of it, so no test here leans on the constant's value. shortTrendReason is
// what the swing decline names beside the window under the second.
const (
	patternTrendValid = 3
)

var shortTrendReason = fmt.Sprintf("smc trend direction down %d/%d short of %d needed", shortTrendDown, patternTrendValid, swingdecline.MinSmcTrendDown)

// notReadTrendReason is what the swing decline names beside the window when
// sophos served no Trend Direction row.
const notReadTrendReason = "smc trend direction not read"

// needsTheTrend skips a test that has a trend fail a ladder when the condition is
// off: at swingdecline.MinSmcTrendDown zero or below no trend is read,
// so none can fail.
func needsTheTrend(t *testing.T) {
	t.Helper()
	if swingdecline.MinSmcTrendDown <= 0 {
		t.Skip("swingdecline.MinSmcTrendDown is zero or below: the condition is off and no trend fails a ladder")
	}
}

// withSmcTrend is the block serving this Trend Direction row beside its series:
// down of the valid rows read down.
func withSmcTrend(block aggragates.SmartSellIndicators, down, valid int) aggragates.SmartSellIndicators {
	block.SmcTrendDown, block.SmcTrendValid = down, valid
	return block
}

// The staircase's shape holds at its fourth fill's bar, but the SMC trend decides
// beside it: with the Trend Direction down on a row short of the need, or with no
// row read — a stray count with no row behind it included — the ladder does not go
// pending — no row, no latch, and nothing activated or sold on any proposal, in buy
// or in trailingBuy — where the same tick under a trend that holds writes the pending row.
func TestSwingDeclineATrendShortOfTheNeedKeepsAHoldingShapeFromGoingPending(t *testing.T) {
	needsTheTrend(t)
	withBothDeclines(t)
	holding := stairFourthBlock()
	if got := applyAtAGain(stairFourth(), "", withBlock(holding)); len(got.Rows) != 1 || got.Rows[0].Event != EventPending {
		t.Fatalf("fixture drifted: under a trend that holds the staircase goes pending at bar 24, got %+v", got)
	}
	for name, trend := range map[string][2]int{"a row short of the need": {shortTrendDown, patternTrendValid}, "no row read": {0, 0}, "a count, no row read": {patternTrendValid, 0}} {
		t.Run(name, func(t *testing.T) {
			assertSoldNever(t, stairFourth(), withBlock(withSmcTrend(holding, trend[0], trend[1])))
		})
	}
}

// A pending, latched ladder's fifth fill is judged on the trend too: the shape
// holds at its bar, the trend reads a row short of the need, and the exit is
// cancelled — the row names the window and the trend alone, under the swing
// decline, the one feature the ladder is pending on, the latch stays and the shared
// action does nothing, on the cancel tick and
// after it. The ladder goes pending again by the trigger alone, on the next tick, once
// the trend meets the need.
func TestSwingDeclineATrendShortOfTheNeedCancelsAJudgedFillWhoseShapeHolds(t *testing.T) {
	needsTheTrend(t)
	withBothDeclines(t)
	short := withSmcTrend(stairFifthBlock(), shortTrendDown, patternTrendValid)
	reading := stairReading(t, stairFifthFill(t), short, false)
	if len(reading.Breaks) != 2 || reading.Breaks[1] != shortTrendReason {
		t.Fatalf("fixture drifted: the shape holds and the breaks name the window and the trend alone, got %q", reading.Breaks)
	}
	fifth := stairFifthFill(t)
	cancelled, got := engineTick(fifth, "trailingBuy", armingPrice(fifth), stairOpen(31), withBlock(short))
	assertRow(t, got, CancelledRow(stairPrices[4], swingFinding(reading.Breaks)))
	assertNoSale(t, got, "trailingBuy")
	if st := rebuildState(cancelled); st.slowDeclinePending || !st.swingDeclineWatched || !st.earlyTakeProfit {
		t.Fatalf("watched, no longer pending and still latched, got %+v", st)
	}
	assertSoldNever(t, cancelled, withBlock(short))

	met := stairFifthBlock()
	pending, again := tickAtAGain(cancelled, "", stairOpen(32), withBlock(met))
	assertRow(t, again, PendingRow(stairPrices[4], swingFinding(stairReading(t, cancelled, met, true).Reasons)))
	if st := rebuildState(pending); !st.slowDeclinePending || st.slowDeclinePendingFrom != stairPrices[4] {
		t.Errorf("pending again from the fifth fill, got %+v", st)
	}
}

// The trend is the tick's own, not the fill's bar's: the fourth fill's bar closes
// with the Trend Direction a row short of the need and nothing is written, a bar
// later it meets the need and the ladder goes pending on that tick, its row naming
// the window up to that bar and the trend it met. Nothing bounds the bars after
// the fill: from the fill's bar on, a trend that meets the need triggers on every
// bar the staircase holds on, and one short of it on none. The ticks run at a print
// over the ladder's break even (gainPrint) with a buy order resting, where the ladder in buy is activated
// nothing and the row stands alone.
func TestSwingDeclineATrendThatTurnsLaterStillTriggers(t *testing.T) {
	needsTheTrend(t)
	withLinearDecline(t, false)
	trade := stairFourth()
	gain := gainPrint(trade)
	assertUntouched(t, Apply(withRestingOrder(trade), "", gain, withBlock(withSmcTrend(stairFourthBlock(), shortTrendDown, patternTrendValid))), "")

	turned := withSmcTrend(stairBlock(stairTurns, 25), patternTrendDown, patternTrendValid)
	pending, got := tickResting(trade, "", gain, stairOpen(26), withBlock(turned))
	assertRow(t, got, PendingRow(stairPrices[3], indecisionFinding(nil), swingFinding(stairReadingAsOfNow(t, trade, turned, true).Reasons)))
	if st := rebuildState(pending); !st.slowDeclinePending || st.slowDeclinePendingFrom != stairPrices[3] || !st.earlyTakeProfit {
		t.Errorf("pending from the fourth fill and latched, got %+v", st)
	}

	for bar := 24; bar <= 30; bar++ {
		met := withSmcTrend(stairBlock(stairTurns, bar), patternTrendDown, patternTrendValid)
		short := withSmcTrend(stairBlock(stairTurns, bar), shortTrendDown, patternTrendValid)
		_, holds := readsAsOf(trade, met)
		if got := Apply(withRestingOrder(trade), "", gain, withBlock(met)); (len(got.Rows) != 0) != holds {
			t.Errorf("a trend that meets the need on the series' bar %d: pending row %v, want %v", bar, len(got.Rows) != 0, holds)
		}
		assertUntouched(t, Apply(withRestingOrder(trade), "", gain, withBlock(short)), "")
	}
}

// The fill that takes a pending ladder to its last depth is judged by nobody, the
// trend included: the series has closed its bar, the trend is short of the need or
// not read, and the exit stays pending and no row of the gate is written. The shared
// action then reads the trend as its trigger: a row short of the swing's need still
// reads a row, so it goes on every tick; no row read holds it with the trend's row.
func TestSwingDeclineTheLastDepthFillIsNotJudgedOnTheTrendEither(t *testing.T) {
	needsTheTrend(t)
	withBothDeclines(t)
	seventh := withRows(stairLadder(0, 9, 15, 24, 30, 36, 42), PendingRow(stairPrices[6], swingFinding(nil)))
	eighth := withFill(seventh, stairPrices[7], stairOpen(48).Add(time.Minute))
	assertActedOn(t, eighth, withBlock(withSmcTrend(stairBlock(stairDecline, 48), shortTrendDown, patternTrendValid)))
	assertSoldNever(t, eighth, withBlock(withSmcTrend(stairBlock(stairDecline, 48), 0, 0)), saleHeldRow(GateSlowDecline, stairPrices[7], "SMC trend not read", []string{FeatureSwingDecline}))
}

// ExitReached is Apply's sale on the empty proposal and an activation is no sale, so
// under the trend too it follows the sale of the ladder resting in trailingBuy and reads
// false on the ladder resting in buy, whose trailingBuy the action activates. The shared
// action follows the trend: under a trend that holds — on the tick that goes pending
// and on the one that judges a new fill, at the need and above it — the ladder in buy
// has its trailingBuy activated at a print under its break even and the one in trailingBuy is
// sold, with the gate's row beside; under one a row short of the need or not read neither
// tick acts, the new fill being cancelled and the ladder never pending.
func TestSwingDeclineTheActionFollowsTheTrendAndExitReachedFollowsItsSale(t *testing.T) {
	needsTheTrend(t)
	withBothDeclines(t)
	for name, tc := range map[string]struct {
		trade aggragates.Trades
		bar   int
	}{"going pending": {stairFourth(), 24}, "judging a new fill": {stairFifthFill(t), 30}} {
		resting := restingIn(tc.trade, "trailingBuy")
		for _, trend := range []struct {
			down, valid int
			acts        bool
		}{{patternTrendValid, patternTrendValid, patternTrendDown <= patternTrendValid}, {patternTrendDown, patternTrendValid, true}, {shortTrendDown, patternTrendValid, false}, {0, 0, false}} {
			block := withBlock(withSmcTrend(stairBlock(stairTurns, tc.bar), trend.down, trend.valid))
			rows := applyAtAGain(tc.trade, "", block).Rows
			t.Run(fmt.Sprintf("%s, trend %d/%d", name, trend.down, trend.valid), func(t *testing.T) {
				if trend.acts {
					assertActedOn(t, tc.trade, block, rows...)
				} else {
					assertSoldNever(t, tc.trade, block, rows...)
				}
				for _, tick := range everyTick(tc.trade) {
					if ExitReached(tc.trade, tick.price, block.SmartSell) || ExitReached(resting, tick.price, block.SmartSell) != trend.acts {
						t.Errorf("at %v: ExitReached %v resting in buy and %v resting in trailingBuy, want false and %v", tick.price, ExitReached(tc.trade, tick.price, block.SmartSell), ExitReached(resting, tick.price, block.SmartSell), trend.acts)
					}
				}
			})
		}
	}
}

// The swing's trend is read at the trigger and at a new fill's judgement, never again: a
// ladder that went pending under a trend that held stays pending whatever the Trend
// Direction reads by then, and the gate writes no row for it. The shared action reads the
// trend of every later tick as its trigger (sharedTriggerOf): a row short of the swing's
// need still reads a row, so the ladder keeps being acted on; no row down, or no row read,
// meets no rule and holds the action with the trend's row and nothing else.
func TestSwingDeclineAPendingLadderKeepsBeingActedOnWhateverTheTrendReadsLater(t *testing.T) {
	needsTheTrend(t)
	withBothDeclines(t)
	pending := stairPending(t)
	assertActedOn(t, pending, withBlock(withSmcTrend(stairBlock(stairTurns, 26), shortTrendDown, patternTrendValid)))
	for name, trend := range map[string][2]int{"no row down": {0, patternTrendValid}, "no row read": {0, 0}} {
		t.Run(name, func(t *testing.T) {
			ai := withBlock(withSmcTrend(stairBlock(stairTurns, 26), trend[0], trend[1]))
			assertSoldNever(t, pending, ai, saleHeldRow(GateSlowDecline, stairPrices[3], trendHeldWords(ai.SmartSell, false), rebuildState(pending).pendingFeatures))
		})
	}
}

// A judgement under a trend that misses names it beside the shape, whatever the
// shape reads: a Trend Direction row not read, a count with no row read behind it,
// and a trend a row short of the need beside a shape that itself breaks — then the
// row names the shape's breaks and the trend after them. Under each the ladder is
// cancelled at its fifth fill, the latch stays, and the shared action activates and
// sells nothing on that tick, an arming proposal included, or on any tick after it.
func TestSwingDeclineAFillJudgedUnderAnyTrendThatMissesIsCancelledNamingIt(t *testing.T) {
	needsTheTrend(t)
	withBothDeclines(t)
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
		cancelled, got := engineTick(fifth, "trailingBuy", armingPrice(fifth), stairOpen(31), withBlock(block))
		assertRow(t, got, CancelledRow(stairPrices[4], swingFinding(slices.Clone(reading.Breaks))))
		assertNoSale(t, got, "trailingBuy")
		if st := rebuildState(cancelled); st.slowDeclinePending || !st.swingDeclineWatched || !st.earlyTakeProfit {
			t.Errorf("%s: cancelled with the latch kept and none written again, got %+v", name, st)
		}
		assertSoldNever(t, cancelled, withBlock(block))
	}
}

// A trend that misses never cancels ahead of its bar: a pending ladder's fifth fill
// whose bar the series has not closed is waited on under a trend not read as under
// one that holds — no row, nothing activated or sold by the shared action on any
// proposal, in buy or in trailingBuy — and the cancellation lands on the tick that has
// closed the bar.
func TestSwingDeclineAMissingTrendWaitsForTheFillsBarToo(t *testing.T) {
	needsTheTrend(t)
	withBothDeclines(t)
	fifth := stairFifthFill(t)
	assertSoldNever(t, fifth, withBlock(withSmcTrend(stairBlock(stairTurns, 29), 0, 0)))
	closed := withBlock(withSmcTrend(stairBlock(stairTurns, 30), 0, 0))
	if got := Apply(fifth, "", slowDeclineBand+1, closed); len(got.Rows) != 1 || got.Rows[0].Event != EventCancelled {
		t.Errorf("the tick that has closed the fill's bar must cancel, got %+v", got)
	}
}

// patternTrendDown is the count of rows down that just meets swingdecline.MinSmcTrendDown,
// shortTrendDown one a row short of it.
var (
	patternTrendDown = swingdecline.MinSmcTrendDown
	shortTrendDown   = swingdecline.MinSmcTrendDown - 1
)

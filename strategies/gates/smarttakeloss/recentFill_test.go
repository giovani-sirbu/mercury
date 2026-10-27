package smarttakeloss

import (
	"testing"
	"time"

	"github.com/giovani-sirbu/mercury/internal/testutil"
	"github.com/giovani-sirbu/mercury/strategies/gates/smarttakeloss/earlytakeprofit"
	"github.com/giovani-sirbu/mercury/trades/aggragates"
)

// windowFrom is the reading with its fill window opening at from; the zero
// time serves none.
func windowFrom(reading aggragates.AIIndicators, from time.Time) aggragates.AIIndicators {
	reading.SmartTakeLoss.QuietDeclineFillFrom = 0
	if !from.IsZero() {
		reading.SmartTakeLoss.QuietDeclineFillFrom = from.UnixMilli()
	}
	return reading
}

// The newest fill is compared with QuietDeclineFillFrom in milliseconds, the
// window's open included: at it, or anywhere inside its first millisecond,
// the fill is recent; a millisecond or a nanosecond before, it is not.
// Nothing bounds the stamp from above, so a fill a day past the window's open
// — in the bar still forming — is recent. No window served, a negative one,
// or a newest fill without a stamp fails closed. Switched off, the rule
// always holds.
func TestRecentFillReadsTheNewestFillAgainstTheFillWindow(t *testing.T) {
	from := testutil.At("10:00:00")
	block := aggragates.SmartTakeLossIndicators{QuietDeclineFillFrom: from.UnixMilli()}
	for name, tc := range map[string]struct {
		stamp  time.Time
		recent bool
	}{
		"at the window's open":           {from, true},
		"inside its first millisecond":   {from.Add(time.Millisecond - time.Nanosecond), true},
		"a millisecond after":            {from.Add(time.Millisecond), true},
		"in the bar forming a day later": {from.Add(24*time.Hour + 38*time.Minute), true},
		"a millisecond before":           {from.Add(-time.Millisecond), false},
		"a nanosecond before":            {from.Add(-time.Nanosecond), false},
		"without a stamp":                {time.Time{}, false},
	} {
		if got := recentFill(entryFill{Price: slowDeclineLastFill, At: tc.stamp}, block); got != tc.recent {
			t.Errorf("%s: recent %v, want %v", name, got, tc.recent)
		}
	}
	for name, served := range map[string]int64{"no window served": 0, "a negative window": -1} {
		if recentFill(entryFill{Price: slowDeclineLastFill, At: from}, aggragates.SmartTakeLossIndicators{QuietDeclineFillFrom: served}) {
			t.Errorf("%s: a fill must not read recent", name)
		}
	}

	withRecentFillRule(t, false)
	for _, fill := range []entryFill{{}, {At: from.Add(-time.Hour)}, {At: from}} {
		for _, served := range []aggragates.SmartTakeLossIndicators{{}, block} {
			if !recentFill(fill, served) {
				t.Errorf("switched off, the rule must hold for %+v on %+v", fill, served)
			}
		}
	}
}

// No bar bounds the newest fill from above, and only the newest fill is
// weighed. A watched ladder whose newest fill printed in the bar still
// forming — a day and more after the oldest of the last closed bars opened —
// goes pending on the verdict, at that fill; so does one whose earlier depths
// filled before the window opened and whose newest fill filled after. A
// millisecond short of the window's open, the newest fill marks nothing. The ticks
// run at a print over the ladder's break even (gainPrint) with a buy order resting, where the ladder in buy
// is activated nothing and the pending row stands alone.
func TestApplyGoesPendingOnTheNewestFillAloneWhereverItSitsAfterTheWindow(t *testing.T) {
	trade := watchedTrade()
	first, newest := trade.History[0].CreatedAt, trade.History[watchedFills-1].CreatedAt
	oldestClosedBar := testutil.At("17:00:00").Add(-24 * time.Hour)
	between := testutil.At("12:00:00")
	if newest.Sub(oldestClosedBar) <= 24*time.Hour || !first.Before(between) || !between.Before(newest) {
		t.Fatal("fixture drifted: the newest fill prints in the bar still forming, the first before noon")
	}
	for _, from := range []time.Time{oldestClosedBar, between} {
		got := applyAtAGain(trade, "", windowFrom(slowDeclineBlock(true), from))
		assertRow(t, got, quietPending(slowDeclineLastFill))
		assertNoSale(t, got, "")
	}
	assertNoSlowDeclineRow(t, applyAtAGain(trade, "", windowFrom(slowDeclineBlock(true), newest.Add(time.Millisecond))), "")
}

// Switched off, going pending reads no fill window: the verdict, the leg on
// and quiet counted from the newest fill and the decline read recently each
// mark the ladder pending with no window served and with one that opens
// after the newest fill — the release before the rule. Switched on, the same
// ticks mark nothing. The ticks run at a print over the ladder's break even
// (gainPrint) with a buy order resting, so a ladder marked pending is activated nothing and the one row is
// the mark.
func TestSwitchedOffGoingPendingReadsNoFillWindow(t *testing.T) {
	late := testutil.At("17:38:00").Add(time.Minute)
	readings := map[string]aggragates.AIIndicators{
		"the verdict":               slowDeclineBlock(true),
		"the leg on and quiet":      quietLegBlock(false, testutil.At("17:00:00"), fillBarClosed),
		"the decline read recently": withRecent(brokenReading(), testutil.At("16:00:00"), testutil.At("09:00:00")),
	}
	for _, on := range []bool{true, false} {
		withRecentFillRule(t, on)
		for name, reading := range readings {
			for _, from := range []time.Time{{}, late} {
				got := applyAtAGain(watchedTrade(), "", windowFrom(reading, from))
				if on {
					assertNoSlowDeclineRow(t, got, "")
					continue
				}
				if len(got.Rows) != 1 || got.Rows[0].Price != slowDeclineLastFill || got.Reason != "" || got.Activated {
					t.Fatalf("switched off, %s with the window from %v must mark the ladder at its newest fill, got %+v", name, from, got)
				}
			}
		}
	}
}

// The fill window's position never touches a ladder already pending. Its new
// fill is judged whatever the window opens at — confirmed on the leg on and
// quiet or on the decline read recently, cancelled on a broken reading — with a
// window that opens after that fill as with one that opens before it. A window
// that is not read at all is a reading not taken: the quiet decline's judgement
// needs the window served, and the fill waits, with no row and nothing done, in buy
// or in stopLoss, at a print under the break even too. The judgements are read at
// gainPrint with a buy order resting (applyAtAGain), where the action waits and the
// row stands alone.
func TestTheJudgementOfANewFillIgnoresTheFillWindow(t *testing.T) {
	withQuietDecline(t, true)
	afterTheSixth := testutil.At("23:09:00").Add(time.Minute)
	stood, oldest := testutil.At("22:00:00"), testutil.At("12:00:00")
	for _, from := range []time.Time{afterTheSixth, testutil.At("00:00:00")} {
		confirmed := applyAtAGain(pendingAtFive(), "", windowFrom(legOnAndQuiet(), from))
		assertRow(t, confirmed, sixthConfirmed())
		recently := applyAtAGain(pendingAtFive(), "", windowFrom(withRecent(brokenReading(), stood, oldest), from))
		assertRow(t, recently, ConfirmedRow(sixthFill, quietFinding(quietDeclineRecentReasons)))
		cancelled := applyAtAGain(pendingAtFive(), "", windowFrom(brokenReading(), from))
		assertRow(t, cancelled, sixthCancelled())
		for _, got := range []Result{confirmed, recently, cancelled} {
			assertNoSale(t, got, "")
		}
	}

	for name, reading := range map[string]aggragates.AIIndicators{
		"the leg on and quiet":      legOnAndQuiet(),
		"the decline read recently": withRecent(brokenReading(), stood, oldest),
		"a broken reading":          brokenReading(),
	} {
		for _, state := range []string{"buy", "stopLoss"} {
			for _, position := range []string{"", "stopLoss"} {
				assertNoSlowDeclineRow(t, Apply(restingIn(pendingAtFive(), state), position, armingPrice(pendingAtFive()), windowFrom(reading, time.Time{})), position)
			}
		}
		if st := rebuildState(pendingAtFive()); !slowDeclineFillUnjudged(pendingAtFive(), st) {
			t.Fatalf("%s: the fill must still wait for its judgement", name)
		}
	}
}

// A pending ladder stays pending however old its newest fill grows: tick
// after tick the window opens later — past the fill, then a day past it —
// and nothing is written, the ladder reads pending from its newest fill, its
// take profit still reads that fill from break even up, and the shared action
// acts on it on every tick: the stopLoss of the ladder in buy activated at a
// print under its break even (nothing over it, where a sale books a gain), the
// ladder in stopLoss sold. The ticks that write nothing run at a print over the break
// even (gainPrint) with a buy order resting, where the ladder in buy is activated nothing.
func TestAPendingLadderStaysPendingAsItsFillAgesPastTheWindow(t *testing.T) {
	withQuietDecline(t, true)
	trade := pendingTrade()
	newest := trade.History[watchedFills-1].CreatedAt
	for index, from := range []time.Time{newest.Add(time.Millisecond), newest.Add(6 * time.Hour), newest.Add(30 * time.Hour)} {
		for _, reading := range []aggragates.AIIndicators{slowDeclineBlock(false), slowDeclineBlock(true), brokenReading()} {
			var got Result
			trade, got = tickAtAGain(trade, "", from.Add(time.Duration(index)*time.Minute), windowFrom(reading, from))
			assertNoSlowDeclineRow(t, got, "")
		}
		if st := rebuildState(trade); !st.slowDeclinePending || st.slowDeclinePendingFrom != slowDeclineLastFill {
			t.Fatalf("window from %v: the ladder must stay pending from its newest fill, got %+v", from, st)
		}
		armed, dead := armingPrice(trade), windowFrom(slowDeclineBlock(false), from)
		assertActivated(t, Apply(trade, "stopLoss", armed, dead), StopLossActivatedRow(slowDeclineLastFill, -handLossPct(trade, armed), FeatureQuietDecline))
		assertForced(t, Apply(restingIn(trade, "stopLoss"), "stopLoss", armed, dead), reasonSlowDecline)
		assertActivated(t, Apply(trade, "", slowDeclineBand, dead), StopLossActivatedRow(slowDeclineLastFill, -handLossPct(trade, slowDeclineBand), FeatureQuietDecline))
		assertNoSlowDeclineRow(t, applyAtAGain(trade, "", dead), "")
	}
	price := betweenTheTakeProfits(t, trade)
	if got := TakeProfitPercentage(trade, price, breakEvenReading); got != moveAgainst(price, slowDeclineLastFill) {
		t.Fatalf("the aged pending ladder's take profit must read its newest fill, got %v", got)
	}
}

// The indecision latch reads no fill window: a watched ladder whose newest
// fill sits before the window is latched on the indecision all the same, as
// with no window served. The first-fill hold reads none either
// (TestEntryHoldIgnoresTheFillWindow).
func TestTheEarlyTakeProfitLatchIgnoresTheFillWindow(t *testing.T) {
	withEarlyTakeProfit(t, true)
	trade := testutil.LadderTrade(false, fills(earlytakeprofit.ArmDepth, "17:38:00")...)
	for _, from := range []time.Time{{}, testutil.At("17:38:00").Add(time.Hour)} {
		got := Apply(trade, "", underTheBand, windowFrom(indecisionReading(), from))
		assertRow(t, got, LatchedRow(trade.PositionPrice, indecisionFinding(indecisionReasons)))
	}
}

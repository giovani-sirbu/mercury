package smarttakeloss

import (
	"testing"
	"time"

	"github.com/giovani-sirbu/mercury/trades/aggragates"
	"github.com/giovani-sirbu/mercury/trades/internal/testutil"
)

// windowFrom is the reading with its fill window opening at from; the zero
// time serves none.
func windowFrom(reading aggragates.AIIndicators, from time.Time) aggragates.AIIndicators {
	reading.SmartTakeLoss.SlowDeclineFillFrom = 0
	if !from.IsZero() {
		reading.SmartTakeLoss.SlowDeclineFillFrom = from.UnixMilli()
	}
	return reading
}

// The newest fill is compared with SlowDeclineFillFrom in milliseconds, the
// window's open included: at it, or anywhere inside its first millisecond,
// the fill is recent; a millisecond or a nanosecond before, it is not.
// Nothing bounds the stamp from above, so a fill a day past the window's open
// — in the bar still forming — is recent. No window served, a negative one,
// or a newest fill without a stamp fails closed. Switched off, the rule
// always holds.
func TestRecentFillReadsTheNewestFillAgainstTheFillWindow(t *testing.T) {
	from := testutil.At("10:00:00")
	block := aggragates.SmartTakeLossIndicators{SlowDeclineFillFrom: from.UnixMilli()}
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
		if recentFill(entryFill{Price: slowDeclineLastFill, At: from}, aggragates.SmartTakeLossIndicators{SlowDeclineFillFrom: served}) {
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
// millisecond short of the window's open, the newest fill marks nothing.
func TestApplyGoesPendingOnTheNewestFillAloneWhereverItSitsAfterTheWindow(t *testing.T) {
	trade := watchedTrade()
	first, newest := trade.History[0].CreatedAt, trade.History[watchedFills-1].CreatedAt
	oldestClosedBar := testutil.At("17:00:00").Add(-24 * time.Hour)
	between := testutil.At("12:00:00")
	if newest.Sub(oldestClosedBar) <= 24*time.Hour || !first.Before(between) || !between.Before(newest) {
		t.Fatal("fixture drifted: the newest fill prints in the bar still forming, the first before noon")
	}
	for _, from := range []time.Time{oldestClosedBar, between} {
		got := Apply(trade, "", underTheBand, windowFrom(slowDeclineBlock(true), from))
		assertRow(t, got.SlowDecline, SlowDeclineMessage("buy", slowDeclineReasons), slowDeclineLastFill)
		assertNoSale(t, got, "")
	}
	assertNoSlowDeclineRow(t, Apply(trade, "", underTheBand, windowFrom(slowDeclineBlock(true), newest.Add(time.Millisecond))), "")
}

// Switched off, going pending reads no fill window: the verdict, the leg on
// and quiet counted from the newest fill and the decline read recently each
// mark the ladder pending with no window served and with one that opens
// after the newest fill — the release before the rule. Switched on, the same
// ticks mark nothing.
func TestSwitchedOffGoingPendingReadsNoFillWindow(t *testing.T) {
	late := testutil.At("17:38:00").Add(time.Minute)
	readings := map[string]aggragates.AIIndicators{
		"the verdict":               slowDeclineBlock(true),
		"the leg on and quiet":      quietLegBlock(false, testutil.At("17:00:00"), fillBarClosed),
		"the decline read recently": withRecent(brokenAtTheBand(), testutil.At("16:00:00"), testutil.At("09:00:00")),
	}
	for _, on := range []bool{true, false} {
		withRecentFillRule(t, on)
		for name, reading := range readings {
			for _, from := range []time.Time{{}, late} {
				got := Apply(watchedTrade(), "", underTheBand, windowFrom(reading, from))
				if on {
					assertNoSlowDeclineRow(t, got, "")
					continue
				}
				if got.SlowDecline == nil || got.SlowDecline.Price != slowDeclineLastFill || got.Reason != "" {
					t.Fatalf("switched off, %s with the window from %v must mark the ladder at its newest fill, got %+v", name, from, got)
				}
			}
		}
	}
}

// The fill window never touches a ladder already pending. Its new fill is
// judged whatever the window — confirmed on the leg on and quiet or on the
// decline read recently, cancelled on a broken reading — with no window
// served and with one that opens after that fill.
func TestTheJudgementOfANewFillIgnoresTheFillWindow(t *testing.T) {
	afterTheSixth := testutil.At("23:09:00").Add(time.Minute)
	stood, oldest := testutil.At("22:00:00"), testutil.At("12:00:00")
	for _, from := range []time.Time{{}, afterTheSixth} {
		confirmed := Apply(pendingAtFive(), "", underJudgeBand, windowFrom(legOnAndQuiet(), from))
		assertRow(t, confirmed.SlowDecline, SlowDeclineMessage("buy", slowDeclineReasons), sixthFill)
		recently := Apply(pendingAtFive(), "", underJudgeBand, windowFrom(withRecent(brokenReading(), stood, oldest), from))
		assertRow(t, recently.SlowDecline, SlowDeclineMessage("buy", slowDeclineRecentReasons), sixthFill)
		cancelled := Apply(pendingAtFive(), "", underJudgeBand, windowFrom(brokenReading(), from))
		assertRow(t, cancelled.SlowDecline, SlowDeclineCancelMessage("buy", slowDeclineBreakReasons), sixthFill)
		for _, got := range []Result{confirmed, recently, cancelled} {
			assertNoSale(t, got, "")
		}
	}
}

// A pending ladder stays pending however old its newest fill grows: tick
// after tick the window opens later — past the fill, then a day past it —
// and nothing is written, the ladder reads pending from its newest fill, its
// take profit still reads that fill from break even up, and the band sells it.
func TestAPendingLadderStaysPendingAsItsFillAgesPastTheWindow(t *testing.T) {
	trade := pendingTrade()
	newest := trade.History[watchedFills-1].CreatedAt
	for index, from := range []time.Time{newest.Add(time.Millisecond), newest.Add(6 * time.Hour), newest.Add(30 * time.Hour)} {
		for _, reading := range []aggragates.AIIndicators{slowDeclineBlock(false), slowDeclineBlock(true), brokenAtTheBand()} {
			var got Result
			trade, got = engineTick(trade, "", underTheBand, from.Add(time.Duration(index)*time.Minute), windowFrom(reading, from))
			assertNoSlowDeclineRow(t, got, "")
		}
		if st := rebuildState(trade); !st.slowDeclinePending || st.slowDeclinePendingFrom != slowDeclineLastFill {
			t.Fatalf("window from %v: the ladder must stay pending from its newest fill, got %+v", from, st)
		}
		assertForced(t, Apply(trade, "", slowDeclineBand, windowFrom(slowDeclineBlock(false), from)), reasonSellBand)
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
func TestTheIndecisionLatchIgnoresTheFillWindow(t *testing.T) {
	trade := testutil.LadderTrade(false, fills(IndecisionArmDepth, "17:38:00")...)
	for _, from := range []time.Time{{}, testutil.At("17:38:00").Add(time.Hour)} {
		got := Apply(trade, "", underTheBand, windowFrom(indecisionReading(), from))
		assertRow(t, got.Indecision, IndecisionMessage("buy", indecisionReasons), trade.PositionPrice)
	}
}

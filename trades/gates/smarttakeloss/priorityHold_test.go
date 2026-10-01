package smarttakeloss

import (
	"strings"
	"testing"
	"time"

	"github.com/giovani-sirbu/mercury/trades/aggragates"
	"github.com/giovani-sirbu/mercury/trades/gates/cooldown"
	"github.com/giovani-sirbu/mercury/trades/internal/testutil"
)

// withDepthPriorityPause is withQuietSlowDeclineExit for the pause a depth
// priority hold puts on the smart take loss.
func withDepthPriorityPause(t *testing.T, on bool) {
	t.Helper()
	previous := depthPriorityHoldPauses
	depthPriorityHoldPauses = on
	t.Cleanup(func() { depthPriorityHoldPauses = previous })
}

// withLogRow is the trade carrying one more row, message and stamp given.
func withLogRow(trade aggragates.Trades, message string, at time.Time) aggragates.Trades {
	row := aggragates.TradesLogs{Message: message, Type: aggragates.LOG_INFO, Price: trade.PositionPrice, CreatedAt: at}
	trade.Logs = append(append([]aggragates.TradesLogs(nil), trade.Logs...), row)
	return trade
}

// resetRow is the reset row, byte for byte, framed with the trade's state.
func resetRow(positionType string) string {
	return "Hold " + positionType + ": smartTakeLoss: quiet slow decline reset, depth priority holds this ladder"
}

// A ladder is held while it carries a row naming cooldown.DepthPriorityHoldMarker
// stamped strictly after its newest fill, newest in slice order. A row at the
// fill's own stamp or before it, a row without a stamp, another gate's row, a
// newest fill without a stamp and a ladder with no fill hold nothing, and
// switched off no row holds.
func TestDepthPriorityHeldReadsTheGatesRowAfterTheNewestFill(t *testing.T) {
	newest := testutil.At("17:38:00")
	entryHold := "Hold entry: " + cooldown.DepthPriorityHoldMarker + ", LINK/USDT at depth 8 of 8 holds the wallet until it closes, this ladder waits at depth 4 of 8"
	for name, tc := range map[string]struct {
		trade aggragates.Trades
		held  bool
	}{
		"the gate's row a nanosecond after the fill": {heldBy(watchedTrade(), newest.Add(time.Nanosecond)), true},
		"the gate's entry row after the fill":        {withLogRow(watchedTrade(), entryHold, newest.Add(time.Hour)), true},
		"the gate's row, then later rows":            {withLogRow(heldBy(watchedTrade(), newest.Add(time.Minute)), "BUY_TO_STOPLOSS", newest.Add(time.Hour)), true},
		"the gate's row at the fill's stamp":         {heldBy(watchedTrade(), newest), false},
		"the gate's row before the fill":             {heldBy(watchedTrade(), newest.Add(-time.Minute)), false},
		"the gate's row without a stamp":             {heldBy(watchedTrade(), time.Time{}), false},
		"depth spacing's row after the fill":         {withLogRow(watchedTrade(), "Hold stopLoss: cooldown: depths too close (depth 4, step 3), next add parked for 1h0m0s", newest.Add(time.Hour)), false},
		"a slow-decline row after the fill":          {withLogRow(watchedTrade(), SlowDeclineResetMessage("buy"), newest.Add(time.Hour)), false},
		"no fill":                                    {heldBy(testutil.LadderTrade(false), newest), false},
	} {
		if got := rebuildState(tc.trade).depthPriorityHeld; got != tc.held {
			t.Errorf("%s: held %v, want %v", name, got, tc.held)
		}
	}

	unstamped := watchedTrade()
	unstamped.History[watchedFills-1].CreatedAt = time.Time{}
	if rebuildState(heldBy(unstamped, newest.Add(time.Hour))).depthPriorityHeld {
		t.Error("a newest fill without a stamp holds nothing")
	}
	outOfOrder := watchedTrade()
	outOfOrder.History[watchedFills-2].CreatedAt, outOfOrder.History[watchedFills-1].CreatedAt = testutil.At("16:00:00"), testutil.At("12:00:00")
	if st := rebuildState(heldBy(outOfOrder, testutil.At("14:00:00"))); !st.depthPriorityHeld || st.lastFill().Price != slowDeclineLastFill {
		t.Errorf("the newest fill is the last in slice order, stamped 12:00, whatever an earlier row's stamp: %+v", st)
	}

	withDepthPriorityPause(t, false)
	if rebuildState(heldBy(watchedTrade(), newest.Add(time.Hour))).depthPriorityHeld {
		t.Error("switched off, no row holds the ladder")
	}
}

// The gate's row alone moves no fold: the ladder still reads pending. On its
// first held tick a pending ladder hands back exactly one slow-decline row —
// the reset row, framed with the trade's raw state, at its newest fill's
// price, never the position price — and sells nothing, at the band or past
// it, a close it proposes passing untouched. From the next tick it reads
// watched and not pending, still held, and no slow-decline row is written and
// nothing is sold again, whatever sophos serves. A reading that serves the
// indecision latches the ladder besides, which writes the indecision row
// beside the reset row on the same tick.
func TestAHeldPendingLadderIsResetOnceAndNoSlowDeclineRowFollows(t *testing.T) {
	held := heldBy(pendingTrade(), testutil.At("18:05:00"))
	if st := rebuildState(held); !st.slowDeclinePending || !st.depthPriorityHeld {
		t.Fatalf("fixture drifted: the gate's row leaves the ladder pending and holds it, got %+v", st)
	}
	if SlowDeclineResetMarker != "smartTakeLoss: quiet slow decline reset, depth priority holds this ladder" || SlowDeclineResetMessage("buy") != resetRow("buy") {
		t.Fatalf("the reset marker is schema and must not move, got %q", SlowDeclineResetMessage("buy"))
	}
	trade, got := engineTick(held, "", slowDeclineBand+1, testutil.At("18:10:00"), slowDeclineBlock(true))
	assertRow(t, got.SlowDecline, resetRow("buy"), slowDeclineLastFill)
	assertNoSale(t, got, "")
	if got.Indecision != nil {
		t.Fatalf("a reading without the indecision latches nothing, got %+v", got)
	}
	if st := rebuildState(trade); !st.slowDeclineWatched || st.slowDeclinePending || st.slowDeclinePendingFrom != 0 || !st.depthPriorityHeld {
		t.Fatalf("the reset row leaves the ladder watched, not pending and held, got %+v", st)
	}
	readings := []aggragates.AIIndicators{slowDeclineBlock(true), quietLegBlock(false, testutil.At("17:00:00"), fillBarClosed), brokenAtTheBand(), withRecent(brokenAtTheBand(), testutil.At("16:00:00"), testutil.At("09:00:00"))}
	for index, reading := range readings {
		for _, position := range []string{"", "stopLoss"} {
			var next Result
			trade, next = engineTick(trade, position, slowDeclineBand+5, testutil.At("18:15:00").Add(time.Duration(index)*time.Minute), reading)
			assertUntouched(t, next, position)
		}
	}
	if resets := len(trade.Logs) - len(held.Logs); resets != 1 || !strings.Contains(trade.Logs[len(trade.Logs)-1].Message, SlowDeclineResetMarker) {
		t.Fatalf("one reset row over every held tick, got %+v", trade.Logs)
	}

	armed := pendingTrade()
	armed.PositionType, armed.PositionPrice = "stopLoss", 181.2
	assertRow(t, Apply(heldBy(armed, testutil.At("18:05:00")), "", slowDeclineBand, slowDeclineBlock(false)).SlowDecline, resetRow("stopLoss"), slowDeclineLastFill)
	protected := Apply(held, "sell", slowDeclineBand+5, slowDeclineBlock(true))
	assertRow(t, protected.SlowDecline, resetRow("buy"), slowDeclineLastFill)
	assertNoSale(t, protected, "sell")
}

// While held the quiet slow decline and capital protection act on nothing: a
// watched ladder goes pending on no verdict, a pending ladder's new fill is
// neither confirmed nor cancelled — the reset row goes out in place of either
// — and neither band sells, capital protection's on a ladder it alone
// watches included. A pending ladder's take profit reads the move it is
// handed. Each control is the same tick without the gate's row.
func TestAHeldLadderIsNeitherMarkedJudgedNorSold(t *testing.T) {
	withCapitalProtectionExit(t, true)
	assertUntouched(t, Apply(heldBy(watchedTrade(), testutil.At("18:05:00")), "", slowDeclineBand, slowDeclineBlock(true)), "")
	assertForced(t, Apply(watchedTrade(), "", slowDeclineBand, slowDeclineBlock(true)), reasonSellBand)

	for _, tc := range []struct {
		reading aggragates.AIIndicators
		unheld  string
	}{
		{brokenReading(), SlowDeclineCancelMessage("buy", slowDeclineBreakReasons)},
		{legOnAndQuiet(), SlowDeclineMessage("buy", slowDeclineReasons)},
	} {
		assertRow(t, Apply(pendingAtFive(), "", judgeBand, tc.reading).SlowDecline, tc.unheld, sixthFill)
		held := Apply(heldBy(pendingAtFive(), judgeTick), "", judgeBand, tc.reading)
		assertRow(t, held.SlowDecline, resetRow("buy"), sixthFill)
		assertNoSale(t, held, "")
	}

	for _, alone := range []bool{false, true} {
		withQuietSlowDeclineExit(t, !alone)
		withIndecisionDirection(t, !alone)
		assertForced(t, Apply(lastDepthLadder(), "", capitalProtectionBand, withBlock(solBlock())), reasonCapitalProtection)
		assertUntouched(t, Apply(heldBy(lastDepthLadder(), testutil.At("21:45:00")), "", capitalProtectionBand, withBlock(solBlock())), "")
	}
	withQuietSlowDeclineExit(t, true)
	withIndecisionDirection(t, true)

	price := betweenTheTakeProfits(t, pendingTrade())
	if got := TakeProfitPercentage(pendingTrade(), price, breakEvenReading); got != moveAgainst(price, slowDeclineLastFill) {
		t.Fatalf("control: a pending ladder's take profit reads its newest fill, got %v", got)
	}
	if got := TakeProfitPercentage(heldBy(pendingTrade(), testutil.At("18:05:00")), price, breakEvenReading); got != breakEvenReading {
		t.Fatalf("held, the take profit reads the move it is handed, got %v", got)
	}
}

// The ladder's next fill ends the hold, and nothing but the go-pending rule
// makes it pending again: that fill is judged on nothing, a reading that
// does not read for the ladder marks nothing, nor does the verdict with the
// fill outside the fill window; the verdict with the fill inside it marks
// the ladder at that fill, its take profit reads it and the band sells it.
func TestTheNextFillEndsTheHold(t *testing.T) {
	reset, _ := engineTick(heldBy(pendingTrade(), testutil.At("18:05:00")), "", underTheBand, testutil.At("18:10:00"), slowDeclineBlock(true))
	fifth := w3sPrice(watchedFills + 1)
	refilled := withFill(reset, fifth, testutil.At("18:30:00"))
	if st := rebuildState(refilled); st.depthPriorityHeld || st.slowDeclinePending || !st.slowDeclineWatched {
		t.Fatalf("the next fill ends the hold and leaves the ladder watched and not pending, got %+v", st)
	}
	assertNoSlowDeclineRow(t, Apply(refilled, "", slowDeclineBand, brokenAtTheBand()), "")
	assertNoSlowDeclineRow(t, Apply(refilled, "", slowDeclineBand, windowFrom(slowDeclineBlock(true), testutil.At("18:31:00"))), "")

	pending, got := engineTick(refilled, "", underTheBand, testutil.At("18:40:00"), slowDeclineBlock(true))
	assertRow(t, got.SlowDecline, SlowDeclineMessage("buy", slowDeclineReasons), fifth)
	assertForced(t, Apply(pending, "", slowDeclineBand, slowDeclineBlock(false)), reasonSellBand)
	if st := rebuildState(pending); !st.slowDeclinePending || st.slowDeclinePendingFrom != fifth {
		t.Fatalf("pending again from the new fill, got %+v", st)
	}
}

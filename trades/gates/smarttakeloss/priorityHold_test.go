package smarttakeloss

import (
	"strings"
	"testing"
	"time"

	"github.com/giovani-sirbu/mercury/trades/aggragates"
	"github.com/giovani-sirbu/mercury/trades/gates"
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

// withLogRow is the trade carrying one more row of text alone, message and
// stamp given: no event beside it, so no state.
func withLogRow(trade aggragates.Trades, message string, at time.Time) aggragates.Trades {
	row := aggragates.TradesLogs{Message: message, Type: aggragates.LOG_INFO, Price: trade.PositionPrice, CreatedAt: at}
	trade.Logs = append(append([]aggragates.TradesLogs(nil), trade.Logs...), row)
	return trade
}

// heldOnEntryBy is heldBy for the frame the gate holds a first fill in: the
// text row is framed "Hold entry", and the ladder the wallet is kept for is a
// full one, which holds the wallet until it closes.
func heldOnEntryBy(trade aggragates.Trades, at time.Time) aggragates.Trades {
	row := aggragates.TradesLogs{
		Message:   "Hold entry: " + cooldown.DepthPriorityHoldMarker + ", LINK/USDT at depth 8 of 8 holds the wallet until it closes, this ladder waits at depth 4 of 8",
		Type:      aggragates.LOG_INFO,
		Price:     trade.PositionPrice,
		CreatedAt: at,
	}
	event := cooldown.NewDepthPriorityEvent(trade.ID, cooldown.DepthPriorityEvent{
		Event:            gates.EventHeld,
		PrioritySymbol:   "LINK/USDT",
		PriorityDepth:    8,
		PriorityMaxDepth: 8,
		Depth:            4,
		MaxDepth:         8,
	}, at)
	return aggragates.AppendStrategyRow(trade, row, event)
}

// resetMessage is the reset row, byte for byte, framed with the trade's state.
func resetMessage(positionType string) string {
	return "Hold " + positionType + ": smartTakeLoss: quiet slow decline reset, depth priority holds this ladder"
}

// A ladder is held while it carries an event of the cooldown depth priority
// gate stamped strictly after its newest fill, newest in slice order — the
// event as the gate's writers build it, on either frame, and whatever
// follows it. An event at the fill's own stamp or before it, an event
// without a stamp, another gate's event, the gate's name filed under another
// param, a row of the gate's text with no event beside it, a newest fill
// without a stamp and a ladder with no fill hold nothing, and switched off no
// event holds. The event alone holds, with no row beside it.
func TestDepthPriorityHeldReadsTheGatesEventAfterTheNewestFill(t *testing.T) {
	newest := testutil.At("17:38:00")
	after := newest.Add(time.Hour)
	spacing := cooldown.NewDepthSpacingEvent(45211, cooldown.DepthSpacingEvent{Event: gates.EventHeld, Depth: 4, Step: 3, Hold: time.Hour}, after)
	firstFill := cooldown.NewFirstFillEvent(45211, cooldown.FirstFillEvent{Event: cooldown.FirstFillActivated, Price: 190, Reference: 190}, after)
	otherParam := aggragates.NewStrategyEvent(45211, aggragates.StrategyParamSmartTakeLoss, cooldown.GateDepthPriority, cooldown.DepthPriorityEvent{Event: gates.EventHeld}, after)
	gateText := heldBy(testutil.LadderTrade(false), time.Time{}).Logs[0].Message
	gateEvent := heldBy(testutil.LadderTrade(false), after).StrategyEvents[0]
	for name, tc := range map[string]struct {
		trade aggragates.Trades
		held  bool
	}{
		"the gate's event a nanosecond after the fill": {heldBy(watchedTrade(), newest.Add(time.Nanosecond)), true},
		"the gate's entry-frame event after the fill":  {heldOnEntryBy(watchedTrade(), after), true},
		"the gate's event without its row":             {withEvents(watchedTrade(), gateEvent), true},
		"the gate's event, then later events":          {withEvents(withRow(heldBy(watchedTrade(), newest.Add(time.Minute)), ResetRow("buy", slowDeclineLastFill), after), spacing), true},
		"the gate's event at the fill's stamp":         {heldBy(watchedTrade(), newest), false},
		"the gate's event before the fill":             {heldBy(watchedTrade(), newest.Add(-time.Minute)), false},
		"the gate's event without a stamp":             {heldBy(watchedTrade(), time.Time{}), false},
		"depth spacing's event after the fill":         {withEvents(watchedTrade(), spacing), false},
		"the first-fill gate's event after the fill":   {withEvents(watchedTrade(), firstFill), false},
		"a slow-decline event after the fill":          {withRow(watchedTrade(), ResetRow("buy", slowDeclineLastFill), after), false},
		"the gate's name filed under another param":    {withEvents(watchedTrade(), otherParam), false},
		"the gate's row without its event":             {withLogRow(watchedTrade(), gateText, after), false},
		"no fill":                                      {heldBy(testutil.LadderTrade(false), newest), false},
	} {
		if got := rebuildState(tc.trade).depthPriorityHeld; got != tc.held {
			t.Errorf("%s: held %v, want %v", name, got, tc.held)
		}
	}

	unstamped := watchedTrade()
	unstamped.History[watchedFills-1].CreatedAt = time.Time{}
	if rebuildState(heldBy(unstamped, after)).depthPriorityHeld {
		t.Error("a newest fill without a stamp holds nothing")
	}
	outOfOrder := watchedTrade()
	outOfOrder.History[watchedFills-2].CreatedAt, outOfOrder.History[watchedFills-1].CreatedAt = testutil.At("16:00:00"), testutil.At("12:00:00")
	if st := rebuildState(heldBy(outOfOrder, testutil.At("14:00:00"))); !st.depthPriorityHeld || st.lastFill().Price != slowDeclineLastFill {
		t.Errorf("the newest fill is the last in slice order, stamped 12:00, whatever an earlier row's stamp: %+v", st)
	}

	withDepthPriorityPause(t, false)
	if rebuildState(heldBy(watchedTrade(), after)).depthPriorityHeld {
		t.Error("switched off, no event holds the ladder")
	}
}

// The gate's event alone moves no fold: the ladder still reads pending. On its
// first held tick a pending ladder hands back exactly one slow-decline row —
// the reset row, framed with the trade's raw state, at its newest fill's
// price, never the position price — and sells nothing, at the band or past
// it, a close it proposes passing untouched. From the next tick it reads
// watched and not pending, still held, and no slow-decline row is written and
// nothing is sold again, whatever sophos serves; the one reset row is the one
// reset event. A reading that serves the indecision latches the ladder
// besides, which writes the indecision row and its latched event beside the
// reset pair on the same tick.
func TestAHeldPendingLadderIsResetOnceAndNoSlowDeclineRowFollows(t *testing.T) {
	held := heldBy(pendingTrade(), testutil.At("18:05:00"))
	if st := rebuildState(held); !st.slowDeclinePending || !st.depthPriorityHeld {
		t.Fatalf("fixture drifted: the gate's event leaves the ladder pending and holds it, got %+v", st)
	}
	if SlowDeclineResetMarker != "smartTakeLoss: quiet slow decline reset, depth priority holds this ladder" || SlowDeclineResetMessage("buy") != resetMessage("buy") {
		t.Fatalf("the reset marker is byte-stable text and must not move, got %q", SlowDeclineResetMessage("buy"))
	}
	trade, got := engineTick(held, "", slowDeclineBand+1, testutil.At("18:10:00"), slowDeclineBlock(true))
	assertRow(t, got.SlowDecline, resetMessage("buy"), slowDeclineLastFill)
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
	if resets := len(trade.StrategyEvents) - len(held.StrategyEvents); resets != 1 || carriedEvents(trade, GateSlowDecline, EventReset) != 1 {
		t.Fatalf("one reset event over every held tick, got %+v", trade.StrategyEvents)
	}

	armed := pendingTrade()
	armed.PositionType, armed.PositionPrice = "stopLoss", 181.2
	assertRow(t, Apply(heldBy(armed, testutil.At("18:05:00")), "", slowDeclineBand, slowDeclineBlock(false)).SlowDecline, resetMessage("stopLoss"), slowDeclineLastFill)
	protected := Apply(held, "sell", slowDeclineBand+5, slowDeclineBlock(true))
	assertRow(t, protected.SlowDecline, resetMessage("buy"), slowDeclineLastFill)
	assertNoSale(t, protected, "sell")
}

// While held the quiet slow decline and capital protection act on nothing: a
// watched ladder goes pending on no verdict, a pending ladder's new fill is
// neither confirmed nor cancelled — the reset row goes out in place of either
// — and neither band sells, capital protection's on a ladder it alone
// watches included. A pending ladder's take profit reads the move it is
// handed. Each control is the same tick without the gate's event.
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
		assertRow(t, held.SlowDecline, resetMessage("buy"), sixthFill)
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

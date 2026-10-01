package smarttakeloss

import (
	"strings"
	"testing"
	"time"

	"github.com/giovani-sirbu/mercury/trades/aggragates"
	"github.com/giovani-sirbu/mercury/trades/gates/cooldown"
	"github.com/giovani-sirbu/mercury/trades/internal/testutil"
	"github.com/giovani-sirbu/mercury/trades/ladder"
)

// indecisionFills is how deep the ladder the indecision direction watches
// stands in these tests: IndecisionArmDepth fills and never fewer than two,
// so the position price differs from the average entry price and a take
// profit that reads the one is told from a take profit that reads the other.
const indecisionFills = max(IndecisionArmDepth, 2)

// indecisionLadder is the w3s ladder the indecision direction watches from
// its first tick: indecisionFills fills, the newest at 17:38.
func indecisionLadder() aggragates.Trades {
	return testutil.LadderTrade(false, fills(indecisionFills, "17:38:00")...)
}

// overBreakEven is a price a dollar over the ladder's break even, with the
// move against its average entry price there and the move against its
// position price, which a latched ladder reads as well.
func overBreakEven(trade aggragates.Trades) (price, fromAverage, fromPosition float64) {
	price = ladder.AverageEntryPrice(trade) + 1
	return price, moveAgainst(price, ladder.AverageEntryPrice(trade)), moveAgainst(price, trade.PositionPrice)
}

// A watched ladder the depth priority holds is latched on the indecision as
// on any other tick: it gets the row the same ladder gets unheld, framed with
// the trade's state and carrying the newest fill's price — never the
// position price, a re-anchor included — beside the proposal it leaves
// untouched. The row marks and sells nothing. From the next tick the ladder
// reads latched and held, and no second row or event goes out however often
// sophos serves a reading. The indecision direction alone watching the ladder,
// the other two rules switched off, changes none of it.
func TestAHeldWatchedLadderIsLatchedOnTheIndecision(t *testing.T) {
	trade := indecisionLadder()
	newest := rebuildState(trade).lastFill().Price
	assertRow(t, Apply(trade, "", underTheBand, indecisionReading()).Indecision, IndecisionMessage("buy", indecisionReasons), newest)

	held := heldBy(trade, testutil.At("18:00:00"))
	reanchored := held
	reanchored.PositionPrice = newest + 5
	for _, candidate := range []aggragates.Trades{held, reanchored} {
		for _, position := range []string{"", "stopLoss", "takeProfit"} {
			got := Apply(candidate, position, underTheBand, indecisionReading())
			assertRow(t, got.Indecision, IndecisionMessage("buy", indecisionReasons), newest)
			assertNoSale(t, got, position)
			if got.SlowDecline != nil {
				t.Fatalf("%q: the latch tick marks nothing, got %+v", position, got)
			}
		}
	}

	latched, _ := engineTick(held, "", underTheBand, testutil.At("18:10:00"), indecisionReading())
	if st := rebuildState(latched); !st.indecision || !st.indecisionWatched || !st.depthPriorityHeld || st.slowDeclinePending {
		t.Fatalf("the tick after: latched, watched and held, not pending, got %+v", st)
	}
	for index, reading := range []aggragates.AIIndicators{indecisionReading(), slowDeclineBlock(true), brokenAtTheBand(), withBlock(aggragates.SmartTakeLossIndicators{})} {
		for _, position := range []string{"", "stopLoss"} {
			var next Result
			latched, next = engineTick(latched, position, slowDeclineBand+5, testutil.At("18:15:00").Add(time.Duration(index)*time.Minute), reading)
			assertUntouched(t, next, position)
		}
	}
	rows, events := carriedRows(latched, IndecisionMarker), carriedEvents(latched, GateIndecision, EventLatched)
	if rows != 1 || events != 1 {
		t.Fatalf("one indecision row and one latched event over every held tick, got %d and %d in %+v and %+v", rows, events, latched.Logs, latched.StrategyEvents)
	}

	withQuietSlowDeclineExit(t, false)
	withCapitalProtectionExit(t, false)
	alone := Apply(held, "", underTheBand, indecisionReading())
	assertRow(t, alone.Indecision, IndecisionMessage("buy", indecisionReasons), newest)
	assertNoSale(t, alone, "")
}

// A held ladder both pending and served the indecision gets both rows on its
// first held tick — the reset row, then the indecision row that latches it,
// each at the newest fill's price — and sells nothing at the band the same
// ladder unheld sells at, a protected close included. The engines write each
// row with its event, so the trade carries the reset event and the latched
// event after it. From the next tick it reads watched, latched and held, not
// pending, and nothing more is written.
func TestAHeldPendingLadderServedTheIndecisionIsResetAndLatchedOnOneTick(t *testing.T) {
	control := Apply(pendingTrade(), "", slowDeclineBand+1, indecisionReading())
	assertForced(t, control, reasonSellBand)
	assertRow(t, control.Indecision, IndecisionMessage("buy", indecisionReasons), slowDeclineLastFill)

	held := heldBy(pendingTrade(), testutil.At("18:05:00"))
	protected := Apply(held, "sell", slowDeclineBand+5, indecisionReading())
	assertRow(t, protected.SlowDecline, resetMessage("buy"), slowDeclineLastFill)
	assertRow(t, protected.Indecision, IndecisionMessage("buy", indecisionReasons), slowDeclineLastFill)
	assertNoSale(t, protected, "sell")

	trade, got := engineTick(held, "", slowDeclineBand+1, testutil.At("18:10:00"), indecisionReading())
	assertRow(t, got.SlowDecline, resetMessage("buy"), slowDeclineLastFill)
	assertRow(t, got.Indecision, IndecisionMessage("buy", indecisionReasons), slowDeclineLastFill)
	assertNoSale(t, got, "")
	if st := rebuildState(trade); !st.slowDeclineWatched || st.slowDeclinePending || !st.indecision || !st.depthPriorityHeld {
		t.Fatalf("the tick after: watched, not pending, latched and held, got %+v", st)
	}
	assertUntouched(t, Apply(trade, "", slowDeclineBand+1, indecisionReading()), "")
	if rows := len(trade.Logs) - len(held.Logs); rows != 2 || carriedRows(trade, SlowDeclineResetMarker) != 1 || carriedRows(trade, IndecisionMarker) != 1 {
		t.Fatalf("one reset row and one indecision row, got %+v", trade.Logs)
	}
	if written := len(trade.StrategyEvents) - len(held.StrategyEvents); written != 2 || carriedEvents(trade, GateSlowDecline, EventReset) != 1 || carriedEvents(trade, GateIndecision, EventLatched) != 1 {
		t.Fatalf("one reset event and one latched event, got %+v", trade.StrategyEvents)
	}
}

// A latch taken before the hold keeps working through it: the ladder reads
// latched and held, no second row goes out, and its take profit reads the
// move against the position price all the same — the larger of that move and
// the move it is handed, and the move it is handed under break even. The next
// fill ends the hold and keeps the latch, and the take profit reads the new
// fill's position price on the one row and event still.
func TestALatchedLadderTheDepthPriorityHoldsStillReadsItsPositionPrice(t *testing.T) {
	latched := latchedBy(indecisionLadder())
	held := heldBy(latched, testutil.At("22:00:00"))
	if st := rebuildState(held); !st.indecision || !st.depthPriorityHeld {
		t.Fatalf("fixture drifted: latched and held, got %+v", st)
	}
	price, fromAverage, fromPosition := overBreakEven(latched)
	if !(fromAverage < fromPosition) {
		t.Fatal("fixture drifted: the position price's move must exceed the average entry price's")
	}
	for name, trade := range map[string]aggragates.Trades{"unheld": latched, "held": held} {
		if got := TakeProfitPercentage(trade, price, fromAverage); got != fromPosition {
			t.Errorf("%s: the take profit reads the move against the position price %v, got %v", name, fromPosition, got)
		}
		if got := TakeProfitPercentage(trade, price, fromPosition+1); got != fromPosition+1 {
			t.Errorf("%s: an input larger than the position price's move comes back, got %v", name, got)
		}
	}
	under := ladder.AverageEntryPrice(held) - 1
	if input := moveAgainst(under, ladder.AverageEntryPrice(held)); input >= 0 || TakeProfitPercentage(held, under, input) != input {
		t.Fatalf("under break even the input comes back, got %v for %v", TakeProfitPercentage(held, under, input), input)
	}
	assertUntouched(t, Apply(held, "", underTheBand, indecisionReading()), "")

	resumed := withFill(held, w3sPrice(indecisionFills+1), testutil.At("22:30:00"))
	if st := rebuildState(resumed); !st.indecision || st.depthPriorityHeld {
		t.Fatalf("the next fill ends the hold and keeps the latch, got %+v", st)
	}
	price, fromAverage, fromPosition = overBreakEven(resumed)
	if got := TakeProfitPercentage(resumed, price, fromAverage); got != fromPosition || !(fromAverage < fromPosition) {
		t.Fatalf("resumed, the take profit reads the new fill %v, got %v", fromPosition, got)
	}
	if got := Apply(resumed, "", underTheBand, indecisionReading()); got.Indecision != nil || carriedRows(resumed, IndecisionMarker) != 1 || carriedEvents(resumed, GateIndecision, EventLatched) != 1 {
		t.Fatalf("one indecision row and event, before the hold and after it, got %+v on %+v and %+v", got, resumed.Logs, resumed.StrategyEvents)
	}
}

// carriedRows counts the trade's rows naming marker: the text the operator
// reads, twin of carriedEvents.
func carriedRows(trade aggragates.Trades, marker string) int {
	count := 0
	for _, logged := range trade.Logs {
		if strings.Contains(logged.Message, marker) {
			count++
		}
	}
	return count
}

// carriedEvents counts the trade's smartTakeLoss events of gate and kind: one
// for every row the engines wrote for it, twin of carriedRows.
func carriedEvents(trade aggragates.Trades, gate, kind string) int {
	count := 0
	for _, event := range trade.StrategyEventsOf(aggragates.StrategyParamSmartTakeLoss, gate) {
		if event.Kind() == kind {
			count++
		}
	}
	return count
}

// Switched off, the gate's events are ignored by every rule: a held pending
// ladder sells at the band with no reset row, a held watched ladder goes
// pending on the verdict and is latched on the indecision, a held pending
// ladder served the indecision is latched beside the sale and gets no reset
// row, and a held latched one reads its position price, as a held pending one
// reads its newest fill.
func TestSwitchedOffTheDepthPriorityRowsAreIgnored(t *testing.T) {
	withDepthPriorityPause(t, false)
	held := heldBy(pendingTrade(), testutil.At("18:05:00"))
	got := Apply(held, "", slowDeclineBand, slowDeclineBlock(true))
	assertForced(t, got, reasonSellBand)
	if got.SlowDecline != nil {
		t.Fatalf("switched off, no reset row, got %+v", got)
	}
	watched := heldBy(watchedTrade(), testutil.At("18:05:00"))
	assertRow(t, Apply(watched, "", underTheBand, slowDeclineBlock(true)).SlowDecline, SlowDeclineMessage("buy", slowDeclineReasons), slowDeclineLastFill)
	assertRow(t, Apply(heldBy(indecisionLadder(), testutil.At("18:00:00")), "", underTheBand, indecisionReading()).Indecision, IndecisionMessage("buy", indecisionReasons), indecisionLadder().PositionPrice)
	pendingServed := Apply(held, "", slowDeclineBand, indecisionReading())
	assertForced(t, pendingServed, reasonSellBand)
	assertRow(t, pendingServed.Indecision, IndecisionMessage("buy", indecisionReasons), slowDeclineLastFill)
	if pendingServed.SlowDecline != nil {
		t.Fatalf("switched off, a held pending ladder served the indecision gets no reset row, got %+v", pendingServed)
	}
	price := betweenTheTakeProfits(t, held)
	if got := TakeProfitPercentage(held, price, breakEvenReading); got != moveAgainst(price, slowDeclineLastFill) {
		t.Fatalf("switched off, the take profit reads the newest fill, got %v", got)
	}
	latched := heldBy(latchedBy(indecisionLadder()), testutil.At("22:00:00"))
	price, fromAverage, fromPosition := overBreakEven(latched)
	if TakeProfitPercentage(latched, price, fromAverage) != fromPosition {
		t.Fatal("switched off, a held latched ladder reads its position price")
	}
}

// No row is taken for another by the filters that find a row by its text (cp,
// the notification filter). No marker the smart take loss writes, nor its
// first-fill hold reason, contains another or the depth priority marker, or
// is contained in either; no framed smart take loss row names the depth
// priority marker, and the gate's framed row names no smart take loss marker.
func TestNoRowIsReadAsAnother(t *testing.T) {
	texts := []string{SlowDeclineMarker, SlowDeclineCancelMarker, SlowDeclineResetMarker, IndecisionMarker, SlowPatternMarker, SlowPatternCancelMarker, SlowDeclineEntryHoldReason, cooldown.DepthPriorityHoldMarker}
	for i, one := range texts {
		for j, other := range texts {
			if i != j && strings.Contains(one, other) {
				t.Errorf("%q contains %q", one, other)
			}
		}
	}
	latchedByThePattern := IndecisionMessage("stopLoss", append([]string{slowPatternLatchReason}, slowPatternReasons...))
	for _, framed := range []string{SlowDeclineMessage("stopLoss", slowDeclineReasons), SlowDeclineCancelMessage("stopLoss", slowDeclineBreakReasons), SlowDeclineResetMessage("stopLoss"), IndecisionMessage("stopLoss", indecisionReasons),
		SlowPatternMessage("stopLoss", slowPatternReasons), SlowPatternCancelMessage("stopLoss", slowPatternReasons), latchedByThePattern} {
		if strings.Contains(framed, cooldown.DepthPriorityHoldMarker) {
			t.Errorf("%q names the depth priority marker", framed)
		}
	}
	for _, marker := range []string{SlowPatternMarker, SlowPatternCancelMarker} {
		if strings.Contains(latchedByThePattern, marker) {
			t.Errorf("the row that latches on the slow pattern names %q and would read as the pattern's own", marker)
		}
	}
	gate := heldBy(watchedTrade(), testutil.At("18:05:00")).Logs[0].Message
	for _, marker := range texts[:len(texts)-1] {
		if strings.Contains(gate, marker) {
			t.Errorf("the gate's row %q names %q", gate, marker)
		}
	}
}

// A depth priority hold pauses the slow pattern decline as it does the quiet
// rule, and never resets it: a pending pattern stays pending through the hold
// with no row and no sale at the band — the unheld control sells — a ladder not
// yet pending is not read, and the ladder's next fill, which ends the hold, is
// judged on the series that has closed its bar.
func TestAHeldLadderIsNotReadBySlowPatternAndItsPendingSurvives(t *testing.T) {
	pending := stairPending(t)
	held := heldBy(pending, stairOpen(25))
	if st := rebuildState(held); !st.slowPatternPending || !st.indecision || !st.depthPriorityHeld {
		t.Fatalf("fixture drifted: pending, latched and held, got %+v", st)
	}
	block := withBlock(stairBlock(stairTurns, 24))
	assertForced(t, Apply(pending, "", slowDeclineBand+1, block), reasonSlowPatternBand)
	for _, price := range []float64{slowDeclineBand - 1, slowDeclineBand + 1} {
		assertUntouched(t, Apply(held, "", price, block), "")
	}
	assertUntouched(t, Apply(heldBy(stairLadder(0, 9, 15, 24), stairOpen(25)), "", slowDeclineBand-1, block), "")
	if st := rebuildState(held); !st.slowPatternPending || st.slowPatternPendingFrom != stairPrices[3] {
		t.Fatalf("no row resets the pattern, it is pending from where it was, got %+v", st)
	}

	refilled := withFill(held, stairPrices[4], stairOpen(30).Add(time.Minute))
	if st := rebuildState(refilled); st.depthPriorityHeld || !st.slowPatternPending {
		t.Fatalf("the next fill ends the hold and the pattern is still pending, got %+v", st)
	}
	assertRow(t, Apply(refilled, "", slowDeclineBand-1, withBlock(stairBlock(stairTurns, 30))).SlowDecline, SlowPatternMessage("buy", patternReasonsAt30), stairPrices[4])
}

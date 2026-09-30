package smarttakeloss

import (
	"strings"
	"testing"

	"github.com/giovani-sirbu/mercury/trades/aggragates"
	"github.com/giovani-sirbu/mercury/trades/gates/cooldown"
	"github.com/giovani-sirbu/mercury/trades/internal/testutil"
	"github.com/giovani-sirbu/mercury/trades/ladder"
)

// indecisionLadder is the w3s ladder the indecision direction watches from
// its first tick: IndecisionArmDepth fills, the newest at 17:38.
func indecisionLadder() aggragates.Trades {
	return testutil.LadderTrade(false, fills(IndecisionArmDepth, "17:38:00")...)
}

// overBreakEven is a price a dollar over the ladder's break even, with the
// move against its average entry price there and the move against its
// position price, which a latched ladder reads as well.
func overBreakEven(trade aggragates.Trades) (price, fromAverage, fromPosition float64) {
	price = ladder.AverageEntryPrice(trade) + 1
	return price, moveAgainst(price, ladder.AverageEntryPrice(trade)), moveAgainst(price, trade.PositionPrice)
}

// No latch starts while the depth priority holds a ladder: the indecision
// reading latches the watched ladder without the gate's event, and with it no
// proposal gets a row and the ladder reads unlatched. A pending ladder the
// indecision reaches on its first held tick gets the reset row alone.
func TestNoLatchStartsWhileTheDepthPriorityHolds(t *testing.T) {
	trade := indecisionLadder()
	assertRow(t, Apply(trade, "", underTheBand, indecisionReading()).Indecision, IndecisionMessage("buy", indecisionReasons), trade.PositionPrice)
	held := heldBy(trade, testutil.At("18:00:00"))
	for _, position := range []string{"", "stopLoss", "takeProfit"} {
		assertUntouched(t, Apply(held, position, underTheBand, indecisionReading()), position)
	}
	if st := rebuildState(held); st.indecision || !st.indecisionWatched {
		t.Fatalf("a held ladder stays watched and unlatched, got %+v", st)
	}

	control := Apply(pendingTrade(), "", underTheBand, indecisionReading())
	assertRow(t, control.Indecision, IndecisionMessage("buy", indecisionReasons), slowDeclineLastFill)
	got := Apply(heldBy(pendingTrade(), testutil.At("18:05:00")), "", underTheBand, indecisionReading())
	assertRow(t, got.SlowDecline, resetMessage("buy"), slowDeclineLastFill)
	if got.Indecision != nil {
		t.Fatalf("the held tick latches nothing, got %+v", got)
	}
}

// A latch taken before the hold keeps its event: the ladder reads latched and
// held, its take profit reads the move it is handed instead of the position
// price's, and no second row goes out. The next fill ends the hold and the
// latch's effects resume from it: the take profit reads the position price —
// the new fill — on the one row and event still.
func TestALatchTakenBeforeTheHoldResumesAfterTheNextFill(t *testing.T) {
	latched := latchedBy(indecisionLadder())
	held := heldBy(latched, testutil.At("22:00:00"))
	if st := rebuildState(held); !st.indecision || !st.depthPriorityHeld {
		t.Fatalf("fixture drifted: latched and held, got %+v", st)
	}
	price, fromAverage, fromPosition := overBreakEven(latched)
	if !(fromAverage < fromPosition) || TakeProfitPercentage(latched, price, fromAverage) != fromPosition {
		t.Fatal("control: a latched ladder's take profit reads its position price")
	}
	if got := TakeProfitPercentage(held, price, fromAverage); got != fromAverage {
		t.Fatalf("held, the take profit reads the move it is handed %v, got %v", fromAverage, got)
	}
	assertUntouched(t, Apply(held, "", underTheBand, indecisionReading()), "")

	resumed := withFill(held, w3sPrice(IndecisionArmDepth+1), testutil.At("22:30:00"))
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
// pending on the verdict and is latched on the indecision, and a held latched
// one reads its position price, as a held pending one reads its newest fill.
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
	texts := []string{SlowDeclineMarker, SlowDeclineCancelMarker, SlowDeclineResetMarker, IndecisionMarker, SlowDeclineEntryHoldReason, cooldown.DepthPriorityHoldMarker}
	for i, one := range texts {
		for j, other := range texts {
			if i != j && strings.Contains(one, other) {
				t.Errorf("%q contains %q", one, other)
			}
		}
	}
	for _, framed := range []string{SlowDeclineMessage("stopLoss", slowDeclineReasons), SlowDeclineCancelMessage("stopLoss", slowDeclineBreakReasons), SlowDeclineResetMessage("stopLoss"), IndecisionMessage("stopLoss", indecisionReasons)} {
		if strings.Contains(framed, cooldown.DepthPriorityHoldMarker) {
			t.Errorf("%q names the depth priority marker", framed)
		}
	}
	gate := heldBy(watchedTrade(), testutil.At("18:05:00")).Logs[0].Message
	for _, marker := range texts[:5] {
		if strings.Contains(gate, marker) {
			t.Errorf("the gate's row %q names %q", gate, marker)
		}
	}
}

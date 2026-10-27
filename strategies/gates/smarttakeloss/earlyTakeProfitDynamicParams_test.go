package smarttakeloss

import (
	"strings"
	"testing"
	"time"

	"github.com/giovani-sirbu/mercury/internal/testutil"
	"github.com/giovani-sirbu/mercury/strategies/gates/dynamicparams"
	"github.com/giovani-sirbu/mercury/trades/aggragates"
)

// withEarlyTakeProfitDynamicParams is withIndecision for the latch on a ladder that opened raised.
func withEarlyTakeProfitDynamicParams(t *testing.T, on bool) {
	t.Helper()
	previous := earlyTakeProfitDynamicParams
	earlyTakeProfitDynamicParams = on
	t.Cleanup(func() { earlyTakeProfitDynamicParams = previous })
}

// The amounts the raised fixtures open with.
const (
	latchRaisePoints = 0.5
	latchRaiseDepths = 1
)

// raiseReasons is what a latch row names of a raise by those amounts.
var raiseReasons = []string{"opened raised by percentage +0.5, depths +1"}

// openedRaisedBy is the ladder as the engine leaves it once it opened raised by these amounts: the
// DynamicParams flag on for a long spot parent and the opened pair the engine's own writer appends.
func openedRaisedBy(trade aggragates.Trades, points float64, depths int) aggragates.Trades {
	trade.Strategy.Params.DynamicParams = true
	trade.Strategy.TradeType = aggragates.Spot
	row, event := dynamicparams.Opened{Points: points, Depths: depths}.Rows(trade, trade.PositionPrice, time.Time{})
	return aggragates.AppendStrategyRow(trade, row, event)
}

// raisedIndecisionLadder is indecisionLadder opened raised by the fixtures' amounts.
func raisedIndecisionLadder() aggragates.Trades {
	return openedRaisedBy(indecisionLadder(), latchRaisePoints, latchRaiseDepths)
}

// A watched ladder that opened raised is latched on the raise alone: one row at the newest fill's
// price naming the amounts, the proposal untouched, held by a depth priority or not; sophos'
// reading, when served, names the row instead. From the next tick it reads latched, writes nothing
// more, and its take profit reads its position price. The row carries neither the opened row's
// prefix nor its marker, so agora notifies it like every latch.
func TestARaisedLadderLatchesTheEarlyTakeProfit(t *testing.T) {
	withEarlyTakeProfit(t, true)
	withEarlyTakeProfitDynamicParams(t, true)
	assertUntouched(t, Apply(indecisionLadder(), "", underTheBand, noReading()), "")

	raised := raisedIndecisionLadder()
	newest := rebuildState(raised).lastFill().Price
	want := LatchedRow(newest, Finding{Feature: FeatureDynamicParams, Reasons: raiseReasons})
	if want.Message != "smartTakeLoss: early take profit latched (dynamic params: opened raised by percentage +0.5, depths +1)" {
		t.Fatalf("the row names the raise, got %q", want.Message)
	}
	for _, other := range []string{dynamicparams.RowPrefix, dynamicparams.OpenedMarker} {
		if strings.Contains(want.Message, other) {
			t.Fatalf("%q carries the opened row's %q", want.Message, other)
		}
	}
	for _, position := range []string{"", "stopLoss", "takeProfit"} {
		got := Apply(raised, position, underTheBand, noReading())
		assertRow(t, got, want)
		assertNoSale(t, got, position)
	}
	held := heldBy(raised, testutil.At("18:00:00"))
	if !rebuildState(held).depthPriorityHeld {
		t.Fatal("fixture drifted: the hold must pause the gates")
	}
	assertRow(t, Apply(held, "", underTheBand, noReading()), want)
	assertRow(t, Apply(raised, "", underTheBand, indecisionReading()), LatchedRow(newest, indecisionFinding(indecisionReasons)))

	latched, _ := engineTick(raised, "", underTheBand, testutil.At("18:10:00"), noReading())
	if st := rebuildState(latched); !st.earlyTakeProfit {
		t.Fatalf("the tick after: latched, got %+v", st)
	}
	assertUntouched(t, Apply(latched, "", underTheBand, noReading()), "")
	if carriedLatches(latched) != 1 {
		t.Fatalf("one event carries the latch, got %+v", latched.StrategyEvents)
	}
	price, fromAverage, fromPosition := overBreakEven(latched)
	if got := TakeProfitPercentage(raised, price, fromAverage); got != fromAverage {
		t.Fatalf("before the latch the take profit reads the average entry %v, got %v", fromAverage, got)
	}
	if got := TakeProfitPercentage(latched, price, fromAverage); got != fromPosition {
		t.Fatalf("latched, the take profit reads the position price %v, got %v", fromPosition, got)
	}
}

// On a tick the gate writes its own row the latch lands beside it: a raised watched ladder going
// pending on the quiet decline gets the one pending row, the raise's amounts under the indecision,
// and reads latched and pending from the next tick.
func TestARaisedLadderIsLatchedBesideTheGatesOwnRow(t *testing.T) {
	withQuietDecline(t, true)
	withEarlyTakeProfit(t, true)
	withEarlyTakeProfitDynamicParams(t, true)
	raised := openedRaisedBy(watchedTrade(), latchRaisePoints, latchRaiseDepths)
	want := PendingRow(slowDeclineLastFill, latchMarker, Finding{Feature: FeatureDynamicParams, Reasons: raiseReasons}, quietFinding(slowDeclineReasons))
	if want.Message != "smartTakeLoss: early take profit latched, quiet decline triggered (dynamic params: opened raised by percentage +0.5, depths +1; quiet decline: leg down 7.0% from its high close, leg 60 bars long on 1h)" {
		t.Fatalf("the pending row names the latch and the raise, got %q", want.Message)
	}
	trade, got := tickAtAGain(raised, "", testutil.At("18:10:00"), slowDeclineBlock(true))
	assertRow(t, got, want)
	assertNoSale(t, got, "")
	if st := rebuildState(trade); !st.earlyTakeProfit || !st.slowDeclinePending {
		t.Fatalf("the tick after: latched and pending, got %+v", st)
	}
}

// Nothing else latches on the raise: the switch off, the indecision off, the flag off with the
// opened pair still on the trade, an opened pair that raises nothing, a ladder under the arm depth
// and one already latched.
func TestTheDynamicParamsLatchNeedsARaisedWatchedLadder(t *testing.T) {
	withEarlyTakeProfit(t, true)
	withEarlyTakeProfitDynamicParams(t, true)
	raised := raisedIndecisionLadder()
	flagOff := raised
	flagOff.Strategy.Params.DynamicParams = false
	cases := map[string]aggragates.Trades{
		"the flag off":           flagOff,
		"a pair raising nothing": openedRaisedBy(indecisionLadder(), 0, 0),
		"under the arm depth":    openedRaisedBy(testutil.LadderTrade(false, fills(1, "17:38:00")...), latchRaisePoints, latchRaiseDepths),
		"a latched ladder":       latchedBy(raised),
	}
	for name, candidate := range cases {
		t.Run(name, func(t *testing.T) {
			assertUntouched(t, Apply(candidate, "", underTheBand, noReading()), "")
		})
	}
	t.Run("the switch off", func(t *testing.T) {
		withEarlyTakeProfitDynamicParams(t, false)
		assertUntouched(t, Apply(raised, "", underTheBand, noReading()), "")
	})
	t.Run("the indecision off", func(t *testing.T) {
		withEarlyTakeProfit(t, false)
		assertUntouched(t, Apply(raised, "", underTheBand, noReading()), "")
	})
}

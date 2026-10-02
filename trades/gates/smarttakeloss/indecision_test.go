package smarttakeloss

import (
	"reflect"
	"slices"
	"testing"
	"time"

	"github.com/giovani-sirbu/mercury/trades/aggragates"
	"github.com/giovani-sirbu/mercury/trades/internal/testutil"
)

// latchNamed is whether a row Apply handed back names the indecision latch: the
// indecision direction among the features of the row's event.
func latchNamed(got Result) bool {
	for _, row := range got.Rows {
		if slices.Contains(row.Features, FeatureIndecision) {
			return true
		}
	}
	return false
}

// The event carries the newest fill's price, and rebuildState folds an event
// only with a price, so the rule watches no ladder before its first fill.
func TestIndecisionArmDepthNeedsAFill(t *testing.T) {
	if IndecisionArmDepth < 1 {
		t.Fatalf("IndecisionArmDepth %d must hold at least one fill", IndecisionArmDepth)
	}
}

// A long spot parent ladder from IndecisionArmDepth fills gets the row on the
// tick sophos serves the indecision on — the latch naming what broke, at the
// newest fill's price — beside the proposal it leaves untouched: an add,
// nothing, or a close the ladder proposes or the trade rests in. It sells
// nothing. An impasse strategy's parent is watched too.
func TestApplyLatchesAWatchedLadderOnTheIndecision(t *testing.T) {
	trade := testutil.LadderTrade(false, fills(IndecisionArmDepth, "17:38:00")...)
	want := LatchedRow(trade.PositionPrice, indecisionFinding(indecisionReasons))
	for _, position := range []string{"", "stopLoss", "buy", "takeProfit", "sell"} {
		got := Apply(trade, position, slowDeclineBand+1, indecisionReading())
		if got.Position != position || got.Reason != "" {
			t.Fatalf("%q: the proposal must pass untouched with no sale, got %+v", position, got)
		}
		assertRow(t, got, want)
	}
	resting := trade
	resting.PositionType = "takeProfit"
	assertRow(t, Apply(resting, "", slowDeclineBand, indecisionReading()), want)
	impasse := trade
	impasse.Strategy.Params.Impasse = true
	assertRow(t, Apply(impasse, "", slowDeclineBand, indecisionReading()), want)
}

// The row goes out once: from the tick after it the ladder reads latched, and
// a latched ladder gets no second row however often sophos serves the
// reading again, nor after a new fill. Nothing takes the latch away: not a
// reading without the indecision, a broken one, the verdict, the zero block,
// nor the cancelled row a new fill gets.
func TestApplyWritesTheIndecisionRowOnce(t *testing.T) {
	trade := testutil.LadderTrade(false, fills(IndecisionArmDepth, "17:38:00")...)
	newest := trade.PositionPrice
	trade, got := engineTick(trade, "", underTheBand, testutil.At("18:00:00"), indecisionReading())
	assertRow(t, got, LatchedRow(newest, indecisionFinding(indecisionReasons)))
	for index, reading := range []aggragates.AIIndicators{indecisionReading(), brokenReading(), withBlock(aggragates.SmartTakeLossIndicators{}), verdictReading(), indecisionReading()} {
		var next Result
		trade, next = engineTick(trade, "", underTheBand, testutil.At("18:15:00").Add(time.Duration(index)*time.Minute), reading)
		if latchNamed(next) {
			t.Fatalf("reading %d: a latched ladder gets no second row, got %+v", index, next)
		}
		if !rebuildState(trade).indecision {
			t.Fatalf("reading %d: nothing takes the latch away", index)
		}
	}
	w3s := fills(IndecisionArmDepth+1, "19:00:00")
	trade = withFill(trade, w3s[IndecisionArmDepth].Price, testutil.At("19:00:00"))
	trade, got = engineTick(trade, "", underTheBand, testutil.At("19:15:00"), indecisionReading())
	if len(got.Rows) != 1 || got.Rows[0].Event != EventCancelled || latchNamed(got) || !rebuildState(trade).indecision {
		t.Fatalf("a new fill is judged on the reading and neither unlatches the ladder nor writes a second latch, got %+v", got)
	}
	rows, latches := carriedRows(trade, indecisionLabel), carriedLatches(trade)
	if rows != 1 || latches != 1 {
		t.Fatalf("one row naming the latch and one event carrying it over every tick, got %d and %d in %+v and %+v", rows, latches, trade.Logs, trade.StrategyEvents)
	}
}

// A ladder the rule does not watch gets no latch on the indecision: one short
// of IndecisionArmDepth, with no fill, inverse, futures, a child, a strategy
// without the flag, and every ladder while Indecision is off. A watched ladder
// gets none on a reading without the indecision, whatever else it serves.
func TestApplyWritesNoIndecisionRowOnALadderItDoesNotWatch(t *testing.T) {
	futures := watchedTrade()
	futures.Strategy.TradeType = aggragates.Futures
	child := watchedTrade()
	child.ParentID = 7
	off := watchedTrade()
	off.Strategy.Params.SmartTakeLoss = false
	for name, trade := range map[string]aggragates.Trades{
		"one short of IndecisionArmDepth": testutil.LadderTrade(false, fills(IndecisionArmDepth-1, "17:38:00")...),
		"with no fill":                    testutil.LadderTrade(false),
		"inverse":                         testutil.LadderTrade(true, fills(watchedFills, "17:38:00")...),
		"futures":                         futures,
		"a child":                         child,
		"without the flag":                off,
	} {
		if got := Apply(trade, "", underTheBand, indecisionReading()); latchNamed(got) {
			t.Errorf("%s: got %+v", name, got)
		}
	}
	for _, reading := range []aggragates.AIIndicators{brokenReading(), legOnAndQuiet(), verdictReading(), withBlock(aggragates.SmartTakeLossIndicators{SlowDeclineBreakReasons: indecisionReasons})} {
		if got := Apply(watchedTrade(), "", underTheBand, reading); latchNamed(got) {
			t.Errorf("a reading without the indecision writes no latch, got %+v", got)
		}
	}
	withIndecision(t, false)
	if got := Apply(watchedTrade(), "", underTheBand, indecisionReading()); latchNamed(got) {
		t.Fatalf("switched off, no latch, got %+v", got)
	}
}

// A pending ladder whose new fill is judged on the indecision reading gets ONE
// row on that tick: the cancelled row, since the leg is not on and quiet, with
// the latch named in its head and the reasons — the same list for the latch and
// for what broke — printed once under both labels, carrying the new fill's
// price. From the next tick it is latched and not pending, and its band sells
// nothing.
func TestApplyCancelsAndLatchesOnOneRow(t *testing.T) {
	trade, got := engineTick(pendingAtFive(), "", underJudgeBand, judgeTick, indecisionReading())
	want := CancelledRow(sixthFill, indecisionFinding(indecisionReasons), quietFinding(indecisionReasons))
	assertRow(t, got, want)
	if len(want.Features) != 2 || want.Event != EventCancelled || !slices.Equal(want.Reasons, indecisionReasons) {
		t.Fatalf("one cancelled event naming the latch and the quiet decline, the shared reasons once, got %+v", want)
	}
	if st := rebuildState(trade); st.slowDeclinePending || !st.indecision {
		t.Fatalf("the tick after: not pending and latched, got %+v", st)
	}
	assertNoSale(t, Apply(trade, "", slowDeclineBand, indecisionReading()), "")
}

// The depth priority hold pauses the slow decline gate and capital
// protection, never the sophos reading of the indecision: a held watched ladder
// gets the row the same ladder gets unheld — the row and the event it files,
// which the Row carries — and a held latched ladder's take profit reads the move
// against its position price as the unheld one does.
func TestADepthPriorityHoldLeavesTheIndecisionDirectionOn(t *testing.T) {
	trade := indecisionLadder()
	unheld := Apply(trade, "", underTheBand, indecisionReading())
	assertRow(t, unheld, LatchedRow(trade.PositionPrice, indecisionFinding(indecisionReasons)))
	held := Apply(heldBy(trade, testutil.At("18:00:00")), "", underTheBand, indecisionReading())
	if len(held.Rows) != 1 || !reflect.DeepEqual(held.Rows[0], unheld.Rows[0]) || held.Position != "" || held.Reason != "" {
		t.Fatalf("held, the ladder gets the unheld row and nothing else, got %+v want %+v", held, unheld)
	}

	latched := latchedBy(trade)
	heldLatched := heldBy(latched, testutil.At("22:00:00"))
	if st := rebuildState(heldLatched); !st.indecision || !st.depthPriorityHeld {
		t.Fatalf("fixture drifted: latched and held, got %+v", st)
	}
	price, fromAverage, fromPosition := overBreakEven(latched)
	if !(fromAverage < fromPosition) {
		t.Fatal("fixture drifted: the position price's move must exceed the average entry price's")
	}
	if got := TakeProfitPercentage(latched, price, fromAverage); got != fromPosition {
		t.Fatalf("control: a latched ladder's take profit reads its position price %v, got %v", fromPosition, got)
	}
	if got := TakeProfitPercentage(heldLatched, price, fromAverage); got != fromPosition {
		t.Fatalf("held, the take profit reads the position price %v, got %v", fromPosition, got)
	}
}

// The swing decline latches a ladder the indecision direction watches the tick
// it goes pending, once: the row that makes the ladder pending names the latch
// beside the swing decline, with no reasons of its own. A ladder already
// latched, and one the sophos reading latches on that very tick, get no second
// latch — the second names its own reasons and the swing decline's both; one the
// indecision direction does not watch goes pending without a latch.
func TestTheSwingDeclineLatchesOnceAndOnlyWhereTheIndecisionWatches(t *testing.T) {
	block := withBlock(stairFourthBlock())
	reasons := stairReading(t, stairFourth(), stairFourthBlock(), true).Reasons

	assertRow(t, Apply(stairFourth(), "", underTheBand, block), PendingRow(stairPrices[3], indecisionFinding(nil), swingFinding(reasons)))

	got := Apply(latchedBy(stairFourth()), "", underTheBand, block)
	assertRow(t, got, PendingRow(stairPrices[3], swingFinding(reasons)))

	reading := stairFourthBlock()
	reading.SlowDeclineIndecision, reading.SlowDeclineBreakReasons = true, indecisionReasons
	got = Apply(stairFourth(), "", underTheBand, withBlock(reading))
	assertRow(t, got, PendingRow(stairPrices[3], indecisionFinding(indecisionReasons), swingFinding(reasons)))

	withIndecision(t, false)
	assertRow(t, Apply(stairFourth(), "", underTheBand, block), PendingRow(stairPrices[3], swingFinding(reasons)))
}

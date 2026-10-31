package smartsell

import (
	"reflect"
	"slices"
	"testing"
	"time"

	"github.com/giovani-sirbu/mercury/internal/testutil"
	"github.com/giovani-sirbu/mercury/strategies/gates/smartsell/earlytakeprofit"
	"github.com/giovani-sirbu/mercury/strategies/gates/smartsell/flatmarket"
	"github.com/giovani-sirbu/mercury/strategies/gates/smartsell/lineardecline"
	"github.com/giovani-sirbu/mercury/strategies/gates/smartsell/quietdecline"
	"github.com/giovani-sirbu/mercury/strategies/gates/smartsell/swingdecline"
	"github.com/giovani-sirbu/mercury/trades/aggragates"
)

// latchNamed is whether a row Apply handed back names the indecision latch: the
// early take profit among the features of the row's event.
func latchNamed(got Result) bool {
	for _, row := range got.Rows {
		if slices.Contains(row.Features, FeatureEarlyTakeProfit) {
			return true
		}
	}
	return false
}

// The declines' and the flat market's rows carry the newest fill's price, which rebuildState folds
// their kinds by, so those gates watch no ladder before its first fill; the latch alone folds
// without a price (TestAnUnfilledLadderIsLatchedOnce), so earlytakeprofit.ArmDepth may be zero.
func TestGatesPricedAtTheNewestFillWatchAFilledLadder(t *testing.T) {
	for name, armDepth := range map[string]int{
		"quietdecline.ArmDepth":  quietdecline.ArmDepth,
		"swingdecline.ArmDepth":  swingdecline.ArmDepth,
		"lineardecline.ArmDepth": lineardecline.ArmDepth,
		"flatmarket.ArmDepth":    flatmarket.ArmDepth,
	} {
		if armDepth < 1 {
			t.Errorf("%s %d must hold at least one fill", name, armDepth)
		}
	}
}

// A long spot parent ladder from earlytakeprofit.ArmDepth fills gets the
// row on the tick sophos serves the indecision on — the latch naming what
// broke, at the newest fill's price — beside the proposal it leaves
// untouched: an add, nothing, or a close the ladder proposes or the trade
// rests in. It sells nothing. An impasse strategy's parent is watched too.
func TestApplyLatchesAWatchedLadderOnTheEarlyTakeProfit(t *testing.T) {
	trade := testutil.LadderTrade(false, fills(earlytakeprofit.ArmDepth, "17:38:00")...)
	want := LatchedRow(trade.PositionPrice, indecisionFinding(indecisionReasons))
	for _, position := range []string{"", "trailingBuy", "buy", "takeProfit", "sell"} {
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
func TestApplyWritesTheEarlyTakeProfitRowOnce(t *testing.T) {
	trade := testutil.LadderTrade(false, fills(earlytakeprofit.ArmDepth, "17:38:00")...)
	newest := trade.PositionPrice
	trade, got := engineTick(trade, "", underTheBand, testutil.At("18:00:00"), indecisionReading())
	assertRow(t, got, LatchedRow(newest, indecisionFinding(indecisionReasons)))
	for index, reading := range []aggragates.AIIndicators{indecisionReading(), brokenReading(), withBlock(aggragates.SmartSellIndicators{}), verdictReading(), indecisionReading()} {
		var next Result
		trade, next = engineTick(trade, "", underTheBand, testutil.At("18:15:00").Add(time.Duration(index)*time.Minute), reading)
		if latchNamed(next) {
			t.Fatalf("reading %d: a latched ladder gets no second row, got %+v", index, next)
		}
		if !rebuildState(trade).earlyTakeProfit {
			t.Fatalf("reading %d: nothing takes the latch away", index)
		}
	}
	w3s := fills(earlytakeprofit.ArmDepth+1, "19:00:00")
	trade = withFill(trade, w3s[earlytakeprofit.ArmDepth].Price, testutil.At("19:00:00"))
	trade, got = engineTick(trade, "", underTheBand, testutil.At("19:15:00"), indecisionReading())
	if len(got.Rows) != 1 || got.Rows[0].Event != EventCancelled || latchNamed(got) || !rebuildState(trade).earlyTakeProfit {
		t.Fatalf("a new fill is judged on the reading and neither unlatches the ladder nor writes a second latch, got %+v", got)
	}
	rows, latches := carriedRows(trade, indecisionLabel), carriedLatches(trade)
	if rows != 1 || latches != 1 {
		t.Fatalf("one row naming the latch and one event carrying it over every tick, got %d and %d in %+v and %+v", rows, latches, trade.Logs, trade.StrategyEvents)
	}
}

// A ladder the rule does not watch gets no latch on the indecision: one short
// of earlytakeprofit.ArmDepth, with no fill, inverse, futures, a child, a
// strategy without the flag, and every ladder while earlytakeprofit.Enabled is
// off. A watched ladder gets none on a reading without the indecision,
// whatever else it serves.
func TestApplyWritesNoEarlyTakeProfitRowOnALadderItDoesNotWatch(t *testing.T) {
	futures := watchedTrade()
	futures.Strategy.TradeType = aggragates.Futures
	child := watchedTrade()
	child.ParentID = 7
	off := watchedTrade()
	off.Strategy.Params.SmartSell = false
	for name, trade := range map[string]aggragates.Trades{
		"one short of earlytakeprofit.ArmDepth": testutil.LadderTrade(false, fills(earlytakeprofit.ArmDepth-1, "17:38:00")...),
		"with no fill":                          testutil.LadderTrade(false),
		"inverse":                               testutil.LadderTrade(true, fills(watchedFills, "17:38:00")...),
		"futures":                               futures,
		"a child":                               child,
		"without the flag":                      off,
	} {
		if got := Apply(trade, "", underTheBand, indecisionReading()); latchNamed(got) {
			t.Errorf("%s: got %+v", name, got)
		}
	}
	for _, reading := range []aggragates.AIIndicators{brokenReading(), legOnAndQuiet(), verdictReading(), withBlock(aggragates.SmartSellIndicators{QuietDeclineBreakReasons: indecisionReasons})} {
		if got := Apply(watchedTrade(), "", underTheBand, reading); latchNamed(got) {
			t.Errorf("a reading without the indecision writes no latch, got %+v", got)
		}
	}
	withEarlyTakeProfit(t, false)
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
	if st := rebuildState(trade); st.slowDeclinePending || !st.earlyTakeProfit {
		t.Fatalf("the tick after: not pending and latched, got %+v", st)
	}
	assertNoSale(t, Apply(trade, "", slowDeclineBand, indecisionReading()), "")
}

// The depth priority hold pauses the slow decline gate and capital
// protection, never the sophos reading of the indecision: a held watched ladder
// gets the row the same ladder gets unheld — the row and the event it files,
// which the Row carries — and a held latched ladder's take profit reads the move
// against its position price as the unheld one does.
func TestADepthPriorityHoldLeavesTheEarlyTakeProfitDirectionOn(t *testing.T) {
	trade := indecisionLadder()
	unheld := Apply(trade, "", underTheBand, indecisionReading())
	assertRow(t, unheld, LatchedRow(trade.PositionPrice, indecisionFinding(indecisionReasons)))
	held := Apply(heldBy(trade, testutil.At("18:00:00")), "", underTheBand, indecisionReading())
	if len(held.Rows) != 1 || !reflect.DeepEqual(held.Rows[0], unheld.Rows[0]) || held.Position != "" || held.Reason != "" {
		t.Fatalf("held, the ladder gets the unheld row and nothing else, got %+v want %+v", held, unheld)
	}

	latched := latchedBy(trade)
	heldLatched := heldBy(latched, testutil.At("22:00:00"))
	if st := rebuildState(heldLatched); !st.earlyTakeProfit || !st.depthPriorityHeld {
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

// The swing decline latches a ladder the early take profit watches the tick
// it goes pending, once: the row that makes the ladder pending names the latch
// beside the swing decline, with no reasons of its own. A ladder already
// latched, and one the sophos reading latches on that very tick, get no second
// latch — the second names its own reasons and the swing decline's both; one the
// early take profit does not watch goes pending without a latch. Every tick
// runs at a print over the ladder's break even (gainPrint) with a buy order resting, where a ladder in buy
// is activated nothing and the gate's row stands alone, and the swing decline and
// the early take profit are switched on for the test (the quiet decline too,
// as the staircase tests are read), so that it reads the same at every tuning.
func TestTheSwingDeclineLatchesOnceAndOnlyWhereTheEarlyTakeProfitWatches(t *testing.T) {
	withBothDeclines(t)
	block := withBlock(stairFourthBlock())
	reasons := stairReading(t, stairFourth(), stairFourthBlock(), true).Reasons
	gain := gainPrint(stairFourth())

	assertRow(t, Apply(withRestingOrder(stairFourth()), "", gain, block), PendingRow(stairPrices[3], indecisionFinding(nil), swingFinding(reasons)))

	got := Apply(withRestingOrder(latchedBy(stairFourth())), "", gain, block)
	assertRow(t, got, PendingRow(stairPrices[3], swingFinding(reasons)))

	reading := stairFourthBlock()
	reading.Indecision, reading.QuietDeclineBreakReasons = true, indecisionReasons
	got = Apply(withRestingOrder(stairFourth()), "", gain, withBlock(reading))
	assertRow(t, got, PendingRow(stairPrices[3], indecisionFinding(indecisionReasons), swingFinding(reasons)))

	withEarlyTakeProfit(t, false)
	assertRow(t, Apply(withRestingOrder(stairFourth()), "", gain, block), PendingRow(stairPrices[3], swingFinding(reasons)))
}

package smarttakeloss

import (
	"strings"
	"testing"
	"time"

	"github.com/giovani-sirbu/mercury/trades/aggragates"
	"github.com/giovani-sirbu/mercury/trades/internal/testutil"
)

// The indecision row is schema like the slow-decline rows: the marker pinned
// byte for byte, framed like them with the reasons in parentheses, and
// neither holding nor held by any other marker rebuildState reads, nor by the
// first-fill hold's reason.
func TestIndecisionMessageIsFramedLikeTheMarker(t *testing.T) {
	if IndecisionMarker != "smartTakeLoss: indecision direction, take profit from the last buy" {
		t.Fatalf("the marker is schema and must not move, got %q", IndecisionMarker)
	}
	if got, want := IndecisionMessage("stopLoss", nil), "Hold stopLoss: "+IndecisionMarker; got != want {
		t.Fatalf("row %q, want %q", got, want)
	}
	if got := IndecisionMessage("buy", []string{"NATR over", "2 of 4 readings hold with a smooth ladder, 3 needed"}); got != "Hold buy: "+IndecisionMarker+" (NATR over, 2 of 4 readings hold with a smooth ladder, 3 needed)" {
		t.Fatalf("the reasons follow the marker in parentheses, got %q", got)
	}
	markers := []string{SlowDeclineMarker, SlowDeclineCancelMarker, SlowDeclineEntryHoldReason, IndecisionMarker}
	for _, one := range markers {
		for _, other := range markers {
			if one != other && strings.Contains(one, other) {
				t.Errorf("%q contains %q", one, other)
			}
		}
	}
}

// The row carries the newest fill's price, and rebuildState folds a row only
// with a price, so the rule watches no ladder before its first fill.
func TestIndecisionArmDepthNeedsAFill(t *testing.T) {
	if IndecisionArmDepth < 1 {
		t.Fatalf("IndecisionArmDepth %d must hold at least one fill", IndecisionArmDepth)
	}
}

// A long spot parent ladder from IndecisionArmDepth fills gets the row on the
// tick sophos serves the indecision on — IndecisionMessage naming what broke,
// at the newest fill's price — beside the proposal it leaves untouched: an
// add, nothing, or a close the ladder proposes or the trade rests in. It
// sells nothing. An impasse strategy's parent is watched too.
func TestApplyLatchesAWatchedLadderOnTheIndecision(t *testing.T) {
	trade := testutil.LadderTrade(false, fills(IndecisionArmDepth, "17:38:00")...)
	for _, position := range []string{"", "stopLoss", "buy", "takeProfit", "sell"} {
		got := Apply(trade, position, slowDeclineBand+1, indecisionReading())
		if got.Position != position || got.Reason != "" || got.SlowDecline != nil {
			t.Fatalf("%q: the proposal must pass untouched with no sale, got %+v", position, got)
		}
		assertRow(t, got.Indecision, IndecisionMessage("buy", indecisionReasons), trade.PositionPrice)
	}
	resting := trade
	resting.PositionType = "takeProfit"
	assertRow(t, Apply(resting, "", slowDeclineBand, indecisionReading()).Indecision, IndecisionMessage("takeProfit", indecisionReasons), trade.PositionPrice)
	impasse := trade
	impasse.Strategy.Params.Impasse = true
	assertRow(t, Apply(impasse, "", slowDeclineBand, indecisionReading()).Indecision, IndecisionMessage("buy", indecisionReasons), trade.PositionPrice)
}

// The row goes out once: from the tick after it the ladder reads latched, and
// a latched ladder gets no second row however often sophos serves the
// reading again, nor after a new fill. Nothing takes the latch away: not a
// reading without the indecision, a broken one, the verdict, the zero block,
// nor the cancel row a new fill gets.
func TestApplyWritesTheIndecisionRowOnce(t *testing.T) {
	trade := testutil.LadderTrade(false, fills(IndecisionArmDepth, "17:38:00")...)
	newest := trade.PositionPrice
	trade, got := engineTick(trade, "", underTheBand, testutil.At("18:00:00"), indecisionReading())
	assertRow(t, got.Indecision, IndecisionMessage("buy", indecisionReasons), newest)
	for index, reading := range []aggragates.AIIndicators{indecisionReading(), brokenReading(), withBlock(aggragates.SmartTakeLossIndicators{}), verdictReading(), indecisionReading()} {
		var next Result
		trade, next = engineTick(trade, "", underTheBand, testutil.At("18:15:00").Add(time.Duration(index)*time.Minute), reading)
		if next.Indecision != nil {
			t.Fatalf("reading %d: a latched ladder gets no second row, got %+v", index, next)
		}
		if !rebuildState(trade).indecision {
			t.Fatalf("reading %d: nothing takes the latch away", index)
		}
	}
	w3s := fills(IndecisionArmDepth+1, "19:00:00")
	trade = withFill(trade, w3s[IndecisionArmDepth].Price, testutil.At("19:00:00"))
	trade, got = engineTick(trade, "", underTheBand, testutil.At("19:15:00"), indecisionReading())
	if got.Indecision != nil || got.SlowDecline == nil || !rebuildState(trade).indecision {
		t.Fatalf("a new fill is judged on the reading and neither unlatches the ladder nor writes a second row, got %+v", got)
	}
	rows := 0
	for _, row := range trade.Logs {
		if strings.Contains(row.Message, IndecisionMarker) {
			rows++
		}
	}
	if rows != 1 {
		t.Fatalf("one indecision row over every tick, got %d in %+v", rows, trade.Logs)
	}
}

// A ladder the rule does not watch gets no row on the indecision: one short
// of IndecisionArmDepth, with no fill, inverse, futures, a child, a strategy
// without the flag, and every ladder while IndecisionDirection is off. A
// watched ladder gets none on a reading without the indecision, whatever
// else it serves.
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
		if got := Apply(trade, "", underTheBand, indecisionReading()); got.Indecision != nil {
			t.Errorf("%s: got %+v", name, got)
		}
	}
	for _, reading := range []aggragates.AIIndicators{brokenReading(), legOnAndQuiet(), verdictReading(), withBlock(aggragates.SmartTakeLossIndicators{SlowDeclineBreakReasons: indecisionReasons})} {
		if got := Apply(watchedTrade(), "", underTheBand, reading); got.Indecision != nil {
			t.Errorf("a reading without the indecision writes no row, got %+v", got)
		}
	}
	withIndecisionDirection(t, false)
	if got := Apply(watchedTrade(), "", underTheBand, indecisionReading()); got.Indecision != nil {
		t.Fatalf("switched off, no row, got %+v", got)
	}
}

// A pending ladder whose new fill is judged on the indecision reading gets
// both rows on that tick: the cancel row, since the leg is not on and quiet,
// then the indecision row that latches it, each naming what broke and
// carrying the new fill's price. From the next tick it is latched and not
// pending, and its band sells nothing.
func TestApplyCancelsAndLatchesOnOneTick(t *testing.T) {
	trade, got := engineTick(pendingAtFive(), "", underJudgeBand, judgeTick, indecisionReading())
	assertRow(t, got.SlowDecline, SlowDeclineCancelMessage("buy", indecisionReasons), sixthFill)
	assertRow(t, got.Indecision, IndecisionMessage("buy", indecisionReasons), sixthFill)
	if st := rebuildState(trade); st.slowDeclinePending || !st.indecision {
		t.Fatalf("the tick after: not pending and latched, got %+v", st)
	}
	assertNoSale(t, Apply(trade, "", slowDeclineBand, indecisionReading()), "")
}

// The depth priority hold pauses the quiet slow decline and capital
// protection, never the indecision direction: a held watched ladder gets the
// row the same ladder gets unheld, and a held latched ladder's take profit
// reads the move against its position price as the unheld one does.
func TestADepthPriorityHoldLeavesTheIndecisionDirectionOn(t *testing.T) {
	trade := indecisionLadder()
	unheld := Apply(trade, "", underTheBand, indecisionReading())
	assertRow(t, unheld.Indecision, IndecisionMessage("buy", indecisionReasons), trade.PositionPrice)
	held := Apply(heldBy(trade, testutil.At("18:00:00")), "", underTheBand, indecisionReading())
	if held.Indecision == nil || *held.Indecision != *unheld.Indecision || held.SlowDecline != nil || held.Position != "" || held.Reason != "" {
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

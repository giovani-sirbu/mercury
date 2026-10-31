package smartsell

import (
	"strings"
	"testing"
	"time"

	"github.com/giovani-sirbu/mercury/internal/testutil"
	"github.com/giovani-sirbu/mercury/strategies/gates/cooldown"
	"github.com/giovani-sirbu/mercury/strategies/gates/dynamicparams"
	"github.com/giovani-sirbu/mercury/strategies/gates/smartsell/earlytakeprofit"
	"github.com/giovani-sirbu/mercury/trades/aggragates"
	"github.com/giovani-sirbu/mercury/trades/ladder"
)

// indecisionFills is how deep the ladder the early take profit watches
// stands in these tests: earlytakeprofit.ArmDepth fills and never fewer
// than two, so the position price differs from the average entry price and a
// take profit that reads the one is told from a take profit that reads the
// other.
var indecisionFills = max(earlytakeprofit.ArmDepth, 2)

// indecisionLabel is the words a row prints for the early take profit, the
// ones the rows naming a latch have in common.
const indecisionLabel = "early take profit"

// indecisionLadder is the w3s ladder the early take profit watches from
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

// gainPrint is the print a dollar over the ladder's break even, where a sale of the whole
// position books a gain, which reaches any trigger at or under zero: the shared action activates
// a ladder in buy there and sells one in trailingBuy. The tests that read a gate's own row on a
// ladder in buy tick it here with a buy order resting (withRestingOrder), where the action waits
// in silence, so the tick writes the rows of the gates' own ticks alone.
func gainPrint(trade aggragates.Trades) float64 {
	price, _, _ := overBreakEven(trade)
	return price
}

// tickResting is engineTick with a buy order resting for the tick (withRestingOrder), the order gone
// after it: in buy the shared action waits, so the trade carries the gates' own rows alone.
// applyAtAGain and tickAtAGain read it at gainPrint.
func tickResting(trade aggragates.Trades, position string, price float64, now time.Time, ai aggragates.AIIndicators) (aggragates.Trades, Result) {
	ticked, got := engineTick(withRestingOrder(trade), position, price, now, ai)
	ticked.PendingOrder = trade.PendingOrder
	return ticked, got
}

func applyAtAGain(trade aggragates.Trades, position string, ai aggragates.AIIndicators) Result {
	return Apply(withRestingOrder(trade), position, gainPrint(trade), ai)
}

func tickAtAGain(trade aggragates.Trades, position string, now time.Time, ai aggragates.AIIndicators) (aggragates.Trades, Result) {
	return tickResting(trade, position, gainPrint(trade), now, ai)
}

// A watched ladder the depth priority holds is latched on the indecision as
// on any other tick: it gets the row the same ladder gets unheld, carrying the
// newest fill's price — never the position price, a re-anchor included —
// beside the proposal it leaves untouched. The row marks and sells nothing.
// From the next tick the ladder reads latched and held, and no second row or
// event goes out however often sophos serves a reading. The indecision
// direction alone watching the ladder, the other features switched off, changes
// none of it.
func TestAHeldWatchedLadderIsLatchedOnTheEarlyTakeProfit(t *testing.T) {
	withEarlyTakeProfit(t, true)
	trade := indecisionLadder()
	newest := rebuildState(trade).lastFill().Price
	want := LatchedRow(newest, indecisionFinding(indecisionReasons))
	assertRow(t, Apply(trade, "", underTheBand, indecisionReading()), want)

	held := heldBy(trade, testutil.At("18:00:00"))
	reanchored := held
	reanchored.PositionPrice = newest + 5
	for _, candidate := range []aggragates.Trades{held, reanchored} {
		for _, position := range []string{"", "trailingBuy", "takeProfit"} {
			got := Apply(candidate, position, underTheBand, indecisionReading())
			assertRow(t, got, want)
			assertNoSale(t, got, position)
		}
	}

	latched, _ := engineTick(held, "", underTheBand, testutil.At("18:10:00"), indecisionReading())
	if st := rebuildState(latched); !st.earlyTakeProfit || !st.earlyTakeProfitWatched || !st.depthPriorityHeld || st.slowDeclinePending {
		t.Fatalf("the tick after: latched, watched and held, not pending, got %+v", st)
	}
	for index, reading := range []aggragates.AIIndicators{indecisionReading(), slowDeclineBlock(true), brokenReading(), withBlock(aggragates.SmartSellIndicators{})} {
		for _, position := range []string{"", "trailingBuy"} {
			var next Result
			latched, next = engineTick(latched, position, slowDeclineBand+5, testutil.At("18:15:00").Add(time.Duration(index)*time.Minute), reading)
			assertUntouched(t, next, position)
		}
	}
	rows, events := carriedRows(latched, indecisionLabel), carriedLatches(latched)
	if rows != 1 || events != 1 {
		t.Fatalf("one row naming the latch and one event carrying it over every held tick, got %d and %d in %+v and %+v", rows, events, latched.Logs, latched.StrategyEvents)
	}

	withQuietDecline(t, false)
	withSwingDecline(t, false)
	withCapitalProtectionExit(t, false)
	alone := Apply(held, "", underTheBand, indecisionReading())
	assertRow(t, alone, want)
	assertNoSale(t, alone, "")
}

// A held ladder both pending and served the indecision gets ONE row on its
// first held tick — the reset row, the indecision latch named in its head and
// its reasons in a group of their own, at the newest fill's price — and neither
// activates the trailingBuy nor sells, on any tick, in any state: not the activation of
// the ladder resting in buy nor the sale of the one resting in trailingBuy that the same
// ladder unheld gets, a protected close included. The engines write the row with its
// event, so the trade carries one reset event naming the latch. From the next tick
// it reads watched, latched and held, not pending, and nothing more is written.
func TestAHeldPendingLadderServedTheEarlyTakeProfitIsResetAndLatchedOnOneRow(t *testing.T) {
	withQuietDecline(t, true)
	withEarlyTakeProfit(t, true)
	withDepthPriorityPause(t, true)
	unheld, latch := pendingTrade(), LatchedRow(slowDeclineLastFill, indecisionFinding(indecisionReasons))
	armed := armingPrice(unheld)
	control := Apply(unheld, "trailingBuy", armed, indecisionReading())
	assertActivated(t, control, latch, ActivatedRow(slowDeclineLastFill, -handLossPct(unheld, armed), FeatureQuietDecline))
	assertSoldWith(t, Apply(restingIn(unheld, "trailingBuy"), "trailingBuy", armed, indecisionReading()), reasonSlowDecline, latch)
	ordinary := applyAtAGain(unheld, "", indecisionReading())
	assertNoSale(t, ordinary, "")
	assertRow(t, ordinary, latch)

	reset := ResetRow(slowDeclineLastFill, indecisionFinding(indecisionReasons), quietFinding(nil))
	if !strings.HasPrefix(reset.Message, "smartSell: early take profit latched, slow decline reset, depth priority holds this ladder (indecision: ") {
		t.Fatalf("the reset row names the latch first, got %q", reset.Message)
	}
	held := heldBy(pendingTrade(), testutil.At("18:05:00"))
	protected := Apply(held, "sell", slowDeclineBand+5, indecisionReading())
	assertRow(t, protected, reset)
	assertNoSale(t, protected, "sell")
	trailingBuy := Apply(restingIn(held, "trailingBuy"), "", armed, indecisionReading())
	assertRow(t, trailingBuy, reset)
	assertNoSale(t, trailingBuy, "")

	trade, got := engineTick(held, "trailingBuy", armingPrice(held), testutil.At("18:10:00"), indecisionReading())
	assertRow(t, got, reset)
	assertNoSale(t, got, "trailingBuy")
	if st := rebuildState(trade); !st.slowDeclineWatched || st.slowDeclinePending || !st.earlyTakeProfit || !st.depthPriorityHeld {
		t.Fatalf("the tick after: watched, not pending, latched and held, got %+v", st)
	}
	assertSoldNever(t, trade, indecisionReading())
	if rows := len(trade.Logs) - len(held.Logs); rows != 1 || carriedRows(trade, "slow decline reset") != 1 {
		t.Fatalf("one row, the reset naming the latch, got %+v", trade.Logs)
	}
	if written := len(trade.StrategyEvents) - len(held.StrategyEvents); written != 1 || carriedEvents(trade, GateSlowDecline, EventReset) != 1 || carriedLatches(trade) != 1 {
		t.Fatalf("one reset event naming the latch, got %+v", trade.StrategyEvents)
	}
}

// A latch taken before the hold keeps working through it: the ladder reads
// latched and held, no second row goes out, and its take profit reads the
// move against the position price all the same — the larger of that move and
// the move it is handed, and the move it is handed under break even. The next
// fill ends the hold and keeps the latch, and the take profit reads the new
// fill's position price on the one row and event still.
func TestALatchedLadderTheDepthPriorityHoldsStillReadsItsPositionPrice(t *testing.T) {
	withEarlyTakeProfit(t, true)
	latched := latchedBy(indecisionLadder())
	held := heldBy(latched, testutil.At("22:00:00"))
	if st := rebuildState(held); !st.earlyTakeProfit || !st.depthPriorityHeld {
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
	if st := rebuildState(resumed); !st.earlyTakeProfit || st.depthPriorityHeld {
		t.Fatalf("the next fill ends the hold and keeps the latch, got %+v", st)
	}
	price, fromAverage, fromPosition = overBreakEven(resumed)
	if got := TakeProfitPercentage(resumed, price, fromAverage); got != fromPosition || !(fromAverage < fromPosition) {
		t.Fatalf("resumed, the take profit reads the new fill %v, got %v", fromPosition, got)
	}
	if got := Apply(resumed, "", underTheBand, indecisionReading()); latchNamed(got) || carriedRows(resumed, indecisionLabel) != 1 || carriedLatches(resumed) != 1 {
		t.Fatalf("one row naming the latch and one event, before the hold and after it, got %+v on %+v and %+v", got, resumed.Logs, resumed.StrategyEvents)
	}
}

// carriedRows counts the trade's rows whose text contains words: the text the
// operator reads, twin of carriedEvents.
func carriedRows(trade aggragates.Trades, words string) int {
	count := 0
	for _, logged := range trade.Logs {
		if strings.Contains(logged.Message, words) {
			count++
		}
	}
	return count
}

// carriedEvents counts the trade's smartSell events of gate and kind: one
// for every row the engines wrote for it, twin of carriedRows.
func carriedEvents(trade aggragates.Trades, gate, kind string) int {
	count := 0
	for _, event := range trade.StrategyEventsOf(aggragates.StrategyParamSmartSell, gate) {
		if event.Kind() == kind {
			count++
		}
	}
	return count
}

// carriedLatches counts the trade's events of the slow decline gate that carry
// the indecision among their features: one for every row that latched the
// ladder.
func carriedLatches(trade aggragates.Trades) int {
	count := 0
	for _, event := range trade.StrategyEventsOf(aggragates.StrategyParamSmartSell, GateSlowDecline) {
		var data EventData
		if event.DecodeData(&data) != nil {
			continue
		}
		for _, feature := range data.Features {
			if feature == FeatureEarlyTakeProfit {
				count++
			}
		}
	}
	return count
}

// Switched off, the gate's events are ignored by every rule: a held pending
// ladder is acted on like any other, its trailingBuy activated in buy and the ladder sold
// in trailingBuy, with no reset row, a held watched ladder goes pending on the verdict and
// is latched on the indecision, a held pending ladder served the indecision is latched
// beside the activation or the sale and gets no reset row, and a held latched one reads
// its position price, as a held pending one reads its newest fill.
func TestSwitchedOffTheDepthPriorityRowsAreIgnored(t *testing.T) {
	withQuietDecline(t, true)
	withEarlyTakeProfit(t, true)
	withDepthPriorityPause(t, false)
	held := heldBy(pendingTrade(), testutil.At("18:05:00"))
	armed := armingPrice(held)
	activation := ActivatedRow(slowDeclineLastFill, -handLossPct(held, armed), FeatureQuietDecline)
	assertActivated(t, Apply(held, "trailingBuy", armed, slowDeclineBlock(true)), activation)
	assertSold(t, Apply(restingIn(held, "trailingBuy"), "trailingBuy", armed, slowDeclineBlock(true)), reasonSlowDecline)
	watched := heldBy(watchedTrade(), testutil.At("18:05:00"))
	assertRow(t, applyAtAGain(watched, "", slowDeclineBlock(true)), quietPending(slowDeclineLastFill))
	assertRow(t, Apply(heldBy(indecisionLadder(), testutil.At("18:00:00")), "", underTheBand, indecisionReading()), LatchedRow(indecisionLadder().PositionPrice, indecisionFinding(indecisionReasons)))
	latch := LatchedRow(slowDeclineLastFill, indecisionFinding(indecisionReasons))
	assertActivated(t, Apply(held, "trailingBuy", armed, indecisionReading()), latch, activation)
	assertSoldWith(t, Apply(restingIn(held, "trailingBuy"), "trailingBuy", armed, indecisionReading()), reasonSlowDecline, latch)
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
// the notification filter). No row the gate writes — going pending, confirming,
// cancelling, resetting, latching, the activation of the trailingBuy, the sale and the
// held sale, whichever bound held it, and the flat market gate's episode rows —
// contains the first-fill hold's reason, the depth priority marker or the opened row's
// prefix and marker, or is contained in any, and the gate's own rows carry no hold frame,
// which belongs to the refusals alone.
func TestNoRowIsReadAsAnother(t *testing.T) {
	trade := lastDepthLadder()
	rows := []Row{
		PendingRow(slowDeclineLastFill, quietFinding(slowDeclineReasons)),
		PendingRow(slowDeclineLastFill, indecisionFinding(nil), swingFinding(slowDeclineReasons)),
		ConfirmedRow(sixthFill, quietFinding(slowDeclineReasons), swingFinding(quietDeclineRecentReasons)),
		CancelledRow(sixthFill, quietFinding(quietDeclineBreakReasons), swingFinding(quietDeclineBreakReasons)),
		ResetRow(slowDeclineLastFill, quietFinding(nil)),
		LatchedRow(slowDeclineLastFill, indecisionFinding(indecisionReasons)),
		LatchedRow(slowDeclineLastFill, Finding{Feature: FeatureDynamicParams, Reasons: raiseReasons}),
		ExitRow(trade, reasonSlowDecline, armingPrice(trade)),
		ExitRow(trade, reasonFlatMarket, armingPrice(trade)),
		ExitRow(trade, reasonCapitalProtection, lastDepthPrint),
		capitalProtectionTriggered(trade),
		CapitalProtectionResetRow(trade.PositionPrice),
		CapitalProtectionSaleHeldRow(trade.PositionPrice, -1.5),
		CapitalProtectionActivatedRow(trade.PositionPrice, -0.25),
		SaleHeldRow(slowDeclineLastFill, -1.5, FeatureQuietDecline),
		SaleHeldAtDepthRow(slowDeclineLastFill, watchedFills, lastDepthFills, FeatureQuietDecline),
		ActivatedRow(slowDeclineLastFill, -0.25, FeatureQuietDecline),
		FlatMarketTriggeredRow(slowDeclineLastFill, slowDeclineReasons),
		FlatMarketResetRow(slowDeclineLastFill),
		FlatMarketSaleHeldRow(slowDeclineLastFill, -1.5),
		FlatMarketSaleHeldAtDepthRow(slowDeclineLastFill, watchedFills, lastDepthFills),
		FlatMarketActivatedRow(slowDeclineLastFill, -0.25),
	}
	others := []string{SlowDeclineEntryHoldReason, cooldown.DepthPriorityHoldMarker, dynamicparams.RowPrefix, dynamicparams.OpenedMarker}
	for _, row := range rows {
		if strings.Contains(row.Message, "Hold ") {
			t.Errorf("%q carries the hold frame", row.Message)
		}
		for _, other := range others {
			if strings.Contains(row.Message, other) || strings.Contains(other, row.Message) {
				t.Errorf("%q and %q contain one another", row.Message, other)
			}
		}
	}
	gate := heldBy(watchedTrade(), testutil.At("18:05:00")).Logs[0].Message
	for _, row := range rows {
		if strings.Contains(gate, row.Message) {
			t.Errorf("the depth priority gate's row %q names %q", gate, row.Message)
		}
	}
}

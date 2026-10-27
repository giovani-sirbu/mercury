package smarttakeloss

import (
	"slices"
	"strings"
	"testing"
	"time"

	"github.com/giovani-sirbu/mercury/internal/testutil"
	"github.com/giovani-sirbu/mercury/strategies/gates/smarttakeloss/earlytakeprofit"
	"github.com/giovani-sirbu/mercury/trades/aggragates"
)

// indecisionRecentReasons is what sophos names for the bar among its last closed
// bars on which the indecision stood: that bar, then its own break reasons.
var indecisionRecentReasons = []string{"indecision read on the 1h bar opening 2021-07-26 16:00 UTC, one of the last 24 closed bars", "2 of 4 readings hold with a smooth ladder, 3 needed"}

// withIndecisionLookBack is the block serving a bar among the last closed bars on
// which the indecision stood, the one opening at at, and that bar's reasons. The
// flag of the last closed bar is the block's own.
func withIndecisionLookBack(block aggragates.SmartTakeLossIndicators, at time.Time) aggragates.SmartTakeLossIndicators {
	block.IndecisionRecentAt = at.UnixMilli()
	block.IndecisionRecentReasons = indecisionRecentReasons
	return block
}

// lookedBackReading is a reading whose last closed bar broke on its own reasons —
// no indecision, no verdict, no leg on and quiet — on a window sophos read
// (QuietDeclineFillFrom), and whose look-back names the bar opening at 16:00, which
// the indecision stood on.
func lookedBackReading() aggragates.AIIndicators {
	broke := aggragates.SmartTakeLossIndicators{QuietDeclineFillFrom: fillWindowFrom.UnixMilli(), QuietDeclineBreakReasons: quietDeclineBreakReasons}
	return withBlock(withIndecisionLookBack(broke, testutil.At("16:00:00")))
}

// A watched ladder is latched on the look-back: sophos served the indecision on a
// bar among its last closed bars, not on the last one, whose flag is false. The
// first tick writes the latch at the newest fill's price, naming that bar's
// reasons and not what the last closed bar broke on, beside the proposal it
// leaves untouched, and sells nothing. The ladder need not have stood when the bar
// did: one opened after it, one with the bar between its fills and one whose
// newest fill came before it are all latched on their first tick.
func TestTheEarlyTakeProfitLookBackLatchesAWatchedLadder(t *testing.T) {
	withEarlyTakeProfit(t, true)
	trade := indecisionLadder()
	want := LatchedRow(trade.PositionPrice, indecisionFinding(indecisionRecentReasons))
	text := "smartTakeLoss: early take profit latched (indecision: " + strings.Join(indecisionRecentReasons, ", ") + ")"
	if want.Message != text || want.Event != EventLatched || !slices.Equal(want.Features, []string{FeatureEarlyTakeProfit}) || !slices.Equal(want.Reasons, indecisionRecentReasons) {
		t.Fatalf("one row of the latched kind naming the bar the indecision stood on, got %+v", want)
	}
	for _, position := range []string{"", "stopLoss", "buy", "takeProfit", "sell"} {
		got := Apply(trade, position, slowDeclineBand+1, lookedBackReading())
		assertRow(t, got, want)
		assertNoSale(t, got, position)
	}
	resting := trade
	resting.PositionType = "takeProfit"
	assertRow(t, Apply(resting, "", slowDeclineBand, lookedBackReading()), want)
	for name, stood := range map[string]string{"before the ladder opened": "07:00:00", "between its fills": "12:00:00", "after its newest fill": "18:00:00"} {
		t.Run(name, func(t *testing.T) {
			assertRow(t, Apply(trade, "", underTheBand, withBlock(withIndecisionLookBack(lookedBackReading().SmartTakeLoss, testutil.At(stood)))), want)
		})
	}
}

// The look-back joins the row of the tick, as the flag does, and never writes a
// second one: the judgement of a pending ladder's new fill keeps its own row with
// the latch named in its head, the ladder not pending from then on, and a swing
// decline that triggers on the same tick shares the pending row, both named. The
// swing decline's tick runs at a print over the ladder's break even (gainPrint) with a buy order resting,
// where a ladder in buy is activated nothing and the gate's row stands alone; the
// quiet decline, the swing decline and the early take profit are switched on
// for the test, so that it reads the same at every tuning.
func TestTheEarlyTakeProfitLookBackJoinsTheRowOfTheTick(t *testing.T) {
	withBothDeclines(t)
	looked := withIndecisionLookBack(brokenReading().SmartTakeLoss, testutil.At("16:00:00"))
	cancelled, got := engineTick(pendingAtFive(), "", underJudgeBand, judgeTick, withBlock(looked))
	assertRow(t, got, CancelledRow(sixthFill, indecisionFinding(indecisionRecentReasons), quietFinding(quietDeclineBreakReasons)))
	if st := rebuildState(cancelled); st.slowDeclinePending || !st.earlyTakeProfit {
		t.Errorf("the tick after: latched and not pending, got %+v", st)
	}

	trade, block := stairFourth(), stairFourthBlock()
	reading, holds := readsAsOf(trade, block)
	needsAScenario(t, rebuildState(trade).swingDeclineWatched && holds, "a staircase the swing decline reads at the fourth fill")
	got = applyAtAGain(trade, "", withBlock(withIndecisionLookBack(block, stairOpen(16))))
	assertRow(t, got, PendingRow(stairPrices[3], indecisionFinding(indecisionRecentReasons), swingFinding(reading.Reasons)))
	if !strings.HasPrefix(got.Rows[0].Message, "smartTakeLoss: early take profit latched, swing decline triggered (") {
		t.Errorf("the one row names both, got %q", got.Rows[0].Message)
	}
}

// The look-back is asked after the flag of the last closed bar and before the
// swing decline's own latch: a pending ladder whose judged newest fill the swing
// decline reads for is latched naming the swing decline's reasons when nothing
// else serves the indecision, naming the look-back's when it does, and the flag's
// break reasons when the flag stands as well. The pending ladder's trigger stands,
// so the ticks run at gainPrint with a buy order resting (applyAtAGain), where the
// action waits and the latch row stands alone; the quiet decline (the
// feature it is pending on), the swing decline and the early take profit are
// switched on for the test, so that it reads the same at every tuning.
func TestTheEarlyTakeProfitLookBackComesAfterTheFlagAndBeforeTheSwingDeclinesLatch(t *testing.T) {
	withBothDeclines(t)
	pending, block := withRows(stairFourth(), PendingRow(stairPrices[3], quietFinding(slowDeclineReasons))), stairFourthBlock()
	reading, holds := readsAsOf(pending, block)
	needsAScenario(t, rebuildState(pending).swingDeclineWatched && holds, "a staircase the swing decline reads at the fourth fill")
	assertRow(t, applyAtAGain(pending, "", withBlock(block)), LatchedRow(stairPrices[3], swingFinding(reading.Reasons)))

	looked := withIndecisionLookBack(block, stairOpen(16))
	assertRow(t, applyAtAGain(pending, "", withBlock(looked)), LatchedRow(stairPrices[3], indecisionFinding(indecisionRecentReasons)))
	looked.Indecision, looked.QuietDeclineBreakReasons = true, indecisionReasons
	assertRow(t, applyAtAGain(pending, "", withBlock(looked)), LatchedRow(stairPrices[3], indecisionFinding(indecisionReasons)))
}

// A depth priority hold pauses the gate and never the look-back: the held ladder
// gets the row the unheld one gets, a held pending ladder one reset row naming the
// latch, and a ladder the swing decline would trigger on, which a hold pauses,
// gets the latch alone.
func TestADepthPriorityHoldLeavesTheEarlyTakeProfitLookBackOn(t *testing.T) {
	trade := indecisionLadder()
	want := LatchedRow(trade.PositionPrice, indecisionFinding(indecisionRecentReasons))
	held := Apply(heldBy(trade, testutil.At("18:00:00")), "", underTheBand, lookedBackReading())
	assertRow(t, held, want)
	assertNoSale(t, held, "")

	reset := Apply(heldBy(pendingTrade(), testutil.At("18:05:00")), "", slowDeclineBand+1, lookedBackReading())
	assertRow(t, reset, ResetRow(slowDeclineLastFill, indecisionFinding(indecisionRecentReasons), quietFinding(nil)))
	assertNoSale(t, reset, "")

	stair, block := stairFourth(), stairFourthBlock()
	_, holds := readsAsOf(stair, block)
	needsAScenario(t, rebuildState(stair).swingDeclineWatched && holds, "a staircase the swing decline reads at the fourth fill")
	looked := withBlock(withIndecisionLookBack(block, stairOpen(16)))
	assertRow(t, Apply(heldBy(stair, stairOpen(25)), "", underTheBand, looked), LatchedRow(stairPrices[3], indecisionFinding(indecisionRecentReasons)))
}

// A ladder the feature does not watch gets no latch from the look-back: one short
// of earlytakeprofit.ArmDepth, with no fill, inverse, futures, a child, a
// strategy without the flag, and every ladder while earlytakeprofit.Enabled is off.
// A watched ladder gets none from reasons with no bar, nor from a bar not above
// zero.
func TestTheEarlyTakeProfitLookBackLatchesNothingItDoesNotWatch(t *testing.T) {
	futures := watchedTrade()
	futures.Strategy.TradeType = aggragates.Futures
	child := watchedTrade()
	child.ParentID = 7
	off := watchedTrade()
	off.Strategy.Params.SmartTakeLoss = false
	for name, trade := range map[string]aggragates.Trades{
		"one short of earlytakeprofit.ArmDepth": testutil.LadderTrade(false, fills(earlytakeprofit.ArmDepth-1, "17:38:00")...),
		"with no fill":                          testutil.LadderTrade(false),
		"inverse":                               testutil.LadderTrade(true, fills(watchedFills, "17:38:00")...),
		"futures":                               futures,
		"a child":                               child,
		"without the flag":                      off,
	} {
		if got := Apply(trade, "", underTheBand, lookedBackReading()); latchNamed(got) {
			t.Errorf("%s: got %+v", name, got)
		}
	}
	for name, at := range map[string]int64{"reasons with no bar": 0, "a bar not above zero": -1} {
		block := aggragates.SmartTakeLossIndicators{QuietDeclineFillFrom: fillWindowFrom.UnixMilli(), IndecisionRecentAt: at, IndecisionRecentReasons: indecisionRecentReasons}
		if got := Apply(indecisionLadder(), "", underTheBand, withBlock(block)); len(got.Rows) != 0 || got.Reason != "" {
			t.Errorf("%s: nothing is latched, got %+v", name, got)
		}
	}
	withEarlyTakeProfit(t, false)
	if got := Apply(indecisionLadder(), "", underTheBand, lookedBackReading()); latchNamed(got) {
		t.Fatalf("switched off, no latch, got %+v", got)
	}
}

// The latch goes out once: from the tick after it the ladder reads latched, and a
// latched ladder gets no second row however often sophos serves the look-back or
// any other reading, nor after a new fill; nothing takes the latch away. A ladder
// the flag latched is not latched again by the look-back either.
func TestTheEarlyTakeProfitLookBackLatchesOnce(t *testing.T) {
	withEarlyTakeProfit(t, true)
	trade := indecisionLadder()
	trade, got := engineTick(trade, "", underTheBand, testutil.At("18:00:00"), lookedBackReading())
	assertRow(t, got, LatchedRow(trade.PositionPrice, indecisionFinding(indecisionRecentReasons)))
	for index, reading := range []aggragates.AIIndicators{lookedBackReading(), indecisionReading(), brokenReading(), withBlock(aggragates.SmartTakeLossIndicators{}), lookedBackReading()} {
		var next Result
		trade, next = engineTick(trade, "", underTheBand, testutil.At("18:15:00").Add(time.Duration(index)*time.Minute), reading)
		if latchNamed(next) || !rebuildState(trade).earlyTakeProfit {
			t.Fatalf("reading %d: a latched ladder gets no second row and keeps its latch, got %+v", index, next)
		}
	}
	trade = withFill(trade, w3sPrice(indecisionFills+1), testutil.At("19:00:00"))
	trade, got = engineTick(trade, "", underTheBand, testutil.At("19:15:00"), lookedBackReading())
	if latchNamed(got) || !rebuildState(trade).earlyTakeProfit {
		t.Fatalf("a new fill neither unlatches the ladder nor writes a second latch, got %+v", got)
	}
	if rows, latches := carriedRows(trade, indecisionLabel), carriedLatches(trade); rows != 1 || latches != 1 {
		t.Fatalf("one row naming the latch and one event carrying it over every tick, got %d and %d in %+v and %+v", rows, latches, trade.Logs, trade.StrategyEvents)
	}
	flagged, _ := engineTick(indecisionLadder(), "", underTheBand, testutil.At("18:00:00"), indecisionReading())
	if got := Apply(flagged, "", underTheBand, lookedBackReading()); latchNamed(got) {
		t.Fatalf("a ladder the flag latched is not latched again, got %+v", got)
	}
}

package smartsell

import (
	"github.com/giovani-sirbu/mercury/strategies/gates/smartsell/series"
	"slices"
	"testing"

	"github.com/giovani-sirbu/mercury/internal/testutil"
	"github.com/giovani-sirbu/mercury/strategies/gates/smartsell/swingdecline"
	"github.com/giovani-sirbu/mercury/trades/aggragates"
)

// The latch reads the control ladder and nothing else: each case below is the control with one thing changed.
func TestTheFlatMarketLatchReadsNothingOffItsWatchOrWithoutItsBars(t *testing.T) {
	onlyTheFlatMarketLatch(t)
	control := latchLadder(latchDepth)
	ai := latchMarket(t, control, 0, 0)
	want := latchRowOf(t, control, ai.SmartSell)
	assertRow(t, Apply(control, "", control.PositionPrice, ai), want)

	futures, child, unflagged, unpriced, unsettled := control, control, control, control, control
	futures.Strategy.TradeType = aggragates.Futures
	child.ParentID = 7
	unflagged.Strategy.Params.SmartSell = false
	unpriced.StrategyPair.StrategySettings = slices.Clone(control.StrategyPair.StrategySettings)
	unpriced.StrategyPair.StrategySettings[0].Percentage = 0
	unsettled.StrategyPair.StrategySettings = nil
	off := map[string]aggragates.Trades{
		"a futures ladder": futures, "an inverse ladder": testutil.LadderTrade(true, fills(latchDepth, "17:38:00")...),
		"a child": child, "a strategy without the flag": unflagged, "a percentage of zero": unpriced,
		"a pair without settings": unsettled, "a ladder with no fill": testutil.LadderTrade(false),
	}
	for name, trade := range off {
		if got := Apply(trade, "", control.PositionPrice, ai); got.Position != "" || got.Reason != "" || len(got.Rows) != 0 {
			t.Errorf("%s: want nothing read, got %+v", name, got)
		}
	}

	for name, miss := range flatMarketMisses(basePercentage(control), aggragates.SideLong) {
		if name == "trend short" {
			assertRow(t, Apply(control, "", control.PositionPrice, miss), want)
			continue
		}
		assertUntouched(t, Apply(control, "", control.PositionPrice, miss), "")
	}

	withEarlyTakeProfitFlatMarket(t, false)
	assertUntouched(t, Apply(control, "", control.PositionPrice, ai), "")
	withEarlyTakeProfitFlatMarket(t, true)
	withEarlyTakeProfit(t, false)
	assertUntouched(t, Apply(control, "", control.PositionPrice, ai), "")
}

// The latch does not need the flat market gate: the same single row goes out with it on, the trend missing, and off.
func TestTheFlatMarketLatchStillLatchesWithTheGateOff(t *testing.T) {
	onlyTheFlatMarketLatch(t)
	needsTheGateTrend(t)
	trade := latchLadder(latchDepth)
	ai := latchMarket(t, trade, 0, 3)
	for _, gate := range []bool{true, false} {
		withFlatMarket(t, gate)
		got := Apply(trade, "", trade.PositionPrice, ai)
		assertRow(t, got, latchRowOf(t, trade, ai.SmartSell))
		assertNoSale(t, got, "")
	}
}

// The sophos flag and the look-back keep their precedence and name sophos' reasons, and so does the linear decline.
func TestTheOtherReadingsKeepPrecedenceOverTheFlatMarketBars(t *testing.T) {
	onlyTheFlatMarketLatch(t)
	trade := latchLadder(latchDepth)
	ai := latchMarket(t, trade, 0, 0)
	newest := trade.PositionPrice
	assertRow(t, Apply(trade, "", newest, ai), latchRowOf(t, trade, ai.SmartSell))

	flag := ai
	flag.SmartSell.Indecision, flag.SmartSell.QuietDeclineBreakReasons = true, indecisionReasons
	assertRow(t, Apply(trade, "", newest, flag), LatchedRow(newest, indecisionFinding(indecisionReasons)))

	recent := withBlock(withIndecisionLookBack(ai.SmartSell, testutil.At("16:00:00")))
	assertRow(t, Apply(trade, "", newest, recent), LatchedRow(newest, indecisionFinding(indecisionRecentReasons)))

	line, block := linearScene(linearTrendMet)
	block = withSmcTrend(flatMarketOver(block, line), linearTrendMet, linearTrendValid)
	withEarlyTakeProfitLinearDecline(t, false)
	assertRow(t, Apply(line, "", line.PositionPrice, withBlock(block)), latchRowOf(t, line, block))
	withEarlyTakeProfitLinearDecline(t, true)
	assertRow(t, Apply(line, "", line.PositionPrice, withBlock(block)), linearRow(t, line, block))
}

// The swing decline, stubbed to read, keeps its precedence: it shares the pending row or names the latch of a judged fill.
func TestTheSwingDeclineKeepsPrecedenceOverTheFlatMarketBars(t *testing.T) {
	onlyTheFlatMarketLatch(t)
	withSwingDecline(t, true)
	depth := max(latchDepth, swingdecline.ArmDepth)
	if depth >= lastDepthFills {
		t.Skip("swingdecline.ArmDepth reaches the last depth, where a new fill is never judged")
	}
	trade := latchLadder(depth)
	down := max(swingdecline.MinSmcTrendDown, 0)
	ai := latchMarket(t, trade, down, max(3, down))
	newest, gain := trade.PositionPrice, gainPrint(trade)
	pending := withRow(trade, PendingRow(newest, swingFinding(nil)), testutil.At("23:00:00"))
	reading := func(holds bool) swingReadFunc {
		return func([]series.Fill, series.Series, series.Trend) (swingdecline.Reading, bool) {
			return swingdecline.Reading{Holds: holds, Reasons: linearSwingReasons}, holds
		}
	}

	withSwingMemo(t, newSwingDeclineMemo(4, reading(true)))
	assertRow(t, Apply(withRestingOrder(trade), "", gain, ai), PendingRow(newest, indecisionFinding(nil), swingFinding(linearSwingReasons)))
	assertRow(t, Apply(withRestingOrder(pending), "", gain, ai), LatchedRow(newest, swingFinding(linearSwingReasons)))

	withSwingMemo(t, newSwingDeclineMemo(4, reading(false)))
	assertRow(t, Apply(withRestingOrder(trade), "", gain, ai), latchRowOf(t, trade, ai.SmartSell))
	assertRow(t, Apply(withRestingOrder(pending), "", gain, ai), latchRowOf(t, pending, ai.SmartSell))
}

// With the gate on and the trend meeting its need the tick writes the latch, then the gate's triggered row, then its activation.
func TestTheLatchComesBeforeTheGatesTriggeredRowOnTheTickItOpensAnEpisode(t *testing.T) {
	onlyTheFlatMarketLatch(t)
	withFlatMarket(t, true)
	trade := flatMarketLadder(false)
	if !earlyTakeProfitWatched(trade) {
		t.Skip("the early take profit does not watch the fixture ladder")
	}
	ai, newest := flatMarketMarket(trade), trade.PositionPrice
	latch, triggered := latchRowOf(t, trade, ai.SmartSell), triggeredBy(trade, ai)

	got := applyAtAGain(trade, "", ai)
	assertRows(t, got, latch, triggered)
	assertNoSale(t, got, "")
	loss := Apply(trade, "", newest, ai)
	assertActivated(t, loss, latch, triggered, FlatMarketActivatedRow(newest, -handLossPct(trade, newest)))

	written, _ := tickAtAGain(trade, "", testutil.At("18:00:00"), ai)
	if st := rebuildState(written); !st.earlyTakeProfit || !st.flatMarketActive || st.slowDeclinePending {
		t.Fatalf("the latch and the episode stand together, got %+v", st)
	}
	assertUntouched(t, Apply(withRestingOrder(written), "", gainPrint(trade), ai), "")
}

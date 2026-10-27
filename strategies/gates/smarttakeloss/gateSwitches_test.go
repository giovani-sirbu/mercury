package smarttakeloss

import (
	"slices"
	"testing"

	"github.com/giovani-sirbu/mercury/internal/testutil"
	"github.com/giovani-sirbu/mercury/strategies/gates/smarttakeloss/swingdecline"
)

// The flat market gate off opens no episode and sells nothing, at the cap or over it; its bars
// still latch the early take profit on their own row.
func TestTheFlatMarketGateOffLeavesTheLatchAlone(t *testing.T) {
	flatMarketAlone(t)
	start := flatMarketLadder(false)
	ai := flatMarketMarket(start)
	atTheCap, oneStepLower := flatMarketPrices(t, start)
	activation := FlatMarketStopLossActivatedRow(start.PositionPrice, -handLossPct(start, atTheCap))
	assertActivated(t, Apply(start, "", atTheCap, ai), triggeredBy(start, ai), activation)

	withFlatMarket(t, false)
	for _, price := range []float64{atTheCap, oneStepLower} {
		assertUntouched(t, Apply(start, "", price, ai), "")
	}
	withEarlyTakeProfit(t, true)
	withEarlyTakeProfitFlatMarket(t, true)
	got := Apply(start, "", atTheCap, ai)
	assertRow(t, got, latchRowOf(t, start, ai.SmartTakeLoss))
	assertNoSale(t, got, "")
}

// The swing decline off makes no ladder pending and sells nothing, at a gain or a loss; under
// earlytakeprofit.SwingDecline its reading still latches the early take profit on its own row on the tick it holds.
func TestTheSwingDeclineOffLatchesWithoutPending(t *testing.T) {
	withTheFeaturesOff(t)
	withEveryEarlyTakeProfitLatch(t)
	trade, block := stairFourth(), stairFourthBlock()
	reading, holds := readsAsOf(trade, block)
	needsAScenario(t, len(rebuildState(trade).fills) >= swingdecline.ArmDepth && holds, "a staircase the swing decline reads at the fourth fill")
	for _, price := range []float64{gainPrint(trade), aLossWithinTheCap(t, trade)} {
		after, got := engineTick(trade, "", price, stairOpen(26), withBlock(block))
		assertRow(t, got, LatchedRow(stairPrices[3], swingFinding(reading.Reasons)))
		assertNoSale(t, got, "")
		if st := rebuildState(after); st.slowDeclinePending || st.slowDeclineWatched || !st.earlyTakeProfit {
			t.Fatalf("at %v: the ladder is latched, not pending and not watched by the gate, got %+v", price, st)
		}
	}
}

// The linear decline on makes the ladder pending on its line, the latch joining the row, and the
// shared action activates the stopLoss on a real loss; a new fill on the line is confirmed.
func TestTheLinearDeclineOnMakesTheLadderPendingAndSells(t *testing.T) {
	onlyTheLinearDecline(t)
	withLinearDecline(t, true)
	withSellLossStartAtDepth(t, 0)
	trade, block := linearScene(linearTrendMet)
	fill := trade.PositionPrice
	pending := PendingRow(fill, linearFinding(linearReading(t, trade, block).Reasons), indecisionFinding(nil))

	if !Armed(trade) {
		t.Fatal("switched on, the linear decline arms the ladder")
	}
	after, got := tickAtAGain(trade, "", testutil.At("23:00:00"), withBlock(block))
	assertRow(t, got, pending)
	assertNoSale(t, got, "")
	if st := rebuildState(after); !st.slowDeclinePending || !slices.Equal(st.pendingFeatures, []string{FeatureLinearDecline}) || !st.earlyTakeProfit {
		t.Fatalf("the ladder is pending on the linear decline and latched, got %+v", st)
	}

	loss := aLossWithinTheCap(t, trade)
	assertActivated(t, Apply(trade, "", loss, withBlock(block)), pending, StopLossActivatedRow(fill, -handLossPct(trade, loss), FeatureLinearDecline))

	deeper := linearLadder(linearSlope, linearDepth+1)
	deeperBlock := linearBlock(linearSlope, linearDepth+1, linearTail, linearTrendMet)
	judged := withFill(withRows(linearLadder(linearSlope, linearDepth), pending), deeper.PositionPrice, rebuildState(deeper).lastFill().At)
	confirmed := ConfirmedRow(deeper.PositionPrice, linearFinding(linearReading(t, deeper, deeperBlock).Reasons))
	assertRow(t, applyAtAGain(judged, "", withBlock(deeperBlock)), confirmed)
}

// A pending event naming the linear decline folds as pending only while lineardecline.Enabled is
// on; off, the gate does not even watch the ladder for it.
func TestRebuildStateFoldsALinearDeclinePendingEventUnderItsSwitch(t *testing.T) {
	onlyTheLinearDecline(t)
	trade := linearLadder(linearSlope, linearDepth)
	pending := withRows(trade, PendingRow(trade.PositionPrice, linearFinding(nil)))
	withLinearDecline(t, true)
	if st := rebuildState(pending); !st.slowDeclinePending || !slices.Equal(st.pendingFeatures, []string{FeatureLinearDecline}) {
		t.Fatalf("switched on, the event makes the ladder pending on the linear decline, got %+v", st)
	}
	withLinearDecline(t, false)
	if st := rebuildState(pending); st.slowDeclinePending || st.slowDeclineWatched {
		t.Fatalf("switched off, the event makes nothing pending and the gate does not watch the ladder, got %+v", st)
	}
}

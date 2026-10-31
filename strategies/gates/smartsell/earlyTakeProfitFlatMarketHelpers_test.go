package smartsell

import (
	"testing"

	"github.com/giovani-sirbu/mercury/internal/testutil"
	"github.com/giovani-sirbu/mercury/strategies/gates/smartsell/earlytakeprofit"
	"github.com/giovani-sirbu/mercury/strategies/gates/smartsell/flatmarket"
	"github.com/giovani-sirbu/mercury/trades/aggragates"
)

// The latch on the flat market bars reads them alone (indecisionFlatMarketReads); its reading is the lowinterest
// package's, so these tests are about what the gate does with the answer. The fixtures follow the constants in force.

// latchDepth is the depth the latch is first read at: the arm depth of the early take profit, never under one fill.
var latchDepth = max(earlytakeprofit.ArmDepth, 1)

// latchTrends are Trend Direction rows the gate's own reading misses on: not served, none and one row of three read down.
var latchTrends = []struct {
	name        string
	down, valid int
}{{"not served", 0, 0}, {"0 of 3 rows down", 0, 3}, {"1 of 3 rows down", 1, 3}}

// withEarlyTakeProfitFlatMarket is withIndecision for the latch on the flat market bars.
func withEarlyTakeProfitFlatMarket(t *testing.T, on bool) {
	t.Helper()
	previous := earlyTakeProfitFlatMarket
	earlyTakeProfitFlatMarket = on
	t.Cleanup(func() { earlyTakeProfitFlatMarket = previous })
}

// onlyTheFlatMarketLatch runs the calling test with the early take profit and its latch on the bars on, every
// other reading and the flat market gate off, and the shared sale unbounded; it skips when no market holds.
func onlyTheFlatMarketLatch(t *testing.T) {
	t.Helper()
	if name := noFlatMarketMarket(testutil.LadderTrade(false).StrategyPair.StrategySettings[0].Percentage); name != "" {
		t.Skipf("%s leaves no market that holds for the fixture ladders to be read under", name)
	}
	withEarlyTakeProfit(t, true)
	withEarlyTakeProfitFlatMarket(t, true)
	withFlatMarket(t, false)
	withQuietDecline(t, false)
	withSwingDecline(t, false)
	withLinearDecline(t, false)
	withCapitalProtectionExit(t, false)
	withAllowedLoss(t, anyLoss)
	withSaleIgnoreLastDepths(t, 0)
}

// needsTheGateTrend skips a test that has a trend keep the gate from reading when the condition is off.
func needsTheGateTrend(t *testing.T) {
	t.Helper()
	if flatmarket.MinSmcTrend <= 0 {
		t.Skip("flatmarket.MinSmcTrend is zero or below: the condition is off and no trend keeps the gate from reading")
	}
}

// latchLadder is the long spot ladder depth fills deep on the w3s row, its newest fill at 17:38.
func latchLadder(depth int) aggragates.Trades {
	return testutil.LadderTrade(false, fills(depth, "17:38:00")...)
}

// latchMarket is the bars that hold for the ladder's base percentage under a Trend Direction row reading down on down of
// valid rows; it fails the test when the gate's own reading holds on a trend that misses its need.
func latchMarket(t *testing.T, trade aggragates.Trades, down, valid int) aggragates.AIIndicators {
	t.Helper()
	block := flatMarketBlock(basePercentage(trade), aggragates.SideLong)
	block.SmcTrendDown, block.SmcTrendUp, block.SmcTrendValid = down, 0, valid
	ai := withBlock(block)
	missed := flatmarket.MinSmcTrend > 0 && (valid <= 0 || down < flatmarket.MinSmcTrend)
	if missed && flatMarketReading(trade, block, ai).Holds {
		t.Fatalf("fixture drifted: %d of %d rows down must miss the gate's own reading", down, valid)
	}
	return ai
}

// latchRowOf is the row the latch writes for the ladder: the latched row at its newest fill naming what the bars read.
func latchRowOf(t *testing.T, trade aggragates.Trades, block aggragates.SmartSellIndicators) Row {
	t.Helper()
	bars, percentage := flatmarket.BarsOf(block), basePercentage(trade)
	reading := flatmarket.TriggerRangeOnBars(bars, percentage, aggragates.SideLong)
	if !reading.Holds {
		t.Fatalf("fixture drifted: the bars must hold, breaks %q", flatmarket.RangeBreaksOnBars(bars, percentage, aggragates.SideLong))
	}
	return LatchedRow(rebuildState(trade).lastFill().Price, Finding{Feature: FeatureFlatMarket, Reasons: reading.Reasons})
}

package smarttakeloss

import (
	"testing"
	"time"

	"github.com/giovani-sirbu/mercury/internal/testutil"
	"github.com/giovani-sirbu/mercury/strategies/gates/smarttakeloss/flatmarket"
	"github.com/giovani-sirbu/mercury/trades/aggragates"
)

// flatMarketLineAlone runs the test with the flat market gate on, a loss of one percent
// allowed and every other rule off, and fails when the market built for a long ladder does not hold.
func flatMarketLineAlone(t *testing.T) {
	t.Helper()
	probe := testutil.LadderTrade(false)
	market := flatMarketLineMarket(probe)
	if reading := flatMarketReading(probe, market.SmartTakeLoss, market); !reading.Holds {
		t.Fatalf("the constants of gates/smarttakeloss/noninterestarea leave no market that holds for the fixture ladders: %q",
			flatmarket.LineBreaks(flatmarket.BarsOf(market.SmartTakeLoss), basePercentage(probe), reading.Line.Side, reading.Line.Trend))
	}
	withFlatMarket(t, true)
	withAllowedLoss(t, 1)
	withQuietDecline(t, false)
	withSwingDecline(t, false)
	withLinearDecline(t, false)
	withEarlyTakeProfit(t, false)
	withCapitalProtectionExit(t, false)
}

// flatMarketLineBars is how many bars a fixture block serves: as many as sophos does and never
// fewer than the rule reads; flatMarketLineTrendRows the rows the fixture trend reads the side's
// way, every row of the dashboard on every read, so the shared sale reads its strictest rule (FullTrendTrigger).
var flatMarketLineBars = max(flatmarket.Window, flatmarket.MinBars)

const flatMarketLineTrendRows = 3

// flatMarketLineBlockOf is the block sophos serves for a ladder at percentage on side: opens and
// closes on a line from flatMarketOpen moving movePct over flatMarketLineBars hourly bars, and a
// trend on every dashboard row the side's way.
func flatMarketLineBlockOf(percentage float64, side string, movePct float64) aggragates.SmartTakeLossIndicators {
	block := aggragates.SmartTakeLossIndicators{}
	last := flatMarketLineBars - 1
	for index := range flatMarketLineBars {
		opened := testutil.At("00:00:00").Add(time.Duration(index) * time.Hour)
		closed := flatMarketOpen * (1 + movePct/100*float64(index)/float64(last))
		block.FlatMarketBarOpens = append(block.FlatMarketBarOpens, opened.UnixMilli())
		block.FlatMarketOpenPrices = append(block.FlatMarketOpenPrices, closed)
		block.FlatMarketCloses = append(block.FlatMarketCloses, closed)
	}
	block = withFullTrend(block, side)
	_ = percentage
	return block
}

// flatMarketLineMarket is the payload that holds for the trade: a flat line under its side's trend.
func flatMarketLineMarket(trade aggragates.Trades) aggragates.AIIndicators {
	return withBlock(flatMarketLineBlockOf(basePercentage(trade), aggragates.EntrySide(trade, aggragates.AIIndicators{}), 0))
}

// flatMarketLineAgainst is the payload that misses for the trade: a line moving well against its
// side, up for a long and down for an inverse.
func flatMarketLineAgainst(trade aggragates.Trades) aggragates.AIIndicators {
	move := 10.0
	if trade.Inverse {
		move = -move
	}
	return withBlock(flatMarketLineBlockOf(basePercentage(trade), aggragates.EntrySide(trade, aggragates.AIIndicators{}), move))
}

// flatMarketLineTriggeredBy is the triggered row the market opens the trade's episode with.
func flatMarketLineTriggeredBy(trade aggragates.Trades, ai aggragates.AIIndicators) Row {
	return FlatMarketTriggeredRow(trade.PositionPrice, flatMarketReading(trade, ai.SmartTakeLoss, ai).Reasons)
}

// flatMarketLineLatchedBy is the latched row the market's trigger writes on a long ladder.
func flatMarketLineLatchedBy(trade aggragates.Trades, ai aggragates.AIIndicators) Row {
	reading := flatMarketReading(trade, ai.SmartTakeLoss, ai)
	return buildRow(rowLatched, trade.PositionPrice, []Finding{{Feature: FeatureFlatMarket, Reasons: reading.Reasons}})
}

// deeperBy is the ladder with one more fill at price, an hour after its newest.
func deeperBy(trade aggragates.Trades, price float64) aggragates.Trades {
	newest := trade.History[len(trade.History)-1]
	trade.History = append(trade.History[:len(trade.History):len(trade.History)], aggragates.TradesHistory{
		Type: newest.Type, Quantity: newest.Quantity, Price: price, OrderId: newest.OrderId + 1, CreatedAt: newest.CreatedAt.Add(time.Hour),
	})
	trade.PositionPrice = price
	return trade
}

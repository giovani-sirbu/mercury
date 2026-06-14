package actions

import (
	"math"
	"testing"

	"github.com/giovani-sirbu/mercury/trades/aggragates"
)

func TestCalculateMinProfitSpotUsesMinNotionalPercentage(t *testing.T) {
	trade := aggragates.Trades{PositionPrice: 50}
	trade.StrategyPair.TradeFilters = aggragates.TradeFilters{MinNotional: 10}
	trade.StrategyPair.StrategySettings = []aggragates.StrategySettings{{Percentage: 2}}

	got := CalculateMinProfit(trade)
	const want = 0.2 // 10 * (2/100)
	if math.Abs(got-want) > 1e-9 {
		t.Errorf("CalculateMinProfit spot = %v, want %v", got, want)
	}
}

func TestCalculateMinProfitInverseDividesByPositionPrice(t *testing.T) {
	trade := aggragates.Trades{PositionPrice: 50, Inverse: true}
	trade.StrategyPair.TradeFilters = aggragates.TradeFilters{MinNotional: 10}
	trade.StrategyPair.StrategySettings = []aggragates.StrategySettings{{Percentage: 2}}

	got := CalculateMinProfit(trade)
	const want = 0.004 // (10 * 2/100) / 50
	if math.Abs(got-want) > 1e-9 {
		t.Errorf("CalculateMinProfit inverse = %v, want %v", got, want)
	}
}

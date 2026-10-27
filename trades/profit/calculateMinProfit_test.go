package profit

import (
	"math"
	"testing"

	"github.com/giovani-sirbu/mercury/trades/aggragates"
)

// A normal trade's floor is the held depth's share of what its entries cost:
// 100 USDT of entries on a ladder of 4 depths at a percentage of 2 must close at
// least 2 / 4 = 0.5% over its average entry price, which is 0.5 USDT. The pair's
// minimum notional is set to what the old flat floor was taken of (10 * 2 / 100 =
// 0.2) and the position price stands away from the entry: neither plays a part.
func TestCalculateMinProfitSpotIsTheDepthShareOfWhatTheEntriesCost(t *testing.T) {
	trade := aggragates.Trades{PositionPrice: 80}
	trade.History = []aggragates.TradesHistory{
		{Type: "BUY", Quantity: 2, Price: 50, OrderId: 1},
	}
	trade.StrategyPair.TradeFilters = aggragates.TradeFilters{MinNotional: 10}
	trade.StrategyPair.StrategySettings = []aggragates.StrategySettings{{Percentage: 2, Depths: 4}}

	got := CalculateMinProfit(trade)
	const want = 0.5 // (2 * 50) * (2 / 4 / 100)
	if math.Abs(got-want) > 1e-9 {
		t.Errorf("CalculateMinProfit spot = %v, want %v", got, want)
	}
}

// An inverse trade counts its profit in base, so its floor is the same share of
// the base its entries sold: 4 base sold on a ladder of 4 depths at a percentage
// of 2 must book at least 4 * 2 / 4 / 100 = 0.02 base. The position price, which
// the old floor was divided by, plays no part, so the answer is the same
// wherever it stands.
func TestCalculateMinProfitInverseIsTheDepthShareOfTheBaseItsEntriesSold(t *testing.T) {
	for _, positionPrice := range []float64{50, 5000} {
		trade := aggragates.Trades{PositionPrice: positionPrice, Inverse: true}
		trade.History = []aggragates.TradesHistory{
			{Type: "SELL", Quantity: 4, Price: 50, OrderId: 1},
		}
		trade.StrategyPair.TradeFilters = aggragates.TradeFilters{MinNotional: 10}
		trade.StrategyPair.StrategySettings = []aggragates.StrategySettings{{Percentage: 2, Depths: 4}}

		got := CalculateMinProfit(trade)
		const want = 0.02 // 4 * (2 / 4 / 100)
		if math.Abs(got-want) > 1e-9 {
			t.Errorf("CalculateMinProfit inverse at position price %v = %v, want %v", positionPrice, got, want)
		}
	}
}

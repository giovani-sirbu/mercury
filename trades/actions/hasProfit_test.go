package actions

import (
	"math"
	"testing"

	"github.com/giovani-sirbu/mercury/events"
	"github.com/giovani-sirbu/mercury/trades/aggragates"
)

func TestHasProfitSucceedsWhenProfitMeetsThreshold(t *testing.T) {
	trade := aggragates.Trades{Symbol: "BTC/USDT", PositionPrice: 200}
	// The entry side is written the way the exchange writes it: the ladder folds
	// the minimum profit reads count an entry by that exact side, so a lowercase
	// row is no entry and would leave the floor at nothing.
	trade.History = []aggragates.TradesHistory{
		{Type: "BUY", Quantity: 1, Price: 100, OrderId: 1},
	}
	trade.StrategyPair.TradeFilters = aggragates.TradeFilters{LotSize: 2, MinNotional: 10, PriceFilter: 2}
	trade.StrategyPair.StrategySettings = []aggragates.StrategySettings{{Percentage: 2, Depths: 6, Tolerance: 0}}

	event := events.Events{Trade: trade}

	got, err := HasProfit(event)
	if err != nil {
		t.Fatalf("expected profit threshold to be met, got error %v", err)
	}
	// quantity = 1, sell @200 - buy @100 = 100 net profit (no fees)
	if got.Trade.Profit <= 0 {
		t.Errorf("expected positive profit recorded, got %v", got.Trade.Profit)
	}
}

func TestHasProfitFailsWhenProfitBelowMinimum(t *testing.T) {
	trade := aggragates.Trades{Symbol: "BTC/USDT", PositionPrice: 100}
	trade.History = []aggragates.TradesHistory{
		{Type: "BUY", Quantity: 1, Price: 100, OrderId: 1},
	}
	trade.StrategyPair.TradeFilters = aggragates.TradeFilters{LotSize: 2, MinNotional: 10, PriceFilter: 2}
	trade.StrategyPair.StrategySettings = []aggragates.StrategySettings{{Percentage: 2, Depths: 6, Tolerance: 0}}

	event := events.Events{Trade: trade}

	_, err := HasProfit(event)
	if err == nil {
		t.Fatal("expected error when simulated profit is below min profit")
	}
}

// hasProfitTrade is the user's own example on a BTC/USDT ladder: 6 depths at a
// take profit percentage of 2.2 that has filled one entry of 10 at 1000, and
// closes at closePrice on the pair's two-decimal price grid. What the entry cost
// is 10000, so the floor the close must book is 10000 * 2.2 / 6 / 100 =
// 36.666..., which is 0.3666...% over the average entry price. The entry row
// carries no fee, so GetFees and UnembodiedOpeningFees charge nothing and the
// floor alone decides the gate.
func hasProfitTrade(closePrice float64) aggragates.Trades {
	trade := aggragates.Trades{Symbol: "BTC/USDT", PositionPrice: closePrice}
	trade.History = []aggragates.TradesHistory{
		{Type: "BUY", Quantity: 10, Price: 1000, OrderId: 1},
	}
	trade.StrategyPair.TradeFilters = aggragates.TradeFilters{LotSize: 2, MinNotional: 10, PriceFilter: 2}
	trade.StrategyPair.StrategySettings = []aggragates.StrategySettings{{Percentage: 2.2, Depths: 6, Tolerance: 0}}

	return trade
}

// The gate holds the close to the floor to the price tick: the first price on
// the grid over 0.3666...% above the average entry price books the floor and is
// taken, and the price one tick under it books less and is refused. The net
// profit recorded on both sides is the entry's 10 units sold at the close price
// over what they cost, with nothing taken off for fees, so the floor and no fee
// is what separates them.
func TestHasProfitHoldsTheCloseToTheDepthShareFloor(t *testing.T) {
	cases := []struct {
		name       string
		closePrice float64
		wantErr    bool
	}{
		{"the first price over the floor passes", 1003.67, false}, // 10 * 3.67 = 36.7, over 36.666...
		{"the price under it is refused", 1003.66, true},          // 10 * 3.66 = 36.6, under 36.666...
	}

	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			got, err := HasProfit(events.Events{Trade: hasProfitTrade(c.closePrice)})

			if (err != nil) != c.wantErr {
				t.Fatalf("closing at %v: error = %v, want an error: %t", c.closePrice, err, c.wantErr)
			}
			if want := 10 * (c.closePrice - 1000); math.Abs(got.Trade.Profit-want) > 1e-6 {
				t.Errorf("net profit = %v, want %v: the fee must not decide this case", got.Trade.Profit, want)
			}
		})
	}
}

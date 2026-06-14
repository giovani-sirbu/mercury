package actions

import (
	"math"
	"testing"

	"github.com/giovani-sirbu/mercury/events"
	"github.com/giovani-sirbu/mercury/trades/aggragates"
)

func TestSubtractToleranceFromPriceSpotReducesPriceByPercentage(t *testing.T) {
	trade := aggragates.Trades{PositionPrice: 100}
	trade.StrategyPair.StrategySettings = []aggragates.StrategySettings{{Tolerance: 5}}
	trade.StrategyPair.TradeFilters = aggragates.TradeFilters{PriceFilter: 2}

	got := subtractToleranceFromPrice(trade)
	if math.Abs(got-95) > 1e-9 {
		t.Errorf("spot tolerance price = %v, want 95", got)
	}
}

func TestSubtractToleranceFromPriceInverseRaisesPriceByPercentage(t *testing.T) {
	trade := aggragates.Trades{PositionPrice: 100, Inverse: true}
	trade.StrategyPair.StrategySettings = []aggragates.StrategySettings{{Tolerance: 5}}
	trade.StrategyPair.TradeFilters = aggragates.TradeFilters{PriceFilter: 2}

	got := subtractToleranceFromPrice(trade)
	if math.Abs(got-105) > 1e-9 {
		t.Errorf("inverse tolerance price = %v, want 105", got)
	}
}

func TestHasProfitSucceedsWhenProfitMeetsThreshold(t *testing.T) {
	trade := aggragates.Trades{Symbol: "BTC/USDT", PositionPrice: 200}
	trade.History = []aggragates.TradesHistory{
		{Type: "buy", Quantity: 1, Price: 100},
	}
	trade.StrategyPair.TradeFilters = aggragates.TradeFilters{LotSize: 2, MinNotional: 10, PriceFilter: 2}
	trade.StrategyPair.StrategySettings = []aggragates.StrategySettings{{Percentage: 2, Tolerance: 0}}

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
		{Type: "buy", Quantity: 1, Price: 100},
	}
	trade.StrategyPair.TradeFilters = aggragates.TradeFilters{LotSize: 2, MinNotional: 10, PriceFilter: 2}
	trade.StrategyPair.StrategySettings = []aggragates.StrategySettings{{Percentage: 2, Tolerance: 0}}

	event := events.Events{Trade: trade}

	_, err := HasProfit(event)
	if err == nil {
		t.Fatal("expected error when simulated profit is below min profit")
	}
}

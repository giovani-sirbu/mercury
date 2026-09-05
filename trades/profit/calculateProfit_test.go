package profit

import (
	"math"
	"testing"

	"github.com/giovani-sirbu/mercury/events"
	"github.com/giovani-sirbu/mercury/trades/aggragates"
)

func TestCalculateProfitSubtractsFeesFromGrossProfit(t *testing.T) {
	trade := aggragates.Trades{
		Symbol: "BTC/USDT",
		History: []aggragates.TradesHistory{
			{Type: "buy", Quantity: 1, Price: 100, Fees: []aggragates.TradesFees{{Asset: "USDT", Fee: 0.5}}},
			{Type: "sell", Quantity: 1, Price: 120, Fees: []aggragates.TradesFees{{Asset: "USDT", Fee: 0.6}}},
		},
	}
	trade.StrategyPair.TradeFilters = aggragates.TradeFilters{PriceFilter: 2}

	got := CalculateProfit(events.Events{Trade: trade})
	// gross = 120 - 100 = 20, fees in quote = 0.5 + 0.6 = 1.1, net = 18.9
	const want = 18.9
	if math.Abs(got-want) > 1e-9 {
		t.Errorf("CalculateProfit = %v, want %v", got, want)
	}
}

func TestCalculateProfitReturnsZeroWhenNoTradeHistory(t *testing.T) {
	trade := aggragates.Trades{Symbol: "BTC/USDT"}
	trade.StrategyPair.TradeFilters = aggragates.TradeFilters{PriceFilter: 2}

	got := CalculateProfit(events.Events{Trade: trade})
	if got != 0 {
		t.Errorf("expected 0 profit with empty history, got %v", got)
	}
}

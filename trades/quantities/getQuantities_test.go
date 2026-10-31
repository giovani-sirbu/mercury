package quantities

import (
	"math"
	"testing"

	"github.com/giovani-sirbu/mercury/events"
	"github.com/giovani-sirbu/mercury/trades/aggragates"
)

func TestGetQuantitiesSpotReturnsBuyMinusSellAndSellType(t *testing.T) {
	trade := aggragates.Trades{Symbol: "BTC/USDT"}
	trade.History = []aggragates.TradesHistory{
		{Type: "buy", Quantity: 2, Price: 100},
		{Type: "sell", Quantity: 0.5, Price: 110},
	}
	trade.StrategyPair.TradeFilters = aggragates.TradeFilters{LotSize: 2}

	quantity, historyType := GetQuantities(events.Events{Trade: trade})

	if math.Abs(quantity-1.5) > 1e-9 {
		t.Errorf("quantity = %v, want 1.5", quantity)
	}
	if historyType != "sell" {
		t.Errorf("historyType = %q, want sell", historyType)
	}
}

func TestGetQuantitiesInverseReturnsBuyTypeAndQuantityDividedByPositionPrice(t *testing.T) {
	trade := aggragates.Trades{Symbol: "BTC/USDT", Inverse: true, PositionPrice: 100}
	trade.History = []aggragates.TradesHistory{
		{Type: "sell", Quantity: 2, Price: 100},
		{Type: "buy", Quantity: 1, Price: 90},
	}
	trade.StrategyPair.TradeFilters = aggragates.TradeFilters{LotSize: 2}

	quantity, historyType := GetQuantities(events.Events{Trade: trade})

	// sellTotal = 2*100 = 200, buyTotal = 1*90 = 90, diff = 110, /100 = 1.10
	if math.Abs(quantity-1.1) > 1e-9 {
		t.Errorf("quantity = %v, want 1.1", quantity)
	}
	if historyType != "buy" {
		t.Errorf("historyType = %q, want buy", historyType)
	}
}

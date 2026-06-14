package actions

import (
	"testing"

	"github.com/giovani-sirbu/mercury/events"
	"github.com/giovani-sirbu/mercury/trades/aggragates"
)

func newRegulateTrade(inverse bool, positionPrice float64, lastTradePrice float64) aggragates.Trades {
	historyType := "BUY"
	if inverse {
		historyType = "SELL"
	}
	trade := aggragates.Trades{
		Symbol:        "BTC/USDT",
		PositionPrice: positionPrice,
		Inverse:       inverse,
	}
	trade.History = []aggragates.TradesHistory{{Type: historyType, Price: lastTradePrice, Quantity: 1}}
	trade.StrategyPair.StrategySettings = []aggragates.StrategySettings{{Percentage: 10}}
	return trade
}

func TestRegulatePriceChangeSpotAllowsPriceBelowAdjustedThreshold(t *testing.T) {
	trade := newRegulateTrade(false, 89, 100) // threshold = 100 - 10% = 90, current 89 < 90 OK
	event := events.Events{Trade: trade}

	_, err := RegulatePriceChange(event)
	if err != nil {
		t.Fatalf("expected no error, got %v", err)
	}
}

func TestRegulatePriceChangeSpotRejectsPriceAboveAdjustedThreshold(t *testing.T) {
	trade := newRegulateTrade(false, 95, 100) // threshold = 90, current 95 > 90 -> reject
	event := events.Events{Trade: trade}

	_, err := RegulatePriceChange(event)
	if err == nil {
		t.Fatal("expected error when current price exceeds adjusted last position price")
	}
}

func TestRegulatePriceChangeInverseAllowsPriceAboveAdjustedThreshold(t *testing.T) {
	trade := newRegulateTrade(true, 115, 100) // threshold = 100 + 10% = 110, current 115 > 110 OK
	event := events.Events{Trade: trade}

	_, err := RegulatePriceChange(event)
	if err != nil {
		t.Fatalf("expected no error, got %v", err)
	}
}

func TestRegulatePriceChangeInverseRejectsPriceBelowAdjustedThreshold(t *testing.T) {
	trade := newRegulateTrade(true, 105, 100) // threshold = 110, current 105 < 110 -> reject
	event := events.Events{Trade: trade}

	_, err := RegulatePriceChange(event)
	if err == nil {
		t.Fatal("expected error when current price is below the adjusted threshold for inverse trade")
	}
}

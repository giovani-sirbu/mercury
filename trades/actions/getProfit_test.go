package actions

import (
	"math"
	"testing"

	"github.com/giovani-sirbu/mercury/trades/aggragates"
)

func TestGetProfitInBaseSumsQuantitiesOnly(t *testing.T) {
	history := []aggragates.TradesHistory{
		{Type: "buy", Quantity: 2, Price: 100},
		{Type: "sell", Quantity: 1, Price: 250},
	}

	sellTotal, buyTotal := GetProfitInBase(history)
	if buyTotal != 2 {
		t.Errorf("buyTotal = %v, want 2", buyTotal)
	}
	if sellTotal != 1 {
		t.Errorf("sellTotal = %v, want 1", sellTotal)
	}
}

func TestGetProfitSpotSubtractsBuyTotalFromSellTotalInQuote(t *testing.T) {
	trade := aggragates.Trades{
		Symbol: "BTC/USDT",
		History: []aggragates.TradesHistory{
			{Type: "buy", Quantity: 2, Price: 100},
			{Type: "sell", Quantity: 2, Price: 150},
		},
	}

	profit := GetProfit(trade)
	const want = 100.0 // (2*150) - (2*100)
	if math.Abs(profit-want) > 1e-9 {
		t.Errorf("GetProfit spot = %v, want %v", profit, want)
	}
}

func TestGetProfitInverseSubtractsSellFromBuyInBase(t *testing.T) {
	trade := aggragates.Trades{
		Symbol:  "BTC/USDT",
		Inverse: true,
		History: []aggragates.TradesHistory{
			{Type: "sell", Quantity: 2, Price: 100},
			{Type: "buy", Quantity: 3, Price: 90},
		},
	}

	profit := GetProfit(trade)
	const want = 1.0 // 3 - 2
	if math.Abs(profit-want) > 1e-9 {
		t.Errorf("GetProfit inverse = %v, want %v", profit, want)
	}
}

func TestGetProfitIncludesDustInSpot(t *testing.T) {
	trade := aggragates.Trades{
		Symbol:        "BTC/USDT",
		PositionPrice: 100,
		Dust:          0.5,
		History: []aggragates.TradesHistory{
			{Type: "buy", Quantity: 1, Price: 100},
			{Type: "sell", Quantity: 1, Price: 110},
		},
	}

	profit := GetProfit(trade)
	const want = 60.0 // (110 - 100) + (0.5 * 100)
	if math.Abs(profit-want) > 1e-9 {
		t.Errorf("GetProfit with dust = %v, want %v", profit, want)
	}
}

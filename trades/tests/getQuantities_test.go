package tests

import (
	"testing"

	"github.com/giovani-sirbu/mercury/trades/actions"
	"github.com/giovani-sirbu/mercury/trades/aggragates"
)

func TestGetQuantities(t *testing.T) {
	tests := []struct {
		name         string
		trade        aggragates.Trades
		wantQty      float64
		wantType     string
	}{
		{
			"NormalOnlyBuys",
			MakeTrade("ATOM/USDT", 5, false, []aggragates.TradesHistory{
				{Quantity: 2, Price: 5, Type: "BUY"},
				{Quantity: 4, Price: 5, Type: "BUY"},
			}),
			6.0, "sell",
		},
		{
			"NormalBuysAndSells",
			MakeTrade("ATOM/USDT", 6, false, []aggragates.TradesHistory{
				{Quantity: 10, Price: 5, Type: "BUY"},
				{Quantity: 3, Price: 6, Type: "SELL"},
			}),
			7.0, "sell",
		},
		{
			"EmptyHistory",
			MakeTrade("ATOM/USDT", 5, false, nil),
			0.0, "sell",
		},
		{
			// Inverse: sellTotal = 10*5 + 20*4 = 130, buyTotal = 0
			// qty = (sellTotal - buyTotal) / positionPrice = 130 / 5 = 26
			"InverseOnlySells",
			MakeTrade("ATOM/USDT", 5, true, []aggragates.TradesHistory{
				{Quantity: 10, Price: 5, Type: "SELL"},
				{Quantity: 20, Price: 4, Type: "SELL"},
			}),
			26.0, "buy",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			event := MakeEvent(tt.trade, "USDT", "1000", []string{})
			gotQty, gotType := actions.GetQuantities(event)
			AssertFloatEqual(t, gotQty, tt.wantQty, 1e-8, "GetQuantities qty")
			if gotType != tt.wantType {
				t.Errorf("GetQuantities type: got %q, want %q", gotType, tt.wantType)
			}
		})
	}
}

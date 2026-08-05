package tests

import (
	"testing"

	"github.com/giovani-sirbu/mercury/trades/actions"
	"github.com/giovani-sirbu/mercury/trades/aggragates"
)

func TestGetFees(t *testing.T) {
	tests := []struct {
		name  string
		trade aggragates.Trades
		want  float64
	}{
		{
			"NoFees",
			aggragates.Trades{
				Symbol: "ATOM/USDT",
				History: []aggragates.TradesHistory{
					{Quantity: 10, Price: 5, Type: "BUY", Fees: []aggragates.TradesFees{}},
				},
			},
			0.0,
		},
		{
			// Fee in base asset (ATOM). feesInQuote = fee * price = 0.1 * 5 = 0.5
			"BaseAssetFees",
			aggragates.Trades{
				Symbol: "ATOM/USDT",
				History: []aggragates.TradesHistory{
					{Quantity: 10, Price: 5, Type: "BUY", Fees: []aggragates.TradesFees{
						{Asset: "ATOM", Fee: 0.1},
					}},
				},
			},
			0.5,
		},
		{
			// Fee in quote asset (USDT). feesInQuote = 0.5 directly
			"QuoteAssetFees",
			aggragates.Trades{
				Symbol: "ATOM/USDT",
				History: []aggragates.TradesHistory{
					{Quantity: 10, Price: 5, Type: "BUY", Fees: []aggragates.TradesFees{
						{Asset: "USDT", Fee: 0.5},
					}},
				},
			},
			0.5,
		},
		{
			// Inverse returns feesInBase. Fee in USDT (quote): feesInBase += fee/price = 0.5/5 = 0.1
			"InverseReturnsBase",
			aggragates.Trades{
				Symbol:  "ATOM/USDT",
				Inverse: true,
				History: []aggragates.TradesHistory{
					{Quantity: 10, Price: 5, Type: "SELL", Fees: []aggragates.TradesFees{
						{Asset: "USDT", Fee: 0.5},
					}},
				},
			},
			0.1,
		},
		{
			// Zero fee skipped
			"ZeroFeeSkipped",
			aggragates.Trades{
				Symbol: "ATOM/USDT",
				History: []aggragates.TradesHistory{
					{Quantity: 10, Price: 5, Type: "BUY", Fees: []aggragates.TradesFees{
						{Asset: "ATOM", Fee: 0},
					}},
				},
			},
			0.0,
		},
		{
			// Negative fee skipped
			"NegativeFeeSkipped",
			aggragates.Trades{
				Symbol: "ATOM/USDT",
				History: []aggragates.TradesHistory{
					{Quantity: 10, Price: 5, Type: "BUY", Fees: []aggragates.TradesFees{
						{Asset: "ATOM", Fee: -0.1},
					}},
				},
			},
			0.0,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			event := MakeEvent(tt.trade, "USDT", "1000", []string{})
			got := actions.GetFees(event)
			AssertFloatEqual(t, got, tt.want, 1e-8, "GetFees")
		})
	}
}

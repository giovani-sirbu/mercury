package profit_test

import (
	"testing"

	"github.com/giovani-sirbu/mercury/trades/aggragates"
	"github.com/giovani-sirbu/mercury/trades/internal/testutil"
	"github.com/giovani-sirbu/mercury/trades/profit"
)

func TestGetProfit(t *testing.T) {
	tests := []struct {
		name  string
		trade aggragates.Trades
		want  float64
	}{
		{
			"NormalBuyAndSell",
			aggragates.Trades{
				History: []aggragates.TradesHistory{
					{Quantity: 10, Price: 5, Type: "BUY"},
					{Quantity: 10, Price: 6, Type: "SELL"},
				},
			},
			10.0, // (10*6) - (10*5) = 10
		},
		{
			"NormalBuyOnly",
			aggragates.Trades{
				History: []aggragates.TradesHistory{
					{Quantity: 10, Price: 5, Type: "BUY"},
				},
			},
			-50.0, // 0 - (10*5) = -50
		},
		{
			// Inverse: qty only (no price multiplication). profit = buyTotal - sellTotal
			"InverseBreakEven",
			aggragates.Trades{
				Inverse: true,
				History: []aggragates.TradesHistory{
					{Quantity: 10, Price: 5, Type: "SELL"},
					{Quantity: 10, Price: 4, Type: "BUY"},
				},
			},
			0.0, // buyTotal(10) - sellTotal(10) = 0
		},
		{
			"InverseWithProfit",
			aggragates.Trades{
				Inverse: true,
				History: []aggragates.TradesHistory{
					{Quantity: 100, Price: 0.05, Type: "SELL"},
					{Quantity: 120, Price: 0.04, Type: "BUY"},
				},
			},
			20.0, // buyTotal(120) - sellTotal(100) = 20
		},
		{
			"WithDust",
			aggragates.Trades{
				PositionPrice: 6,
				Dust:          0.5,
				History: []aggragates.TradesHistory{
					{Quantity: 10, Price: 5, Type: "BUY"},
					{Quantity: 9, Price: 6, Type: "SELL"},
				},
			},
			7.0, // (9*6) - (10*5) + (0.5*6) = 54 - 50 + 3 = 7
		},
		{
			"EmptyHistory",
			aggragates.Trades{},
			0.0,
		},
		{
			"MultipleBuysOneSell",
			aggragates.Trades{
				History: []aggragates.TradesHistory{
					{Quantity: 2, Price: 5, Type: "BUY"},
					{Quantity: 4, Price: 5, Type: "BUY"},
					{Quantity: 6, Price: 6, Type: "SELL"},
				},
			},
			6.0, // (6*6) - (2*5 + 4*5) = 36 - 30 = 6
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := profit.GetProfit(tt.trade)
			testutil.AssertFloatEqual(t, got, tt.want, 1e-8, "GetProfit")
		})
	}
}

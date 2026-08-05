package tests

import (
	"testing"

	"github.com/giovani-sirbu/mercury/trades/actions"
	"github.com/giovani-sirbu/mercury/trades/aggragates"
)

func TestCalculateMinOrderQty(t *testing.T) {
	tests := []struct {
		name  string
		trade aggragates.Trades
		want  float64
	}{
		{
			"Normal",
			aggragates.Trades{
				PositionPrice: 10,
				StrategyPair: aggragates.StrategiesPairs{
					TradeFilters: aggragates.TradeFilters{MinNotional: 5, LotSize: 2},
				},
			},
			0.51, // 5/10 + 0.01 = 0.51
		},
		{
			"Inverse",
			aggragates.Trades{
				Inverse:       true,
				PositionPrice: 10,
				StrategyPair: aggragates.StrategiesPairs{
					TradeFilters: aggragates.TradeFilters{MinNotional: 5, LotSize: 2},
				},
			},
			5.0, // MinNotional directly for inverse
		},
		{
			"ZeroMinNotional",
			aggragates.Trades{
				PositionPrice: 10,
				StrategyPair: aggragates.StrategiesPairs{
					TradeFilters: aggragates.TradeFilters{MinNotional: 0, LotSize: 2},
				},
			},
			0,
		},
		{
			"ZeroLotSize",
			aggragates.Trades{
				PositionPrice: 10,
				StrategyPair: aggragates.StrategiesPairs{
					TradeFilters: aggragates.TradeFilters{MinNotional: 5, LotSize: 0},
				},
			},
			0,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := actions.CalculateMinOrderQty(tt.trade)
			AssertFloatEqual(t, got, tt.want, 1e-10, "CalculateMinOrderQty")
		})
	}
}

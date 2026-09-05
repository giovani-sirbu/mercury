package quantities_test

import (
	"testing"

	"github.com/giovani-sirbu/mercury/trades/aggragates"
	"github.com/giovani-sirbu/mercury/trades/internal/testutil"
	"github.com/giovani-sirbu/mercury/trades/quantities"
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
			// Inverse orders are SELLs with base-denominated quantity, so the
			// exchange minimum is MinNotional/price; the lot-step pad applies
			// only to spot buys.
			0.5,
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
			got := quantities.CalculateMinOrderQty(tt.trade)
			testutil.AssertFloatEqual(t, got, tt.want, 1e-10, "CalculateMinOrderQty")
		})
	}
}

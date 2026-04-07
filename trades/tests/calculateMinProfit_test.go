package tests

import (
	"testing"

	"github.com/giovani-sirbu/mercury/trades/actions"
	"github.com/giovani-sirbu/mercury/trades/aggragates"
)

func TestCalculateMinProfit(t *testing.T) {
	tests := []struct {
		name string
		trade aggragates.Trades
		want  float64
	}{
		{
			"Normal",
			aggragates.Trades{
				StrategyPair: aggragates.StrategiesPairs{
					TradeFilters:     aggragates.TradeFilters{MinNotional: 5},
					StrategySettings: []aggragates.StrategySettings{{Percentage: 2}},
				},
			},
			0.1, // 5 * (2/100) = 0.1
		},
		{
			"Inverse",
			aggragates.Trades{
				Inverse:       true,
				PositionPrice: 10,
				StrategyPair: aggragates.StrategiesPairs{
					TradeFilters:     aggragates.TradeFilters{MinNotional: 5},
					StrategySettings: []aggragates.StrategySettings{{Percentage: 2}},
				},
			},
			0.01, // 5 * (2/100) / 10 = 0.01
		},
		{
			"ZeroPercentage",
			aggragates.Trades{
				StrategyPair: aggragates.StrategiesPairs{
					TradeFilters:     aggragates.TradeFilters{MinNotional: 5},
					StrategySettings: []aggragates.StrategySettings{{Percentage: 0}},
				},
			},
			0.0,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := actions.CalculateMinProfit(tt.trade)
			AssertFloatEqual(t, got, tt.want, 1e-10, "CalculateMinProfit")
		})
	}
}

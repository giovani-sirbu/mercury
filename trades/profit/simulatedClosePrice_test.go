package profit

import (
	"math"
	"testing"

	"github.com/giovani-sirbu/mercury/trades/aggragates"
)

func TestSubtractToleranceFromPriceSpotReducesPriceByPercentage(t *testing.T) {
	trade := aggragates.Trades{PositionPrice: 100}
	trade.StrategyPair.StrategySettings = []aggragates.StrategySettings{{Tolerance: 5}}
	trade.StrategyPair.TradeFilters = aggragates.TradeFilters{PriceFilter: 2}

	got := subtractToleranceFromPrice(trade)
	if math.Abs(got-95) > 1e-9 {
		t.Errorf("spot tolerance price = %v, want 95", got)
	}
}

func TestSubtractToleranceFromPriceInverseRaisesPriceByPercentage(t *testing.T) {
	trade := aggragates.Trades{PositionPrice: 100, Inverse: true}
	trade.StrategyPair.StrategySettings = []aggragates.StrategySettings{{Tolerance: 5}}
	trade.StrategyPair.TradeFilters = aggragates.TradeFilters{PriceFilter: 2}

	got := subtractToleranceFromPrice(trade)
	if math.Abs(got-105) > 1e-9 {
		t.Errorf("inverse tolerance price = %v, want 105", got)
	}
}

// TestSimulatedClosePrice_HaircutOnlyWhileArming pins the rule that the
// tolerance haircut forecasts exactly one move — the drop between arming a
// takeProfit and the exit the strategy triggers `tolerance` below that anchor.
// Charging it again in a chain that runs Sell on the same pass double-counted
// it, so an exit needed 2*tolerance + fees of headroom over average cost while
// the ladder only builds one; deep depths then re-armed into `buy` instead of
// closing a position that was in fact profitable.
func TestSimulatedClosePrice_HaircutOnlyWhileArming(t *testing.T) {
	newTrade := func(positionType string) aggragates.Trades {
		return aggragates.Trades{
			Symbol:        "BTC/USDT",
			PositionType:  positionType,
			PositionPrice: 92724.70,
			StrategyPair: aggragates.StrategiesPairs{
				StrategySettings: []aggragates.StrategySettings{{Percentage: 2.5, Tolerance: 0.25}},
				TradeFilters:     aggragates.TradeFilters{PriceFilter: 2},
			},
		}
	}

	// 92724.70 * (1 - 0.25/100), floored to the pair's price filter.
	const haircut = 92492.88

	arming := []string{"takeProfit"}
	for _, positionType := range arming {
		trade := newTrade(positionType)
		if got := SimulatedClosePrice(trade); got != haircut {
			t.Errorf("%s arms a later exit, want haircut price %f, got %f", positionType, haircut, got)
		}
	}

	closing := []string{"sell", "sellParent", "sellLoss"}
	for _, positionType := range closing {
		trade := newTrade(positionType)
		if got := SimulatedClosePrice(trade); got != trade.PositionPrice {
			t.Errorf("%s closes in this chain and Sell submits at PositionPrice, want %f, got %f",
				positionType, trade.PositionPrice, got)
		}
	}
}

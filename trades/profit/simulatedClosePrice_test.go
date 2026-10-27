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
	if math.Abs(got-90) > 1e-9 {
		t.Errorf("spot tolerance price = %v, want 90", got)
	}
}

func TestSubtractToleranceFromPriceInverseRaisesPriceByPercentage(t *testing.T) {
	trade := aggragates.Trades{PositionPrice: 100, Inverse: true}
	trade.StrategyPair.StrategySettings = []aggragates.StrategySettings{{Tolerance: 5}}
	trade.StrategyPair.TradeFilters = aggragates.TradeFilters{PriceFilter: 2}

	got := subtractToleranceFromPrice(trade)
	if math.Abs(got-110) > 1e-9 {
		t.Errorf("inverse tolerance price = %v, want 110", got)
	}
}

// TestSimulatedClosePrice_HaircutOnlyWhileArming pins the haircut to the arming chain: a chain that sells on
// this pass is valued at its own price.
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

	// 92724.70 * (1 - 2*0.25/100), floored to the pair's price filter.
	const haircut = 92261.07

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

// TestSimulatedClosePrice_ArmLeavesRoomForTheFirstPrintUnderTheSaleLine is trade 172523 (HBAR, tolerance 0.1, prices
// on 0.0001): armed at 0.1072 the sale fires at 0.1070, under the min profit price 0.1070032, so 0.1072 must not arm.
func TestSimulatedClosePrice_ArmLeavesRoomForTheFirstPrintUnderTheSaleLine(t *testing.T) {
	const minSellPrice = 0.1070032
	armAt := func(anchor float64) float64 {
		trade := aggragates.Trades{PositionType: "takeProfit", PositionPrice: anchor}
		trade.StrategyPair.StrategySettings = []aggragates.StrategySettings{{Tolerance: 0.1}}
		trade.StrategyPair.TradeFilters = aggragates.TradeFilters{PriceFilter: 5}
		return SimulatedClosePrice(trade)
	}

	if got := armAt(0.1072); got >= minSellPrice {
		t.Errorf("anchor 0.1072 valued at %v, want under %v: its sale lands at 0.1070", got, minSellPrice)
	}
	if got := armAt(0.1073); got < minSellPrice {
		t.Errorf("anchor 0.1073 valued at %v, want at least %v: its sale lands at 0.1071", got, minSellPrice)
	}
}

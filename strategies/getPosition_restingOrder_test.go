package strategies

import (
	"testing"

	"github.com/giovani-sirbu/mercury/trades/aggragates"
)

// TestGetPositionBuyCancelsARunawayRestingAdd pins the `buy` row's resting-order
// head (buyLogic, byte for byte the row of the three engine tables). Trade
// 211288 (backtest 349, HBAR, Oct 10 2025): the depth 6 add rested at 0.0732, the
// wick's bounce print, under a last entry at 0.21482, and the price never came
// back; measured from the limit the next depth was out of reach and the ladder
// froze. While the order rests, a print more than one step (percentage 1.5 +
// tolerance 0.1) over it cancels it (update_buy); any other print reads the row
// as before.
func TestGetPositionBuyCancelsARunawayRestingAdd(t *testing.T) {
	strategy := func(resting bool) Strategy {
		return Strategy{
			Position: Position{Type: "buy", Price: 0.0732, AverageEntryPrice: 0.21781, RestingOrder: resting},
			Logic:    map[string]string{"buy": buyLogic},
			Settings: []aggragates.StrategySettings{{Percentage: 1.5, Tolerance: 0.1, TrailingTakeProfit: 0.9}},
		}
	}

	tests := []struct {
		name             string
		resting          bool
		percentage       float64
		profitPercentage float64
		want             string
	}{
		{"resting, a print 10% over the order", true, 10, -17, "update_buy"},
		{"resting, a print 0.41% over the order, under one step", true, 0.41, -17, ""},
		{"resting, a print just under one step (1.6%) over the order", true, 1.59, -17, ""},
		{"resting, a print just over one step over the order", true, 1.61, -17, "update_buy"},
		{"resting, a print 1.7% over the order, one step is 1.6%", true, 1.7, -17, "update_buy"},
		{"resting, a print a step under the order still arms the next depth", true, -2, -17, "trailingBuy"},
		{"not resting, a print 10% over the last fill reads as before", false, 10, -17, ""},
		{"not resting, the take profit still arms over break even", false, 1.7, 1.7, "takeProfit"},
		{"resting, the take profit is behind the cancel", true, 1.7, 1.7, "update_buy"},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			if got := strategy(tc.resting).GetPosition(tc.percentage, tc.profitPercentage); got != tc.want {
				t.Fatalf("GetPosition(%v, %v) with RestingOrder %v = %q, want %q", tc.percentage, tc.profitPercentage, tc.resting, got, tc.want)
			}
		})
	}
}

// TestGetPositionBuyRestingAddOnTheRunawayPrints runs the 211288 prints through
// the strategy's own metric: 0.08134, 5 ms after the placement, and 0.18259,
// days later, are over one step from the 0.0732 limit; 0.0735 is not.
func TestGetPositionBuyRestingAddOnTheRunawayPrints(t *testing.T) {
	strategy := Strategy{
		Position: Position{Type: "buy", Price: 0.0732, AverageEntryPrice: 0.21781, RestingOrder: true},
		Logic:    map[string]string{"buy": buyLogic},
		Settings: []aggragates.StrategySettings{{Percentage: 1.5, Tolerance: 0.1, TrailingTakeProfit: 0.9}},
	}
	decide := func(price float64) string {
		return strategy.GetPosition(strategy.GetPercentage(price), strategy.GetProfitPercentage(price))
	}

	for price, want := range map[float64]string{0.08134: "update_buy", 0.18259: "update_buy", 0.0735: ""} {
		if got := decide(price); got != want {
			t.Errorf("a print at %v on the resting add = %q, want %q", price, got, want)
		}
	}

	strategy.Position.RestingOrder = false
	if got := decide(0.08134); got != "" {
		t.Errorf("without the resting order the 0.08134 print must propose nothing, got %q", got)
	}
}

// TestBuyLogicParses guards the silent failure: logicExpression caches nil for a
// string govaluate cannot parse, and GetPosition then answers "" for every input.
func TestBuyLogicParses(t *testing.T) {
	if logicExpression(buyLogic) == nil {
		t.Fatal("the buy row does not parse: GetPosition would answer \"\" for every print")
	}
}

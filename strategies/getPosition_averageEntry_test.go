package strategies

import (
	"math"
	"testing"

	"github.com/giovani-sirbu/mercury/trades/aggragates"
)

// The engines' `buy` row: the downside keeps the last fill as its anchor, the
// upside reads profitPercentage, the move against the average entry price.
const buyLogic = "percentage <= -tradePercentage-tolerance ? 'stopLoss' : (profitPercentage >= tradePercentage+tolerance ? 'takeProfit' : '')"

// Backtest 119 / trade 39385: eight ETH fills averaging 1926.76, the last at
// 1400.21. Against the last fill alone the take profit was proposed from
// 1439.8 up — 25% under break even — and only the profit gate stood in the
// way. Against the average it is proposed at 1926.76 / (1 - 0.0275) = 1981.24,
// break even + percentage + tolerance, while the next depth still arms 2.75%
// under the last fill.
func TestGetPositionBuyArmsTakeProfitFromTheAverageEntryPrice(t *testing.T) {
	strategy := Strategy{
		Position: Position{Type: "buy", Price: 1400.21, AverageEntryPrice: 1926.76},
		Logic:    map[string]string{"buy": buyLogic},
		Settings: []aggragates.StrategySettings{{Percentage: 2.5, Tolerance: 0.25}},
	}
	decide := func(price float64) string {
		return strategy.GetPosition(strategy.GetPercentage(price), strategy.GetProfitPercentage(price))
	}

	tests := []struct {
		name  string
		price float64
		want  string
	}{
		{"back at break even, 27% over the last fill", 1935.50, ""},
		{"just under break even + 2.75%", 1980.00, ""},
		{"break even + percentage + tolerance", 1981.30, "takeProfit"},
		{"2.75% under the last fill", 1361.00, "stopLoss"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := decide(tt.price); got != tt.want {
				t.Errorf("at %v = %q, want %q", tt.price, got, tt.want)
			}
		})
	}
}

// At depth 1 the average entry price IS the fill, so the first depth arms
// exactly where it always did.
func TestGetPositionFirstDepthIsUnchanged(t *testing.T) {
	strategy := Strategy{
		Position: Position{Type: "buy", Price: 100, AverageEntryPrice: 100},
		Logic:    map[string]string{"buy": buyLogic},
		Settings: []aggragates.StrategySettings{{Percentage: 2, Tolerance: 0.25}},
	}
	decide := func(price float64) string {
		return strategy.GetPosition(strategy.GetPercentage(price), strategy.GetProfitPercentage(price))
	}

	if got := decide(102.30); got != "" {
		t.Fatalf("+2.25%% must not arm on a 2%% + 0.25%% ladder, got %q", got)
	}
	if got := decide(102.32); got != "takeProfit" {
		t.Fatalf("100 / (1 - 0.0225) = 102.30 arms the take profit, got %q", got)
	}
	if got := decide(97.70); got != "stopLoss" {
		t.Fatalf("-2.35%% arms the next depth, got %q", got)
	}
}

// Without entries there is no break even yet: profitPercentage falls back to
// the last-fill metric, so rows keep evaluating as before.
func TestGetProfitPercentageFallsBackToTheLastFillWithoutEntries(t *testing.T) {
	strategy := Strategy{Position: Position{Type: "buy", Price: 100}}

	if got, want := strategy.GetProfitPercentage(102), strategy.GetPercentage(102); math.Abs(got-want) > 1e-12 {
		t.Fatalf("profit percentage without entries = %v, want the last-fill percentage %v", got, want)
	}
}

// The metric itself: (price - average) / price, the same shape as
// GetPercentage, so the engines can negate both the same way for inverse.
func TestGetProfitPercentageMeasuresAgainstTheAverageEntryPrice(t *testing.T) {
	strategy := Strategy{Position: Position{Type: "buy", Price: 100, AverageEntryPrice: 110}}

	if got := strategy.GetProfitPercentage(105); math.Abs(got-(-4.761904761904762)) > 1e-9 {
		t.Fatalf("profit percentage = %v, want (105 - 110) / 105 * 100", got)
	}
}

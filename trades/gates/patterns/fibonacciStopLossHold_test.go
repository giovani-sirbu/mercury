package patterns

import (
	"testing"

	"github.com/giovani-sirbu/mercury/trades/aggragates"
	"github.com/giovani-sirbu/mercury/trades/internal/testutil"
)

func fibTrade(price float64, inverse bool) aggragates.Trades {
	trade := testutil.NewHoldTrade("stopLoss", inverse)
	trade.Strategy.Params.UsePatterns = true
	trade.PositionPrice = price
	trade.StrategyPair.TradeFilters = aggragates.TradeFilters{PriceFilter: 2}
	return trade
}

func TestFibonacciHoldWaitsForTheNextLevel(t *testing.T) {
	cases := []struct {
		price float64
		want  string
	}{
		{106.50, "fibonacci: waiting for a better price (next level 106.18)"},
		{106.30, ""}, // within 0.25% of 106.18: at the level
		{105.40, "fibonacci: waiting for a better price (next level 105.00)"},
		{101.00, ""}, // under every level: nothing left to wait for
		{112.00, ""}, // above the swing high: no pullback to measure
	}
	for _, c := range cases {
		got := fibonacciStopLossHold(fibTrade(c.price, false), testutil.FibAI())
		if got != c.want {
			t.Errorf("price %v: got %q, want %q", c.price, got, c.want)
		}
	}
}

func TestFibonacciHoldInertCases(t *testing.T) {
	if got := fibonacciStopLossHold(fibTrade(106.5, true), testutil.FibAI()); got != "" {
		t.Errorf("inverse must not wait on the up-swing retracement, got %q", got)
	}
	empty := testutil.FibAI()
	empty.FibLevels = nil
	if got := fibonacciStopLossHold(fibTrade(106.5, false), empty); got != "" {
		t.Errorf("no levels must hold nothing, got %q", got)
	}
	invalid := testutil.FibAI()
	invalid.FibSwingHigh = invalid.FibSwingLow
	if got := fibonacciStopLossHold(fibTrade(106.5, false), invalid); got != "" {
		t.Errorf("a degenerate swing must hold nothing, got %q", got)
	}
	if got := fibonacciStopLossHold(fibTrade(0, false), testutil.FibAI()); got != "" {
		t.Errorf("no price must hold nothing, got %q", got)
	}
}

func TestNextLowerFibLevel(t *testing.T) {
	levels := []float64{106.18, 105, 103.82, 102.14}
	if level, ok := nextLowerFibLevel(levels, 106.5); !ok || level != 106.18 {
		t.Errorf("106.5 → %v %v, want 106.18", level, ok)
	}
	if level, ok := nextLowerFibLevel(levels, 105.4); !ok || level != 105 {
		t.Errorf("105.4 → %v %v, want 105", level, ok)
	}
	if _, ok := nextLowerFibLevel(levels, 101); ok {
		t.Error("101 has no lower level")
	}
	if _, ok := nextLowerFibLevel(nil, 105); ok {
		t.Error("no levels, no answer")
	}
}

package strategies

import (
	"testing"

	"github.com/giovani-sirbu/mercury/trades/aggragates"
)

// A position type the logic map does not know must fail closed (no
// transition) instead of evaluating a nil expression and panicking — hermes
// runs GetPosition in an unrecovered goroutine per trade.
func TestGetPositionUnknownLogicKeyFailsClosed(t *testing.T) {
	strategy := Strategy{
		Position: Position{Type: "forceTrailingStopLoss", Price: 100},
		Settings: []aggragates.StrategySettings{{Percentage: 2, Tolerance: 0.25, TrailingTakeProfit: 0.5}},
		Logic: map[string]string{
			"buy": "percentage <= -tradePercentage-tolerance ? 'stopLoss' : ''",
		},
	}
	if got := strategy.GetPosition(-5); got != "" {
		t.Fatalf("unknown logic key must yield no position, got %q", got)
	}
	strategy.Position.Type = "buy"
	if got := strategy.GetPosition(-5); got != "stopLoss" {
		t.Fatalf("known logic key must still evaluate, got %q", got)
	}
}

package patterns

import (
	"github.com/giovani-sirbu/mercury/events"
	"github.com/giovani-sirbu/mercury/strategies/gates"
	"github.com/giovani-sirbu/mercury/trades/aggragates"
)

// PositionHold is the gate on an open position, never on the first fill, on the 15m chart-pattern
// verdict sophos serves: a pattern in the trade's direction prevents a trailingBuy (else the fibonacci
// wait may), and one with its target still ahead defers the takeProfit; anything else releases. The
// hold is keyed by the rule that spoke (HOLD_PATTERN, HOLD_FIBONACCI) and writes the row alone.
func PositionHold(event events.Events, position string, ai aggragates.AIIndicators) gates.Hold {
	if event.Params.OldPosition == aggragates.PositionNew {
		return gates.Hold{}
	}
	switch position {
	case aggragates.PositionTrailingBuy:
		if reason := patternTrailingBuyHold(event.Trade, ai); reason != "" {
			return gates.Hold{Reason: reason, Key: aggragates.LogKeyHoldPattern}
		}
		if reason := fibonacciTrailingBuyHold(event.Trade, ai); reason != "" {
			return gates.Hold{Reason: reason, Key: aggragates.LogKeyHoldFibonacci}
		}
	case aggragates.PositionTakeProfit:
		if reason := patternTakeProfitHold(event.Trade, ai); reason != "" {
			return gates.Hold{Reason: reason, Key: aggragates.LogKeyHoldPattern}
		}
	}
	return gates.Hold{}
}

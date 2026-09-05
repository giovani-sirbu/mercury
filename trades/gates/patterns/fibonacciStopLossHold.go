package patterns

import (
	"fmt"

	"github.com/giovani-sirbu/mercury/trades/aggragates"
	"github.com/giovani-sirbu/mercury/trades/gates"
)

// FibLevelTolerancePct: a rung this close ABOVE the next fibonacci level
// counts as sitting at the level.
const FibLevelTolerancePct = 0.25

// fibonacciStopLossHold is the UsePatterns flag's "waiting for a better
// price": sophos serves the 0.382/0.5/0.618/0.786 retracements of the last
// 15m up-swing, and a rung that would arm above the next lower level waits
// until price reaches it. Long only for now — the swing lens reads the
// up-move, so an inverse mirror needs its own swing first. Above the swing
// high there is no pullback to measure, and below the deepest level every
// level is already beaten; both release.
func fibonacciStopLossHold(trade aggragates.Trades, ai aggragates.AIIndicators) string {
	if trade.Inverse {
		return ""
	}
	// The engines set PositionPrice to the tick price before the chain runs,
	// so this is the price the rung would arm at.
	price := trade.PositionPrice
	if price <= 0 || len(ai.FibLevels) == 0 || ai.FibSwingHigh <= ai.FibSwingLow {
		return ""
	}
	if price > ai.FibSwingHigh {
		return ""
	}
	level, ok := nextLowerFibLevel(ai.FibLevels, price)
	if !ok {
		return ""
	}
	if price <= level*(1+FibLevelTolerancePct/100) {
		return ""
	}
	// The level, not the price, goes in the message: the price moves every
	// tick and would defeat the dedup; the level moves only when a new swing
	// forms, which is a new fact worth a row.
	return fmt.Sprintf("fibonacci: waiting for a better price (next level %s)", gates.FormatPriceLevel(trade, level))
}

// nextLowerFibLevel is the highest level strictly below price.
func nextLowerFibLevel(levels []float64, price float64) (float64, bool) {
	best, found := 0.0, false
	for _, level := range levels {
		if level < price && (!found || level > best) {
			best, found = level, true
		}
	}
	return best, found
}

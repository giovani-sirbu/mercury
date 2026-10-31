package patterns

import (
	"fmt"

	"github.com/giovani-sirbu/mercury/helpers"
	"github.com/giovani-sirbu/mercury/trades/aggragates"
)

// FibLevelTolerancePct is how close above the next fibonacci level a depth counts as at the level.
const FibLevelTolerancePct = 0.25

// fibonacciTrailingBuyHold is the UsePatterns flag's wait for a better price, long only: a depth that
// would arm above the next lower retracement of the last 15m up-swing sophos serves waits until
// price reaches it; above the swing high or below the deepest level it releases.
func fibonacciTrailingBuyHold(trade aggragates.Trades, ai aggragates.AIIndicators) string {
	if trade.Inverse {
		return ""
	}
	// PositionPrice is the tick price the depth would arm at.
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
	// The level, not the price, goes in the message so the row deduplicates.
	return fmt.Sprintf("fibonacci: waiting for a better price (next level %s)", helpers.FormatPriceLevel(trade, level))
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

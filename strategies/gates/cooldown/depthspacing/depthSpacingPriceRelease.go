package depthspacing

import (
	"github.com/giovani-sirbu/mercury/trades/aggragates"
	"github.com/giovani-sirbu/mercury/trades/ladder"
)

// depthSpacingReleasePrice is the price at which a standing hold lifts, and whether one could be
// priced: lastFill * (1 ∓ (percentage + tolerance + percentage*step)/100) on the held depth's row;
// a long releases at or below it, an inverse at or above.
func depthSpacingReleasePrice(trade aggragates.Trades, lastFill float64, step int) (float64, bool) {
	if lastFill <= 0 || step < 1 {
		return 0, false
	}
	settings := trade.StrategyPair.StrategySettings
	if len(settings) == 0 {
		return 0, false
	}
	row := settings[ladder.SettingsIndexOrBase(settings, ladder.CountFilledEntries(trade)-1)]
	if row.Percentage <= 0 {
		return 0, false
	}

	discount := (row.Percentage + row.Tolerance + row.Percentage*float64(step)) / 100
	if discount <= 0 || discount >= 1 {
		return 0, false
	}
	if trade.Inverse {
		return lastFill * (1 + discount), true
	}
	return lastFill * (1 - discount), true
}

// depthSpacingPriceReleased is whether the tick price (trade.PositionPrice, set by the engines
// before the chain runs) has already paid the release price, so the depth may arm despite the clock.
func depthSpacingPriceReleased(trade aggragates.Trades, now, lastFill float64, step int) bool {
	if now <= 0 {
		return false
	}
	release, ok := depthSpacingReleasePrice(trade, lastFill, step)
	if !ok {
		return false
	}
	if trade.Inverse {
		return now >= release
	}
	return now <= release
}

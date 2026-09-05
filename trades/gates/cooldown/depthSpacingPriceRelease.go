package cooldown

import (
	"github.com/giovani-sirbu/mercury/trades/aggragates"
	"github.com/giovani-sirbu/mercury/trades/ladder"
)

// The price release is what makes the wait pay for itself.
//
// A pure time gate only ever DELAYS a depth: the ladder is anchored to price,
// so the same entry fills at the same level, later. Measured over five years
// that cost 15% of the profit and left depth, blockage and the longest stall
// unchanged — the gate bought nothing with the time it spent.
//
// So the hold is not a wait, it is a PRICE it asks the market to pay. While a
// depth is held, one extra `percentage` step per escalation level buys it out:
//
//	release = lastFillPrice * (1 - (percentage + tolerance + percentage*step)/100)
//
// `percentage + tolerance` is where the depth would have armed anyway, so what
// the gate actually demands is `percentage * step` BELOW that. On HBAR/USDT
// (percentage 2.5, tolerance 0.2) the first hold releases 5.2% under the last
// fill instead of the usual 2.7%, the second 7.7%, the third 10.2%. Either the
// drop keeps going and the ladder buys meaningfully lower than it would have,
// or it does not and the capital was right to wait.
//
// The reference is the LAST FILL, not the trailed stopLoss anchor: the anchor
// follows the low and is path-dependent, so two engines replaying the same
// tape could disagree on it, while the fill price is a persisted fact.

// depthSpacingReleasePrice is the price at which a standing hold lifts, and
// whether one could be computed at all. A long releases at or below it; an
// inverse trade, whose entries are SELLs, releases at or above it.
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

	// The whole distance from the last fill: the ladder's own step, plus one
	// more step per escalation level.
	discount := (row.Percentage + row.Tolerance + row.Percentage*float64(step)) / 100
	if discount <= 0 || discount >= 1 {
		return 0, false
	}
	if trade.Inverse {
		return lastFill * (1 + discount), true
	}
	return lastFill * (1 - discount), true
}

// depthSpacingPriceReleased reports whether the market has already paid the
// price the hold asks for, so the depth may arm despite the clock.
//
// `now` is the tick price: the engines set trade.PositionPrice to the print
// being evaluated before the action chain runs, and SaveHoldLog restores the
// old one when a gate stops the chain, so a held trade never carries a moved
// anchor into the next tick.
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

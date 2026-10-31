package dynamicparams

import "github.com/giovani-sirbu/mercury/trades/aggragates"

// Opening is what a ladder opens with on these reads (Raise of their tier, TierBothBearish in free
// fall, at the constants' current amounts) and true, the one moment the flag consults them; the zero
// Opened and false when the flag does not shape the trade, the trade has a fill or an opened event,
// or the increase raises nothing.
func Opening(trade aggragates.Trades, reads aggragates.DynamicParamsIndicators) (Opened, bool) {
	return openingFor(trade, reads, MixedIncrease)
}

// openingFor is Opening with the mixed tier's increase handed in, for the tests.
func openingFor(trade aggragates.Trades, reads aggragates.DynamicParamsIndicators, mixed Increase) (Opened, bool) {
	if !Applies(trade) || len(trade.History) > 0 {
		return Opened{}, false
	}

	if _, _, opened := OpenedRaise(trade); opened {
		return Opened{}, false
	}

	increase := raiseFor(openingTier(reads), mixed)
	if !increase.changesRows() {
		return Opened{}, false
	}

	points, depths := increaseAmounts(increase)

	return Opened{Points: points, Depths: depths}, true
}

// openingTier is the tier a ladder opens on: TierBothBearish in free fall (FreeFall), else TierOf.
func openingTier(reads aggragates.DynamicParamsIndicators) Tier {
	if FreeFall(reads) {
		return TierBothBearish
	}

	return TierOf(reads)
}

// increaseAmounts is what an increase adds to every row under the constants.
func increaseAmounts(increase Increase) (float64, int) {
	var points float64
	if increase.raisesPercentage() {
		points = BearPercentagePoints
	}

	var depths int
	if increase.raisesDepths() {
		depths = BearDepths
	}

	return points, depths
}

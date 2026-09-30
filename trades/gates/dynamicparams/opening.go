package dynamicparams

import "github.com/giovani-sirbu/mercury/trades/aggragates"

// Opening is the opened row's text for a ladder that opens on these reads,
// and true: the one moment the flag consults the reads. The increase is
// Raise(TierOf(reads)), and its amounts are the constants' at this moment —
// BearPercentagePoints when it names the percentage, BearDepths when it names
// the depths — written into the row (OpenedMessage). The ladder then trades
// the amounts its row carries for its whole life (RaisedSettings).
//
// It answers nothing — "" and false — when the flag does not shape the trade
// (Applies); when the trade has an entry fill, len(trade.History) > 0 as
// aggragates.Params.SizingTrade reads a first entry, so a ladder that opened
// on its configured rows stays on them; when the trade already carries an
// opened row (OpenedRaise), so a first entry that is held or refused funds,
// and is judged again on a later tick, writes no second row; and when the
// increase raises nothing — the base tier, a block sophos did not read, or
// amounts that are zero.
func Opening(trade aggragates.Trades, reads aggragates.DynamicParamsIndicators) (string, bool) {
	return openingFor(trade, reads, MixedIncrease)
}

// openingFor is Opening with the mixed tier's increase handed in, as raiseFor
// takes it, so every option MixedIncrease can take is testable without
// changing the constant.
func openingFor(trade aggragates.Trades, reads aggragates.DynamicParamsIndicators, mixed Increase) (string, bool) {
	if !Applies(trade) || len(trade.History) > 0 {
		return "", false
	}

	if _, _, opened := OpenedRaise(trade); opened {
		return "", false
	}

	increase := raiseFor(TierOf(reads), mixed)
	if !increase.changesRows() {
		return "", false
	}

	points, depths := increaseAmounts(increase)

	return OpenedMessage(points, depths), true
}

// increaseAmounts is what an increase adds to every row under the constants:
// BearPercentagePoints to the percentage when it names the percentage,
// BearDepths to the depths when it names the depths, and nothing to a field
// it does not name.
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

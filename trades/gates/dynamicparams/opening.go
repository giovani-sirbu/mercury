package dynamicparams

import "github.com/giovani-sirbu/mercury/trades/aggragates"

// Opening is what a ladder opens with on these reads, and true: the one
// moment the flag consults the reads. The increase is Raise(TierOf(reads)),
// and its amounts are the constants' at this moment — BearPercentagePoints
// when it names the percentage, BearDepths when it names the depths — which
// the engines write as the ladder's opened pair (Opened.Rows). The ladder then
// trades the amounts its opened event carries for its whole life
// (RaisedSettings).
//
// It answers nothing — the zero Opened and false — when the flag does not
// shape the trade (Applies); when the trade has an entry fill,
// len(trade.History) > 0 as aggragates.Params.SizingTrade reads a first entry,
// so a ladder that opened on its configured rows stays on them; when the
// trade already carries an opened event (OpenedRaise), so a first entry that is
// held or refused funds, and is judged again on a later tick, writes no second
// pair; and when the increase raises nothing — the base tier, a block sophos
// did not read, or amounts that are zero.
func Opening(trade aggragates.Trades, reads aggragates.DynamicParamsIndicators) (Opened, bool) {
	return openingFor(trade, reads, MixedIncrease)
}

// openingFor is Opening with the mixed tier's increase handed in, as raiseFor
// takes it, so every option MixedIncrease can take is testable without
// changing the constant.
func openingFor(trade aggragates.Trades, reads aggragates.DynamicParamsIndicators, mixed Increase) (Opened, bool) {
	if !Applies(trade) || len(trade.History) > 0 {
		return Opened{}, false
	}

	if _, _, opened := OpenedRaise(trade); opened {
		return Opened{}, false
	}

	increase := raiseFor(TierOf(reads), mixed)
	if !increase.changesRows() {
		return Opened{}, false
	}

	points, depths := increaseAmounts(increase)

	return Opened{Points: points, Depths: depths}, true
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

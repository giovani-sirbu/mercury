package ladder

import (
	"math"

	"github.com/giovani-sirbu/mercury/helpers"
	"github.com/giovani-sirbu/mercury/trades/aggragates"
)

// NextEntryCost is what the trade's next entry spends, and the asset it
// spends it in: the ladder's next depth for a trade that has already filled
// one, the ladder's initial bid for a trade that has not.
//
// It mirrors funds.GetFundsQuantities — the entry-side branch of it — and has
// to move with it. The duplication is deliberate: the funds gate reaches the
// exchange for a balance and a permission check on a path that may place an
// order, while this one is a pure read the wallet gate makes about a trade
// that is only being weighed. Sizing them apart is what would leave the gate
// holding an entry for an amount the engine never spends.
//
// It prices the entry at the position the chain is about to run, which is
// already the level that entry arms at — so unlike the remaining-depths walk
// it takes no further step down the grid.
//
// budget is the wallet the first entry would be sized against, exactly as the
// buy action sizes it; it takes no part once the ladder has a fill to
// multiply.
func NextEntryCost(trade aggragates.Trades, budget float64) (string, float64) {
	asset := SpendingAsset(trade)

	settings := trade.StrategyPair.StrategySettings
	if len(settings) == 0 {
		return asset, 0
	}

	filled := CountFilledEntries(trade)
	if filled == 0 {
		return asset, initialEntryCost(trade, budget)
	}

	if !trade.Inverse && trade.PositionPrice <= 0 {
		return asset, 0
	}

	// Entry number filled+1 reads row filled, the row-selection contract the
	// funds gate and Buy share.
	cost := lastEntryQuantity(trade) * settings[SettingsIndexOrBase(settings, filled)].Multiplier
	if !trade.Inverse {
		cost *= trade.PositionPrice
		// The funds gate fits a long's needed amount to the pair's lot size
		// before it compares, and it fits DOWNWARD — so leaving it out here
		// would weigh the entry a hair above what the engine ever spends.
		// Mirrored rather than waived: the two readings have to move
		// together, and an unmirrored fit is exactly the kind of difference
		// that survives until the day someone changes it.
		cost = helpers.ToFixed(cost, int(trade.StrategyPair.TradeFilters.LotSize))
	}

	return asset, cost
}

// initialEntryCost is the first entry: the ladder's initial bid, never below
// the pair's minimum, in the asset the entry spends. An inverse ladder is bid
// in base units, so its minimum is the notional converted at the position
// price; without a price there is no conversion and no floor to raise it to.
//
// It sizes the bid directly rather than through CalculateInitialBid, which
// searches downward through the depths for the first bid that clears the
// pair's minimum. That search only ever departs from the full-depth bid on a
// wallet too small to reach the minimum from it — and on such a wallet the
// funds gate refuses the entry outright, so no order is placed and there is
// nothing for the reserve to have mis-sized. Everywhere the gate can actually
// hold something, the two agree exactly; what the search costs, by contrast,
// is paid on every entry tick of every ladder of the wallet.
//
// An inverse ladder is planned without the percentage discount, and an
// impasse child on its own depth, both for the reasons CalculateInitialBid
// gives.
func initialEntryCost(trade aggragates.Trades, budget float64) float64 {
	settings := trade.StrategyPair.StrategySettings
	row := settings[SettingsIndexOrBase(settings, 0)]

	percentage := row.Percentage
	if trade.Inverse {
		percentage = 0
	}

	depth := plannedLadderDepth(row.Depths)
	if trade.ParentID != 0 {
		depth = row.ImpasseDepth
	}

	bid := GetInitialBidByDepth(budget*(1-InitialBidReservePercent/100), depth, row.Multiplier, percentage)

	minNotional := trade.StrategyPair.TradeFilters.MinNotional
	floor := minNotional
	if trade.Inverse {
		floor = 0
		if trade.PositionPrice > 0 {
			floor = minNotional / trade.PositionPrice
		}
	}

	return math.Max(bid, floor)
}

// plannedLadderDepth is the depth the initial bid is planned against: the
// row's configured depths brought onto the half-depth grid CalculateInitialBid
// steps down, so a pair configured off that grid is sized here at the same
// depth the search would have started from.
func plannedLadderDepth(depths float64) float64 {
	return math.Floor(depths*2) / 2
}

package ladder

import (
	"math"

	"github.com/giovani-sirbu/mercury/helpers"
	"github.com/giovani-sirbu/mercury/trades/aggragates"
)

// NextEntryCost is what the trade's next entry spends and the asset it spends
// it in: the next depth of a ladder with a fill, priced at the position the
// chain is about to run, else the initial bid sized against budget. It
// mirrors the funds gate's entry-side sizing (funds.GetFundsQuantities) and
// must move with it, so the depth-priority gate never holds an entry for an
// amount the engine does not spend. A first entry is priced on the rows of
// the trade it is handed (Params.EntrySettings through SizingTrade).
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

	// Entry filled+1 reads row filled.
	cost := lastEntryQuantity(trade) * settings[SettingsIndexOrBase(settings, filled)].Multiplier
	if !trade.Inverse {
		cost *= trade.PositionPrice
		// Mirrors the funds gate's downward lot-size fit.
		cost = helpers.ToFixed(cost, int(trade.StrategyPair.TradeFilters.LotSize))
	}

	return asset, cost
}

// initialEntryCost is the first entry: the ladder's initial bid, never below
// the pair's minimum notional, in the asset it spends (an inverse ladder's
// minimum is converted at the position price). It sizes the bid directly at
// the planned depth rather than through CalculateInitialBid's downward
// search, which departs from that bid only on a wallet the funds gate refuses.
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

// plannedLadderDepth brings the configured depths onto the half-depth grid CalculateInitialBid steps down.
func plannedLadderDepth(depths float64) float64 {
	return math.Floor(depths*2) / 2
}

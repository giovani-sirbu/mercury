package ladder

import (
	"strings"

	"github.com/giovani-sirbu/mercury/trades/aggragates"
)

// RemainingCost is the ladder's remaining PLANNED budget: what the entries
// from its next depth through its ceiling were sized to cost, with the asset
// they spend. It is the amount the cooldown depth-priority gate reserves the
// wallet for. The walk follows the grid's own arithmetic on the rows the
// ladder trades (tradedSettings): quantity up by the row's multiplier, price
// down by the row's percentage per depth, from the planned quantity of the
// last filled depth, never the executed one, so a part-filled entry does not
// read the ladder one depth shallower. Zero on a ladder that cannot name an
// amount: no fill, no settings row, no ceiling or already at it, a long one
// without a position price.
func RemainingCost(trade aggragates.Trades) (string, float64) {
	settings := tradedSettings(trade)
	filled := CountFilledEntries(trade)

	return remainingCostAt(trade, settings, filled, ceilingOf(settings, filled))
}

// remainingCostAt is RemainingCost for a caller that already holds the traded
// rows, the filled count and the ceiling.
func remainingCostAt(trade aggragates.Trades, settings []aggragates.StrategySettings, filled, ceiling int) (string, float64) {
	return SpendingAsset(trade), remainingCostFrom(trade, settings, filled, ceiling, trade.PositionPrice)
}

// remainingCostFrom is the walk itself, down the grid from price: RemainingCost
// starts it at the position price, the planned reading at the ladder's last
// fill. An inverse ladder is counted in bare base quantities, as its grid is
// planned; a long walk with no price names no amount.
func remainingCostFrom(trade aggragates.Trades, settings []aggragates.StrategySettings, filled, ceiling int, price float64) float64 {
	if len(settings) == 0 {
		return 0
	}

	if filled == 0 || ceiling <= 0 || filled >= ceiling {
		return 0
	}

	if !trade.Inverse && price <= 0 {
		return 0
	}

	quantity := plannedQuantityAtDepth(trade, settings, filled)
	cost := 0.0

	for depth := filled; depth < ceiling; depth++ {
		// Entry depth+1 reads row depth.
		row := settings[SettingsIndexOrBase(settings, depth)]
		quantity *= row.Multiplier

		if trade.Inverse {
			cost += quantity
			continue
		}

		// One percentage step down per depth.
		price *= 1 - row.Percentage/100
		cost += quantity * price
	}

	return cost
}

// SpendingAsset is the asset a ladder's entries spend: quote for a long
// ladder, base for an inverse one, the asset the funds gate compares the
// wallet against. An unparsable symbol names none, and a view entry with no
// asset matches no balance.
func SpendingAsset(trade aggragates.Trades) string {
	pair := strings.Split(trade.Symbol, "/")
	if len(pair) != 2 {
		return ""
	}

	if trade.Inverse {
		return pair[0]
	}
	return pair[1]
}

// plannedQuantityAtDepth is the first entry's quantity taken through the
// multipliers of the rows the entries after it read.
func plannedQuantityAtDepth(trade aggragates.Trades, settings []aggragates.StrategySettings, depth int) float64 {
	quantity := firstEntryQuantity(trade)

	// Entry k+1 reads row k; the first entry reads none.
	for entry := 1; entry < depth; entry++ {
		quantity *= settings[SettingsIndexOrBase(settings, entry)].Multiplier
	}

	return quantity
}

// firstEntryQuantity is the earliest entry-side history row's quantity,
// skipping the accounting rows; order status is not consulted.
func firstEntryQuantity(trade aggragates.Trades) float64 {
	entrySide := "BUY"
	if trade.Inverse {
		entrySide = "SELL"
	}

	for _, history := range trade.History {
		if history.Type != entrySide || history.Quantity <= 0 {
			continue
		}
		if history.Price <= AccountingPriceCeiling {
			continue
		}

		return history.Quantity
	}

	return 0
}

// lastEntryQuantity is the latest entry's quantity, read on a copy because
// GetLatestQuantityByHistory sorts the slice.
func lastEntryQuantity(trade aggragates.Trades) float64 {
	entrySide := "BUY"
	if trade.Inverse {
		entrySide = "SELL"
	}

	history := make([]aggragates.TradesHistory, len(trade.History))
	copy(history, trade.History)

	return GetLatestQuantityByHistory(history, entrySide)
}

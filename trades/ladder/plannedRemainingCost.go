package ladder

import "github.com/giovani-sirbu/mercury/trades/aggragates"

// plannedRemainingCostAt is the ladder's remaining depths priced down the grid
// from its LAST FILL: the walk RemainingCost takes, from the planned quantity
// of the last filled depth, started at the price that depth's entry was placed
// at instead of the position price.
//
// The cooldown depth-priority gate splits two ladders at the same depth on
// it, and the anchor is the whole reason it exists. The position price moves
// when a ladder arms its next entry and again while it trails, and on a
// blocked ladder sits wherever the block caught it, so RemainingCost changes
// between two fills with nothing about the ladder changed. The last fill
// moves only when a depth fills — exactly when the depth itself moves — so a
// ranking read off it cannot hand the front back and forth between two level
// ladders while neither of them buys anything.
//
// An inverse ladder's depths are counted in bare base units and the walk
// takes no price there, so on an inverse ladder this is RemainingCost itself.
// It names an amount on exactly the ladders RemainingCost does, the long one
// without a position price included: both describe the same remaining depths,
// and a ladder the view says reserves nothing ranks as one with nothing left
// to pay for.
//
// settings are the rows the ladder trades (tradedSettings), the ones the
// ceiling was read from, so a ladder that opened raised is ranked on the cost
// of the raised depths it still has to fill.
func plannedRemainingCostAt(trade aggragates.Trades, settings []aggragates.StrategySettings, filled, ceiling int) float64 {
	if !trade.Inverse && trade.PositionPrice <= 0 {
		return 0
	}

	return remainingCostFrom(trade, settings, filled, ceiling, lastEntryPrice(trade))
}

// lastEntryPrice is the price of the ladder's latest entry, read in history
// order from the end: the newest row CountFilledEntries would count, on the
// side this ladder enters from, skipping the bookkeeping rows an impasse
// child's profit transfer leaves on its parent — the filter firstEntryQuantity
// reads the first entry with.
//
// GetLatestTradePrice is not that reading. It keeps the accounting rows, so on
// an impasse parent it would start the walk at a profit transfer's sentinel
// price and the ladder would rank as all but free to finish. Order status is
// not consulted, for the reason firstEntryQuantity gives.
//
// A row rewritten while its entry settles in parts keeps the price the entry
// was placed at, so the anchor does not move while an entry is settling
// either.
func lastEntryPrice(trade aggragates.Trades) float64 {
	entrySide := "BUY"
	if trade.Inverse {
		entrySide = "SELL"
	}

	for index := len(trade.History) - 1; index >= 0; index-- {
		history := trade.History[index]
		if history.Type != entrySide || history.Quantity <= 0 {
			continue
		}
		if history.Price <= AccountingPriceCeiling {
			continue
		}

		return history.Price
	}

	return 0
}

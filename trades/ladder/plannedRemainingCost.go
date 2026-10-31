package ladder

import "github.com/giovani-sirbu/mercury/trades/aggragates"

// plannedRemainingCostAt is the ladder's remaining depths priced down the
// grid from its LAST FILL: RemainingCost's walk started at the price the last
// entry was placed at instead of the position price. The depth-priority gate
// ranks two ladders at the same depth on it: the position price moves while
// a ladder arms and trails, the last fill only when a depth fills, so the
// ranking cannot flip while neither ladder buys. On an inverse ladder it is
// RemainingCost itself, and it names an amount on exactly the ladders
// RemainingCost does.
func plannedRemainingCostAt(trade aggragates.Trades, settings []aggragates.StrategySettings, filled, ceiling int) float64 {
	if !trade.Inverse && trade.PositionPrice <= 0 {
		return 0
	}

	return remainingCostFrom(trade, settings, filled, ceiling, lastEntryPrice(trade))
}

// lastEntryPrice is the price of the ladder's newest entry-side fill, read
// from the end of the history and skipping the accounting rows, which
// GetLatestTradePrice keeps. A row rewritten while its entry settles keeps
// the price it was placed at, so the anchor does not move mid-fill.
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

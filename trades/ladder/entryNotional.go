package ladder

import "github.com/giovani-sirbu/mercury/trades/aggragates"

// EntryNotional is the amount the position has used: the quantity times the
// price of every entry fill — BUYs for a normal trade, SELLs for an inverse
// one — summed, so a partial fill of one order simply adds up. It is the
// numerator of AverageEntryPrice, the same fold, and what a loss is measured
// against when it is read as a share of the trade. Accounting rows
// (child-profit transfers onto an impasse parent) are bookkeeping, not
// entries, and are skipped by their sentinel price like CountFilledEntries
// does. Zero when nothing has filled yet.
func EntryNotional(trade aggragates.Trades) float64 {
	_, notional := entryTotals(trade)

	return notional
}

// entryTotals folds the entry fills into the quantity bought and what it
// cost: the one fold under EntryNotional and AverageEntryPrice, so the amount
// a trade has used and its average price can never disagree on which rows are
// entries.
func entryTotals(trade aggragates.Trades) (quantity, notional float64) {
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

		quantity += history.Quantity
		notional += history.Quantity * history.Price
	}

	return quantity, notional
}

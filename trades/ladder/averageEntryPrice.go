package ladder

import "github.com/giovani-sirbu/mercury/trades/aggragates"

// AverageEntryPrice is what the position cost per unit of base across its
// entry fills — BUYs for a normal trade, SELLs for an inverse one — the
// ladder's break even before fees. Partial fills of one order simply add up.
// Accounting rows (child-profit transfers onto an impasse parent) are
// bookkeeping, not entries, and are skipped by their sentinel price like
// CountFilledEntries does. Zero when nothing has filled yet.
func AverageEntryPrice(trade aggragates.Trades) float64 {
	entrySide := "BUY"
	if trade.Inverse {
		entrySide = "SELL"
	}

	var quantity, notional float64
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

	if quantity <= 0 {
		return 0
	}

	return notional / quantity
}

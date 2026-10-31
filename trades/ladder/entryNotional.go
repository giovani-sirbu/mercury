package ladder

import "github.com/giovani-sirbu/mercury/trades/aggragates"

// EntryNotional is the amount the position has used, quantity times price
// over every entry fill: the numerator of AverageEntryPrice and what a loss
// is measured against as a share of the trade. Zero before the first fill.
func EntryNotional(trade aggragates.Trades) float64 {
	_, notional := entryTotals(trade)

	return notional
}

// EntryQuantity is the base the position's entry fills moved: the denominator
// of AverageEntryPrice and what an inverse trade's profit, counted in base, is
// measured against. Zero before the first fill.
func EntryQuantity(trade aggragates.Trades) float64 {
	quantity, _ := entryTotals(trade)

	return quantity
}

// entryTotals is the one fold over the entry fills (accounting rows skipped)
// under EntryNotional, EntryQuantity and AverageEntryPrice.
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

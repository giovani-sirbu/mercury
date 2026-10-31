package ladder

import "github.com/giovani-sirbu/mercury/trades/aggragates"

// AverageEntryPrice is the position's cost per unit of base across its entry
// fills, the break even before fees: EntryNotional over EntryQuantity.
// Accounting rows are skipped; zero before the first fill.
func AverageEntryPrice(trade aggragates.Trades) float64 {
	quantity, notional := entryTotals(trade)
	if quantity <= 0 {
		return 0
	}

	return notional / quantity
}

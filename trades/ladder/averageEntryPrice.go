package ladder

import "github.com/giovani-sirbu/mercury/trades/aggragates"

// AverageEntryPrice is what the position cost per unit of base across its
// entry fills — BUYs for a normal trade, SELLs for an inverse one — the
// ladder's break even before fees: EntryNotional over the quantity of those
// fills. Partial fills of one order simply add up. Accounting rows
// (child-profit transfers onto an impasse parent) are bookkeeping, not
// entries, and are skipped by their sentinel price like CountFilledEntries
// does. Zero when nothing has filled yet.
func AverageEntryPrice(trade aggragates.Trades) float64 {
	quantity, notional := entryTotals(trade)
	if quantity <= 0 {
		return 0
	}

	return notional / quantity
}

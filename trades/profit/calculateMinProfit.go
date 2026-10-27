package profit

import (
	"math"

	"github.com/giovani-sirbu/mercury/strategies/gates/dynamicparams"
	"github.com/giovani-sirbu/mercury/trades/aggragates"
	"github.com/giovani-sirbu/mercury/trades/ladder"
)

// CalculateMinProfit is the least net profit a close must book before the
// profit gates take it: what the ladder's entries cost, times the share of
// the take profit percentage one depth stands for (the held depth's
// Percentage over its floored Depths, at least one), so a ladder of N depths
// closes at least Percentage / N over its average entry price. The rows are
// the ones the ladder trades (dynamicparams.RaisedSettings), read here
// because the chain hands over the stored rows. A normal trade's floor is
// the share of ladder.EntryNotional (profit in quote), an inverse trade's of
// ladder.EntryQuantity (profit in base); neither reads the position price.
// Zero with no settings rows or before the first fill.
func CalculateMinProfit(trade aggragates.Trades) float64 {
	rows, _ := dynamicparams.RaisedSettings(trade)
	if len(rows) == 0 {
		return 0
	}

	held := rows[ladder.SettingsIndexOrBase(rows, ladder.CountFilledEntries(trade)-1)]
	share := held.Percentage / math.Max(math.Floor(held.Depths), 1) / 100

	if trade.Inverse {
		return ladder.EntryQuantity(trade) * share
	}

	return ladder.EntryNotional(trade) * share
}

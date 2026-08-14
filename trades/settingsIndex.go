package trades

import "github.com/giovani-sirbu/mercury/trades/aggragates"

// accountingHistoryPriceCeiling identifies bookkeeping history rows: an
// impasse child closing marks its profit onto the parent as a BUY row with the
// sentinel price 1e-13. Anything at or below this ceiling is a ledger entry,
// never an executed market fill.
const accountingHistoryPriceCeiling = 1e-12

// CountFilledEntries counts the trade's executed entry orders — BUYs for a
// normal trade, SELLs for an inverse one. It counts DISTINCT exchange orders,
// not history rows: partial fills update the same order id and must not
// advance the ladder row the strategy reads. Accounting rows (child-profit
// transfers onto an impasse parent) are bookkeeping, not entries, and are
// skipped by their sentinel price.
func CountFilledEntries(trade aggragates.Trades) int {
	entrySide := "BUY"
	if trade.Inverse {
		entrySide = "SELL"
	}

	seenOrders := make(map[int64]struct{}, len(trade.History))
	filledEntries := 0
	for index, history := range trade.History {
		if history.Type != entrySide || history.Quantity <= 0 {
			continue
		}
		if history.Price <= accountingHistoryPriceCeiling {
			continue
		}

		orderID := history.OrderId
		if orderID == 0 {
			// Legacy rows may not carry an exchange order id. They are separate
			// persisted entries, so give each one a stable synthetic identity.
			orderID = -int64(index + 1)
		}
		if _, exists := seenOrders[orderID]; exists {
			continue
		}
		seenOrders[orderID] = struct{}{}
		filledEntries++
	}

	return filledEntries
}

// SettingsIndexOrBase resolves which StrategySettings row governs a given
// ladder depth. The contract, per configuration semantics:
//   - a single configured row applies to every depth;
//   - a depth whose row exists uses exactly that row;
//   - a depth whose row does NOT exist falls back to row 0 (the base row) —
//     never to the last row.
func SettingsIndexOrBase(settings []aggragates.StrategySettings, index int) int {
	if index < 0 || index >= len(settings) {
		return 0
	}
	return index
}

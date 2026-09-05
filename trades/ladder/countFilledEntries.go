package ladder

import "github.com/giovani-sirbu/mercury/trades/aggragates"

// AccountingPriceCeiling identifies bookkeeping history rows: an impasse
// child closing marks its profit onto the parent as a BUY row with the
// sentinel price 1e-13. Anything at or below this ceiling is a ledger entry,
// never an executed market fill. Every fold over the ladder's entries
// (CountFilledEntries, cooldown's depth spacing) skips rows by it.
const AccountingPriceCeiling = 1e-12

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
		if history.Price <= AccountingPriceCeiling {
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

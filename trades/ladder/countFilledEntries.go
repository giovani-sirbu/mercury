package ladder

import "github.com/giovani-sirbu/mercury/trades/aggragates"

// AccountingPriceCeiling identifies bookkeeping history rows: an impasse child
// marks its profit onto the parent as a BUY at a sentinel price under it.
// Every fold over the ladder's entries skips rows by it.
const AccountingPriceCeiling = 1e-12

// CountFilledEntries counts the trade's executed entry orders (BUYs, SELLs on
// an inverse trade) as DISTINCT exchange orders: partial fills update the same
// order id and must not advance the ladder row. Accounting rows are skipped.
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
			// A legacy row without an order id gets a stable synthetic identity.
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

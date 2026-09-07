package smarttakeloss

import (
	"time"

	"github.com/giovani-sirbu/mercury/trades/aggragates"
	"github.com/giovani-sirbu/mercury/trades/ladder"
)

// entryFill is one executed entry order: what it paid and when its history
// row was stamped.
type entryFill struct {
	Price float64
	At    time.Time
}

// entryFills is the trade's ladder, one entry per executed entry order, in
// history slice order — the placement order on every engine (GORM id order
// in hermes, append order in sisyphus). The one-more-depth counter and the
// last fill both read that order, not the stamps: ladder.GetLatestTradePrice
// ranks by CreatedAt, which live-testing stamps by hand.
//
// Membership mirrors ladder.CountFilledEntries row for row: entry-side rows
// (BUY, or SELL on an inverse trade) with a real quantity, above the
// accounting sentinel price, one entry per distinct exchange order id — a
// partial fill updates its order in place and never adds a depth — and a
// synthetic id for legacy rows that carry none. The first row of an order
// supplies its price and stamp. A zero stamp is kept: it only keeps that
// fill from activating (see activates), it does not void the ladder.
func entryFills(trade aggragates.Trades) []entryFill {
	entrySide := "BUY"
	if trade.Inverse {
		entrySide = "SELL"
	}

	seenOrders := make(map[int64]struct{}, len(trade.History))
	fills := make([]entryFill, 0, len(trade.History))
	for index, history := range trade.History {
		if history.Type != entrySide || history.Quantity <= 0 {
			continue
		}
		if history.Price <= ladder.AccountingPriceCeiling {
			continue
		}

		orderID := history.OrderId
		if orderID == 0 {
			orderID = -int64(index + 1)
		}
		if _, exists := seenOrders[orderID]; exists {
			continue
		}
		seenOrders[orderID] = struct{}{}
		fills = append(fills, entryFill{Price: history.Price, At: history.CreatedAt})
	}
	return fills
}

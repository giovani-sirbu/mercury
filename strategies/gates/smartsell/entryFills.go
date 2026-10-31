package smartsell

import (
	"time"

	"github.com/giovani-sirbu/mercury/trades/aggragates"
	"github.com/giovani-sirbu/mercury/trades/ladder"
)

// entryFill is one executed entry order: its price and its history row's stamp.
type entryFill struct {
	Price float64
	At    time.Time
}

// entryFills is the trade's ladder, one entry per executed entry order in history order (the
// placement order on every engine), mirroring ladder.CountFilledEntries row for row; a zero stamp is
// kept, it only keeps the leg from being read smooth from that fill.
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

package depthspacing

import (
	"github.com/giovani-sirbu/mercury/trades/ladder"
	"sort"
	"time"

	"github.com/giovani-sirbu/mercury/trades/aggragates"
)

// depthFill is one executed entry order: its stamp and the price it paid, which the price release
// measures from.
type depthFill struct {
	At    time.Time
	Price float64
}

// depthFills is the trade's ladder, one entry per executed entry order, oldest first, mirroring
// ladder.CountFilledEntries row for row; a fill without a stamp voids the whole read (nil).
func depthFills(trade aggragates.Trades) []depthFill {
	entrySide := "BUY"
	if trade.Inverse {
		entrySide = "SELL"
	}

	seenOrders := make(map[int64]struct{}, len(trade.History))
	fills := make([]depthFill, 0, len(trade.History))
	for index, history := range trade.History {
		if history.Type != entrySide || history.Quantity <= 0 {
			continue
		}
		if history.Price <= ladder.AccountingPriceCeiling {
			continue
		}

		orderID := history.OrderId
		if orderID == 0 {
			// Legacy rows without an order id get a stable synthetic one, as CountFilledEntries does.
			orderID = -int64(index + 1)
		}
		if _, exists := seenOrders[orderID]; exists {
			continue
		}
		seenOrders[orderID] = struct{}{}

		if history.CreatedAt.IsZero() {
			return nil
		}
		fills = append(fills, depthFill{At: history.CreatedAt.UTC(), Price: history.Price})
	}

	// The fold needs a sorted ladder; a re-hydrated trade's order is not trusted.
	sort.Slice(fills, func(i, j int) bool { return fills[i].At.Before(fills[j].At) })
	return fills
}

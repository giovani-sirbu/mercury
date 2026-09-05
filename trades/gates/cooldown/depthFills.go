package cooldown

import (
	"github.com/giovani-sirbu/mercury/trades/ladder"
	"sort"
	"time"

	"github.com/giovani-sirbu/mercury/trades/aggragates"
)

// depthFill is one executed entry order: when it landed and what it paid.
// The price is what the price-release leg measures its discount from, so it
// travels with the stamp rather than being fetched again from the history.
type depthFill struct {
	At    time.Time
	Price float64
}

// depthFills is the trade's ladder, one entry per executed entry order,
// oldest first.
//
// Membership mirrors ladder.CountFilledEntries exactly: entry-side rows (BUY,
// or SELL on an inverse trade) carrying a real quantity, above the accounting
// sentinel price, one entry per distinct exchange order id so a partial fill
// does not read as a depth, and a synthetic id for legacy rows that carry
// none. It is mirrored rather than called because that helper returns a count
// and this one needs the stamps.
//
// A depth with an unknown clock voids the whole read: without every stamp the
// fold cannot tell a fast depth from a slow one, and guessing here would park
// a trade on no evidence.
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
			// Legacy rows may not carry an exchange order id. They are
			// separate persisted entries, so give each a stable synthetic
			// identity — the same rule CountFilledEntries uses.
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

	// Persisted history arrives in insertion order, which is chronological,
	// but the fold is only correct on a sorted ladder and a re-hydrated trade
	// is not worth trusting on that.
	sort.Slice(fills, func(i, j int) bool { return fills[i].At.Before(fills[j].At) })
	return fills
}

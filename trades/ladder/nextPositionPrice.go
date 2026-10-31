package ladder

import (
	"math"

	"github.com/giovani-sirbu/mercury/trades/aggragates"
)

// NextPositionPrice is the anchor a transition writes, read on the trade as the tick found it
// (position type and pending order untouched): the tick price, except that a resting add's
// update_buy re-anchors at the last entry, and every other update_buy never anchors past the last
// entry (above it on a long, below it on an inverse ladder).
func NextPositionPrice(trade aggragates.Trades, position string, price float64) float64 {
	last := lastEntryPrice(trade)
	if position != "update_buy" || last <= 0 {
		return price
	}
	// The cancelled add leaves the ladder at its last fill, as if it had never been placed.
	if trade.PositionType == "buy" && trade.PendingOrder != 0 {
		return last
	}
	if trade.Inverse {
		return math.Max(price, last)
	}

	return math.Min(price, last)
}

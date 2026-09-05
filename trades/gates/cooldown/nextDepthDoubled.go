package cooldown

import (
	"github.com/giovani-sirbu/mercury/trades/aggragates"
	"github.com/giovani-sirbu/mercury/trades/ladder"
)

// NextDepthDoubled reports whether the second depth of this trade arms at
// double the ladder step: the first-fill hold was released UP through its
// reference and the entry went to market above it. The hold had read the
// bar as a local top and the market disagreed; the ladder is then one step
// closer to the next top than it planned for, and asking the second depth to
// come 2p down instead of p puts it where the first depth would have been.
//
// Only while the ladder is exactly one fill deep: the doubling is a one-time
// correction on the depth that follows the wrong call, not a wider grid.
// And only when that fill really landed at or above the reference: an entry
// released above R but filled below it (funds-blocked at the release, filled
// on a later, lower tick) did not enter above the reference and needs no
// correction. An inverse ladder mirrors — at or below R.
//
// The engines ask right after the position is resolved and apply
// WidenSettingsForNextDepth only when that position is stopLoss:
// strategies.Strategy.GetPosition binds one percentage to every transition
// of the row, so a doubled row applied blind would move the take profit to
// 2p + t as well.
func NextDepthDoubled(trade aggragates.Trades) bool {
	state := firstFillState(trade)
	if !state.activated || !state.enteredAbove {
		return false
	}
	if ladder.CountFilledEntries(trade) != 1 {
		return false
	}
	fill, ok := firstEntryFill(trade)
	if !ok {
		return false
	}
	if trade.Inverse {
		return fill <= state.reference
	}
	return fill >= state.reference
}

// firstEntryFill is the price of the ladder's first executed entry, by the
// membership ladder.CountFilledEntries uses: the entry side, a real
// quantity, above the accounting sentinel.
func firstEntryFill(trade aggragates.Trades) (float64, bool) {
	entrySide := "BUY"
	if trade.Inverse {
		entrySide = "SELL"
	}
	for _, history := range trade.History {
		if history.Type != entrySide || history.Quantity <= 0 || history.Price <= ladder.AccountingPriceCeiling {
			continue
		}
		return history.Price, true
	}
	return 0, false
}

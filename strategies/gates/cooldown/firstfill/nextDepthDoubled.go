package firstfill

import (
	"github.com/giovani-sirbu/mercury/trades/aggragates"
	"github.com/giovani-sirbu/mercury/trades/ladder"
)

// NextDepthDoubled is whether the second depth arms at double the step: the hold was released up
// through its reference (the entered event), the ladder is exactly one fill deep and that fill
// landed at or above the reference (at or below inverse); applied only on a trailingBuy position.
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

// firstEntryFill is the price of the first executed entry, by ladder.CountFilledEntries' membership.
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

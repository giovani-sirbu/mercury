package cooldown

import (
	"strings"

	"github.com/giovani-sirbu/mercury/trades/aggragates"
)

// firstFillRecord is what the trade's own log rows say about its first fill:
// whether the gate activated and at what reference, whether it armed and
// where the anchor sits, and whether the price ran through the reference
// instead. It is rebuilt from trade.Logs on every tick, the way
// crashguard.rebuildCapitulationEpisode rebuilds an episode: the rows are
// the only state. They reach the gate on all three engines (backtesting's
// memory trades, hermes' redis copy, live-testing's storage) and agora copies
// them whole on update-trade. trade.PositionPrice is deliberately not one of
// them: on a new trade it is 0 on every creation path, and sisyphus
// (hasOpenPosition) and agora (newDepthRequired) read a positive value as
// "the trade has entered".
type firstFillRecord struct {
	activated bool
	// reference is the price the hold activated at: the Price column of the
	// FIRST waiting row. gates.SaveHoldLog writes the same message again once
	// the standing row is a day old, at that day's price, so a later waiting
	// row is a re-log and never a new reference.
	reference float64
	armed     bool
	// anchor is the extreme the armed hold trails: the lowest armed-row Price
	// on a long, the highest on an inverse ladder. A re-logged armed row
	// carries the price of its day, which can sit anywhere inside the current
	// step, and taking the extreme keeps such a row from moving the anchor
	// the wrong way.
	anchor float64
	// enteredAbove: the price ran through the reference and the entry went
	// to market. The gate is finished with this trade.
	enteredAbove bool
}

// firstFillState rebuilds the record from the trade logs. A row is matched
// by its marker anywhere in the message, because the hold rows carry
// gates.SaveHoldLog's "Hold entry: " frame in front of it and the entered row
// does not. A hold row without a price carries no level and is skipped.
func firstFillState(trade aggragates.Trades) firstFillRecord {
	var state firstFillRecord
	for _, row := range trade.Logs {
		if strings.Contains(row.Message, FirstFillEnteredPrefix) {
			state.enteredAbove = true
			continue
		}
		if row.Price <= 0 {
			continue
		}
		switch {
		case strings.Contains(row.Message, FirstFillWaitingPrefix):
			if !state.activated {
				state.activated = true
				state.reference = row.Price
			}
		case strings.Contains(row.Message, FirstFillArmedPrefix):
			if !state.armed || firstFillDeeper(trade.Inverse, row.Price, state.anchor) {
				state.armed = true
				state.anchor = row.Price
			}
		}
	}
	return state
}

// firstFillDeeper reports whether price is further along the hold's own
// direction than anchor: lower on a long, higher on an inverse ladder.
func firstFillDeeper(inverse bool, price, anchor float64) bool {
	if inverse {
		return price > anchor
	}
	return price < anchor
}

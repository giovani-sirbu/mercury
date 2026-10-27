package firstfill

import (
	"time"

	"github.com/giovani-sirbu/mercury/strategies/gates/cooldown"
	"github.com/giovani-sirbu/mercury/trades/aggragates"
)

// firstFillRecord is what the trade's first-fill events say: activated, at what reference and when,
// armed and where the anchor sits, or entered above the reference. The events are the only state;
// trade.PositionPrice is never one of them.
type firstFillRecord struct {
	activated bool
	// reference is the Price of the FIRST activated event; a later one is a re-log.
	reference float64
	// activatedAt is that event's stamp, which FirstFillMaxHold measures from; zero never expires.
	activatedAt time.Time
	armed       bool
	// anchor is the extreme of the armed events' prices: the lowest on a long, the highest on an
	// inverse ladder, so a re-logged event cannot move it the wrong way.
	anchor float64
	// enteredAbove is the price having run through the reference; the gate is finished.
	enteredAbove bool
}

// firstFillState rebuilds the record from the trade's first-fill events in order; an event without
// a price carries no level and is skipped, the verdict event and unreadable events change nothing.
func firstFillState(trade aggragates.Trades) firstFillRecord {
	var state firstFillRecord
	for _, event := range trade.StrategyEventsOf(aggragates.StrategyParamCooldown, cooldown.GateFirstFill) {
		var data cooldown.FirstFillEvent
		if err := event.DecodeData(&data); err != nil {
			continue
		}
		if data.Event == cooldown.FirstFillEntered {
			state.enteredAbove = true
			continue
		}
		if data.Price <= 0 {
			continue
		}
		switch data.Event {
		case cooldown.FirstFillActivated:
			if !state.activated {
				state.activated = true
				state.reference = data.Price
				state.activatedAt = event.CreatedAt
			}
		case cooldown.FirstFillArmed:
			if !state.armed || firstFillDeeper(trade.Inverse, data.Price, state.anchor) {
				state.armed = true
				state.anchor = data.Price
			}
		}
	}
	return state
}

// firstFillDeeper is whether price is further along the hold's direction than anchor.
func firstFillDeeper(inverse bool, price, anchor float64) bool {
	if inverse {
		return price > anchor
	}
	return price < anchor
}

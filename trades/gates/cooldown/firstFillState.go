package cooldown

import (
	"time"

	"github.com/giovani-sirbu/mercury/trades/aggragates"
)

// firstFillRecord is what the trade's own first-fill events say about its
// first fill: whether the gate activated and at what reference, whether it
// armed and where the anchor sits, and whether the price ran through the
// reference instead. It is rebuilt from trade.StrategyEvents on every tick:
// the events are the only state. They reach the gate on all three engines
// (backtesting's memory trades, hermes' redis copy, live-testing's storage)
// and agora copies them whole on update-trade. trade.PositionPrice is
// deliberately not one of them: on a new trade it is 0 on every creation path,
// and sisyphus (hasOpenPosition) and agora (newDepthRequired) read a positive
// value as "the trade has entered".
type firstFillRecord struct {
	activated bool
	// reference is the price the hold activated at: the Price of the FIRST
	// activated event. gates.SaveHoldLog writes the same message again once
	// the standing row is older than its re-log window, with an activated
	// event at the price of that later tick, so a later activated event is a
	// re-log and never a new reference.
	reference float64
	// activatedAt is that same event's stamp — the tick the hold started,
	// which FirstFillMaxHold measures from. Zero when the engine did not stamp
	// it, and then the hold never expires (firstFillExpired).
	activatedAt time.Time
	armed       bool
	// anchor is the extreme the armed hold trails: the lowest armed-event
	// Price on a long, the highest on an inverse ladder. A re-logged armed
	// event carries the price of the tick it was re-logged at, which can sit
	// anywhere inside the current step, and taking the extreme keeps such an
	// event from moving the anchor the wrong way.
	anchor float64
	// enteredAbove: the price ran through the reference and the entry went
	// to market. The gate is finished with this trade.
	enteredAbove bool
}

// firstFillState rebuilds the record from the trade's first-fill events, in
// the order the trade carries them. The entered event counts wherever it
// stands, priced or not. Every other event needs the Price of the tick it was
// written on — an event without one carries no level and is skipped — and
// the verdict event, an event whose document cannot be read and an event of
// a kind this fold does not know change nothing.
func firstFillState(trade aggragates.Trades) firstFillRecord {
	var state firstFillRecord
	for _, event := range trade.StrategyEventsOf(aggragates.StrategyParamCooldown, GateFirstFill) {
		var data FirstFillEvent
		if err := event.DecodeData(&data); err != nil {
			continue
		}
		if data.Event == FirstFillEntered {
			state.enteredAbove = true
			continue
		}
		if data.Price <= 0 {
			continue
		}
		switch data.Event {
		case FirstFillActivated:
			if !state.activated {
				state.activated = true
				state.reference = data.Price
				state.activatedAt = event.CreatedAt
			}
		case FirstFillArmed:
			if !state.armed || firstFillDeeper(trade.Inverse, data.Price, state.anchor) {
				state.armed = true
				state.anchor = data.Price
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

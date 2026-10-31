package firstfill

import (
	"github.com/giovani-sirbu/mercury/events"
	"github.com/giovani-sirbu/mercury/strategies/gates"
	"github.com/giovani-sirbu/mercury/strategies/gates/cooldown"
	"github.com/giovani-sirbu/mercury/trades/aggragates"
)

// FirstFillHold is the first-fill gate: the zero Hold proceeds, a refusal carries the row's text and
// the FirstFillEvent. On spot the verdict starts the hold at the tick price, then the price runs up
// through up(R) and the entry goes to market (NextDepthDoubled) or falls to arm(R) and fills like a
// depth, FirstFillMaxHold releasing silently; futures keep the verdict-only hold; it fails open.
func FirstFillHold(event events.Events, side string) (events.Events, gates.Hold) {
	trade := event.Trade
	verdict := event.Params.CoolDownIndicators

	if trade.Strategy.TradeType == aggragates.Futures {
		if firstFillVerdictRefuses(side, verdict) {
			verdictHeld := cooldown.FirstFillEvent{Event: cooldown.FirstFillVerdictHeld}
			return event, firstFillRefusal(verdictHeld, firstFillVerdictMessage(side), aggragates.LogKeyHoldFirstFillVerdict)
		}
		return event, gates.Hold{}
	}

	levels, ok := firstFillLevelsFrom(trade, side)
	tick := trade.PositionPrice
	if !ok || tick <= 0 {
		return event, gates.Hold{}
	}

	state := firstFillState(trade)
	if state.enteredAbove {
		return event, gates.Hold{}
	}
	// The cap releases silently: no entered pair, since no wrong call was made.
	if firstFillExpired(state, event.TickTime()) {
		return event, gates.Hold{}
	}
	if !state.activated {
		if !firstFillVerdictRefuses(side, verdict) {
			return event, gates.Hold{}
		}
		return event, firstFillWaitingHold(trade, levels, tick, tick)
	}
	if !state.armed {
		if levels.atOrAbove(tick, levels.up(state.reference)) {
			return appendFirstFillEnteredRow(event, levels, state.reference), gates.Hold{}
		}
		if levels.atOrBelow(tick, levels.arm(state.reference)) {
			return event, firstFillArmedHold(trade, levels, tick, state.reference, tick)
		}
		return event, firstFillWaitingHold(trade, levels, tick, state.reference)
	}
	if levels.above(tick, levels.bounce(state.anchor)) {
		return event, gates.Hold{}
	}
	if levels.below(tick, levels.trail(state.anchor)) {
		return event, firstFillArmedHold(trade, levels, tick, state.reference, tick)
	}
	return event, firstFillArmedHold(trade, levels, tick, state.reference, state.anchor)
}

// firstFillVerdictRefuses is the sophos read for side; a missing verdict refuses nothing.
func firstFillVerdictRefuses(side string, verdict aggragates.CoolDownIndicators) bool {
	if !verdict.HasFirstFillVerdict {
		return false
	}
	switch side {
	case aggragates.SideLong:
		return !verdict.AllowLongEntry
	case aggragates.SideShort:
		return !verdict.AllowShortEntry
	}
	return false
}

// appendFirstFillEnteredRow writes the entered pair on the trade once, the INFO row and the event,
// stamped with the tick clock; a tick that comes back with the event present writes no second pair.
func appendFirstFillEnteredRow(event events.Events, levels firstFillLevels, reference float64) events.Events {
	if firstFillState(event.Trade).enteredAbove {
		return event
	}
	now := event.TickTime()
	data := cooldown.FirstFillEvent{
		Event:     cooldown.FirstFillEntered,
		Price:     event.Trade.PositionPrice,
		Reference: reference,
	}
	event.Trade.Logs = append(event.Trade.Logs, aggragates.TradesLogs{
		Message:    firstFillEnteredMessage(event.Trade, levels, data),
		MessageKey: aggragates.LogKeyCooldownFirstFillEntered,
		Type:       aggragates.LOG_INFO,
		Price:      data.Price,
		TradeID:    event.Trade.ID,
		CreatedAt:  now,
		UpdatedAt:  now,
	})
	event.Trade.StrategyEvents = append(event.Trade.StrategyEvents, cooldown.NewFirstFillEvent(event.Trade.ID, data, now))
	return event
}

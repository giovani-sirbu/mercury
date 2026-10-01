// Package cooldown is the Cooldown flag's three gates. Two sit on either side
// of the first fill: the first-fill gate (FirstFillHold) — a hold on a local
// top, activated by the higher-highs verdict sophos serves on /cooldown,
// released by price and bounded by FirstFillMaxHold — and depth spacing, the
// gate that keeps a ladder from cascading through every depth in one drop
// (DepthSpacingHold). The third looks past the trade at the wallet:
// depth priority (DepthPriorityHold) makes the shallower ladders of one
// wallet wait while a sibling is short of the depth its grid was sized for.
//
// Every execution of a gate writes a strategy event beside its log row
// (strategyEvents.go). The events are the gates' state; the rows are the
// human-readable output.
package cooldown

import (
	"github.com/giovani-sirbu/mercury/events"
	"github.com/giovani-sirbu/mercury/trades/aggragates"
	"github.com/giovani-sirbu/mercury/trades/gates"
)

// FirstFillHold is the Cooldown flag's whole first-fill gate. The zero Hold
// means the chain may proceed; a refusal names the text of its row and the
// FirstFillEvent that goes beside it (gates.SaveHoldLog writes both). The
// event comes back because on the tick the gate lets an entry through above
// its reference it has written the entered row and its event on the trade;
// the caller must carry that event on, or the pair never reaches updateTrade.
//
// SPOT. The verdict only starts the hold; price ends it. sophos /cooldown
// reports whether the last closed bar of its location interval is a local top
// — too few of the bars before it printed a higher high, so nothing to the
// left has been higher recently. A refused verdict activates the hold at the
// tick price, which is the reference R, and from then on the verdict is not
// fetched again (FirstFillVerdictNeeded): the entry is priced with the
// ladder's own arithmetic (firstFillLevels) and one of three things happens:
//
//   - the price runs UP through up(R). The hold called the wrong direction,
//     the entry goes to market on this tick, and the gate writes the entered
//     event from which NextDepthDoubled asks the next depth to arm at 2p —
//     the ladder is one step closer to the next top than it planned for;
//   - the price falls to arm(R). The entry is armed exactly as a depth is,
//     the anchor follows every full (tr + t) step lower — one event per step —
//     and a t bounce off the anchor fills it exactly as STOPLOSS_TO_BUY does;
//   - anything else is a hold, written once and re-logged daily by
//     gates.SaveHoldLog.
//
// A time cap, FirstFillMaxHold, sits over all three: past it the entry goes
// through at the tick price whatever the band says. An earlier gate expired on
// a fixed wait because a verdict-only hold stayed refused all the way up a
// rally and the trade entered higher for having waited; that cap went away
// once a rally became the first case above and enters on the tick it is
// proven. What removing it exposed is the case a rally never covered — a
// market that drifts sideways INSIDE the band, touching neither edge, where
// the hold would otherwise stand without end — and that is what the constant
// bounds now. The release is silent and writes neither a row nor an event: see
// firstFillExpired and the call site.
//
// Every fact of the hold lives in the trade's first-fill strategy events
// (firstFillState) and nowhere else; the rows written beside them are for the
// operator. trade.PositionPrice is the tick and is NEVER written here:
// the engines read a positive PositionPrice on a new trade as "the trade has
// entered" (sisyphus hasOpenPosition, agora newDepthRequired), so an anchor
// kept there would route a held, funds-blocked trade down the wrong branch.
//
// The gate fails OPEN when it cannot price the hold — no ladder row, a zero
// step, a step of 100% or more, an unknown tick: nothing activates and
// nothing panics.
//
// FUTURES. The ladder rule is spot's. A futures entry keeps the verdict-only
// gate: held while the verdict refuses the side, open the moment it allows
// it or is missing.
//
// `side` is aggragates.EntrySide, not the Inverse flag. sophos scores the two
// directions separately, and taking Inverse here judged every futures entry —
// short ones included — against AllowLongEntry, because a futures trade is
// never marked inverse. On spot the short side IS the inverse ladder and
// every level mirrors. An empty side is no direction to judge, so nothing is
// held.
func FirstFillHold(event events.Events, side string) (events.Events, gates.Hold) {
	trade := event.Trade
	verdict := event.Params.CoolDownIndicators

	if trade.Strategy.TradeType == aggragates.Futures {
		if firstFillVerdictRefuses(side, verdict) {
			verdictHeld := FirstFillEvent{Event: FirstFillVerdictHeld}
			return event, firstFillRefusal(verdictHeld, firstFillVerdictMessage(side))
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
	// The cap. It releases silently: no row and no event is written, and in
	// particular NOT the entered pair — that one means "the price ran up
	// through the reference" and is what asks the second depth to arm at 2p
	// (NextDepthDoubled). A hold that simply ran out of time made no wrong
	// call about direction and earns no correction. The standing hold rows
	// and the fill's own history row are the trace.
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

// firstFillVerdictRefuses is the sophos read for the side the entry would
// take. A missing verdict refuses nothing: sophos is allowed to be
// unreachable, and on spot it is not fetched at all once the hold stands.
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

// appendFirstFillEnteredRow writes the entered pair on the trade, once: the
// INFO row and the entered event beside it, in place like every row the chain
// writes and stamped with the same tick clock. The chain past this gate can
// still stop — hasFunds is next — and then nothing is persisted, so the same
// release may come back on a later tick with the pair missing and write it
// then; a tick that comes back with the event present must not write a second
// pair. The pair carries the tick price and the tick clock like a hold row, so
// the operator reads where and when the reference was passed.
func appendFirstFillEnteredRow(event events.Events, levels firstFillLevels, reference float64) events.Events {
	if firstFillState(event.Trade).enteredAbove {
		return event
	}
	now := event.TickTime()
	data := FirstFillEvent{
		Event:     FirstFillEntered,
		Price:     event.Trade.PositionPrice,
		Reference: reference,
	}
	event.Trade.Logs = append(event.Trade.Logs, aggragates.TradesLogs{
		Message:   firstFillEnteredMessage(event.Trade, levels, data),
		Type:      aggragates.LOG_INFO,
		Price:     data.Price,
		TradeID:   event.Trade.ID,
		CreatedAt: now,
		UpdatedAt: now,
	})
	event.Trade.StrategyEvents = append(event.Trade.StrategyEvents, NewFirstFillEvent(event.Trade.ID, data, now))
	return event
}

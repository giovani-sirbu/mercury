// Package cooldown is the Cooldown flag's two gates, one on each side of the
// first fill: the first-fill gate (FirstFillHold) — a hold on a local top,
// activated by the higher-highs verdict sophos serves on /markers and then
// released by price alone, with no time cap — and depth spacing, the gate
// that keeps a ladder from cascading through every depth in one drop
// (DepthSpacingHoldReason).
package cooldown

import (
	"github.com/giovani-sirbu/mercury/events"
	"github.com/giovani-sirbu/mercury/trades/aggragates"
)

// FirstFillHold is the Cooldown flag's whole first-fill gate. Empty means the
// chain may proceed. The event comes back because on the tick the gate lets
// an entry through above its reference it has written a row on the trade;
// the caller must carry that event on, the way crashguard.ApplyCapitulationOverride's
// caller does, or the row never reaches updateTrade.
//
// SPOT. The verdict only starts the hold; price ends it. sophos /markers
// reports whether the last closed 15m bar is a local top — fewer than five
// higher highs among the forty bars before it (HBAR 2021-10-04 18:45 had
// none). A refused verdict activates the hold at the tick price, which is
// the reference R, and from then on the verdict is not fetched again
// (FirstFillVerdictNeeded): the entry is priced with the ladder's own
// arithmetic (firstFillLevels) and one of three things happens:
//
//   - the price runs UP through up(R). The hold called the wrong direction,
//     the entry goes to market on this tick, and the gate writes the entered
//     row from which NextDepthDoubled asks the next depth to arm at 2p — the
//     ladder is one step closer to the next top than it planned for;
//   - the price falls to arm(R). The entry is armed exactly as a depth is,
//     the anchor follows every full (tr + t) step lower — one row per step —
//     and a t bounce off the anchor fills it exactly as STOPLOSS_TO_BUY does;
//   - anything else is a hold, written once and re-logged daily by
//     gates.SaveHoldLog.
//
// No time cap. The previous gate expired after eight hours (run 97: every
// gain sat in waits under eight hours) because its verdict stayed "expensive"
// all the way up a rally and the trade entered higher for having waited. Here
// a rally is the first case above and enters on the tick it is proven, so
// the cap has nothing left to protect. Product decision 2026-09-05, not yet
// measured on a backtest.
//
// Every fact of the hold lives in the trade's log rows (firstFillState) and
// nowhere else. trade.PositionPrice is the tick and is NEVER written here:
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
func FirstFillHold(event events.Events, side string) (events.Events, string) {
	trade := event.Trade
	verdict := event.Params.CoolDownIndicators

	if trade.Strategy.TradeType == aggragates.Futures {
		if firstFillVerdictRefuses(side, verdict) {
			return event, firstFillVerdictMessage(side)
		}
		return event, ""
	}

	levels, ok := firstFillLevelsFrom(trade, side)
	tick := trade.PositionPrice
	if !ok || tick <= 0 {
		return event, ""
	}

	state := firstFillState(trade)
	if state.enteredAbove {
		return event, ""
	}
	if !state.activated {
		if !firstFillVerdictRefuses(side, verdict) {
			return event, ""
		}
		return event, firstFillWaitingMessage(trade, levels, tick)
	}
	if !state.armed {
		if levels.atOrAbove(tick, levels.up(state.reference)) {
			return appendFirstFillEnteredRow(event, firstFillEnteredMessage(trade, levels, state.reference)), ""
		}
		if levels.atOrBelow(tick, levels.arm(state.reference)) {
			return event, firstFillArmedMessage(trade, levels, state.reference, tick)
		}
		return event, firstFillWaitingMessage(trade, levels, state.reference)
	}
	if levels.above(tick, levels.bounce(state.anchor)) {
		return event, ""
	}
	if levels.below(tick, levels.trail(state.anchor)) {
		return event, firstFillArmedMessage(trade, levels, state.reference, tick)
	}
	return event, firstFillArmedMessage(trade, levels, state.reference, state.anchor)
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

// appendFirstFillEnteredRow writes the entered row on the trade, once. The
// chain past this gate can still stop — hasFunds is next — and then nothing
// is persisted, so the same release may come back on a later tick with the
// row missing and write it then; a tick that comes back with the row
// present must not write a second one. The row carries the tick price and
// the tick clock like a hold row, so the operator reads where and when the
// reference was passed.
func appendFirstFillEnteredRow(event events.Events, message string) events.Events {
	if firstFillState(event.Trade).enteredAbove {
		return event
	}
	now := event.TickTime()
	event.Trade.Logs = append(event.Trade.Logs, aggragates.TradesLogs{
		Message:   message,
		Type:      aggragates.LOG_INFO,
		Price:     event.Trade.PositionPrice,
		TradeID:   event.Trade.ID,
		CreatedAt: now,
		UpdatedAt: now,
	})
	return event
}

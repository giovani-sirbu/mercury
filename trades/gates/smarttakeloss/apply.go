// Package smarttakeloss is the SmartTakeLoss flag: a rule read off the
// pattern window that turns a deep ladder into a seller instead of a buyer.
//
// It is two readings of one window — the last WindowBars closed bars sophos
// serves on GET /:symbol/patterns — both taken against the tick price, and
// both a count of how many of those bars closed their BODY under it:
//
//   - from ArmDepth filled entries, every tick at a price with few enough
//     bars to its left ACTIVATES the trade, with one marker row,
//     "Hold buy: smartTakeLoss: Potential trend reversal",
//     carrying the newest fill's price; nothing is refused on that tick;
//   - from then on the trade sells at the resistance line through the last
//     two lower highs or at the upper Bollinger band. While
//     LastPermittedDepthExit is on it is also permitted PermittedDepths more
//     depths, on the last of which every add-side proposal becomes the exit
//     and the price sells one tolerance under the last fill once the window
//     ALSO says there is no bar to the left at all. That rule is switched
//     OFF: the ladder keeps adding.
//
// The loss-side exit needs no activation and applies from ARMING on: the
// support line sophos draws on the same window through the last two LOWER
// lows, the support a falling market holds (the inverse mirror reads the
// higher highs). While the price holds it the ladder trades normally; once at least
// SupportBreakBars closed bars stayed under the line and the tick price plus
// one tolerance is still under it, the trade sells — and a support the price
// had already bounced from sells after SupportBounceBreakBars closed bars
// under that level with the price under it (supportBreakReached).
//
// No exit fires within MinAgeAfterLastFill of the newest fill — a zero wait
// disables that — and while a wait is on and the trade is on
// its last permitted depth the ladder's add is dropped, with one row per
// fill saying so. Inverse ladders mirror every rule (bars to the left are
// bars that printed OVER the price, support line through two higher lows as
// the bounce target, resistance line through two lower highs as the break,
// lower band, one tolerance over the last fill).
//
// State lives in the trade's own rows only — trade.Logs for the activation,
// trade.History for the fills — and is rebuilt on every tick (rebuildState);
// there is no Redis key, no column and nothing on trade.PositionPrice.
//
// The exit is the engines' existing sellLoss chain (cancelPendingOrder,
// acceptLoss, sell, updateTrade): a limit at the tick price, re-placed one
// tolerance lower by the sellLoss logic row on a further dip. Apply is the
// single entry point; hermes, sisyphus backtesting and sisyphus live-testing
// call it identically after the ladder has chosen a position and write the
// rows it hands back with their own clock (ActivationLog, ExitMessage).
// SmartTakeLoss owns no hold gate in ShouldHold.
//
// Vocabulary: depth, fill, filled entries, tolerance, take profit, stop
// loss, held.
package smarttakeloss

import (
	"time"

	"github.com/giovani-sirbu/mercury/trades/aggragates"
	"github.com/giovani-sirbu/mercury/trades/gates"
)

// Row is a trade-log row Apply asks the engine to write: the activation
// marker with the activating fill's price. The price is the fill, never
// trade.PositionPrice — rebuildState locates the activating fill by it.
type Row struct {
	Message string
	Price   float64
}

// Result is the overlay's answer. Position is the ladder's proposal, or
// "sellLoss" when the rule forced the exit — then Reason names the leg that
// fired ("support line break", "resistance line", "upper bollinger band",
// "tolerance under the last fill", or the inverse mirror) for the engine's
// ExitMessage row, and is empty otherwise. Activation is non-nil on the one
// tick the trade activates and Wait on the one tick a fill's wait first
// refuses the next depth; the engine appends either with ActivationLog.
type Result struct {
	Position   string
	Reason     string
	Activation *Row
	Wait       *Row
}

// Apply overlays the smart take loss on the ladder's proposal for this tick.
// It returns the proposal untouched — and proposes no row — without the
// flag, on an impasse child, without a price or without settings, and on
// any trade below ArmDepth.
//
// The ladder's own closes are never replaced: a proposal in the protected
// set (protectedPosition) passes through, and so does every tick of a trade
// whose STATE already is a close — a trailing take profit proposes nothing
// between −tolerance and the trail and would otherwise be replaced by a
// limit at the tick, and a resting sellLoss limit is re-placed only by its
// own logic row, never by this overlay on every print under it.
func Apply(trade aggragates.Trades, position string, price float64, now time.Time, ai aggragates.AIIndicators) Result {
	result := Result{Position: position}
	if !trade.Strategy.Params.SmartTakeLoss || trade.ParentID != 0 || price <= 0 {
		return result
	}
	if len(trade.StrategyPair.StrategySettings) == 0 {
		return result
	}

	st := rebuildState(trade)
	if !st.armed {
		return result
	}
	if !st.active && activates(trade, st, price, ai.SmartTakeLoss) {
		st.active = true
		st.activationPrice = st.lastFill().Price
		result.Activation = &Row{Message: ActivationMessage(trade.PositionType), Price: st.activationPrice}
	}
	if protectedPosition(gates.PositionType(position)) || protectedPosition(gates.PositionType(trade.PositionType)) {
		return result
	}

	onLastPermittedDepth := LastPermittedDepthExit && st.active && st.depthsAfterActivation >= PermittedDepths
	if !minAgeReached(st, now) {
		// Inside MinAgeAfterLastFill nothing sells — the support break
		// included, it is the same sell. On the last permitted depth no NEW
		// depth is armed either: the add-side proposal is dropped (an empty
		// position runs no chain and is re-proposed on the next tick) rather
		// than turned into the exit. Selling it would break the wait, and
		// letting it through would fill past the permitted depth while the
		// exit waits — and every fill restarts the wait. The remainder of an
		// entry that only partly filled is not an add and keeps filling: it
		// is the permitted depth completing.
		//
		// The refusal overrides the ladder, so it says so once per fill: the
		// row is what tells an operator why a deep trade stopped trading, and
		// the wait itself — no sell while the price sits at a target — writes
		// nothing, because nothing was overridden.
		if onLastPermittedDepth && addSide(position) {
			result.Position = ""
			if !st.waitLogged {
				result.Wait = &Row{Message: WaitMessage(trade.PositionType), Price: st.lastFill().Price}
			}
		}
		return result
	}

	// The support line is read from arming on, activated or not: a ladder
	// that holds it keeps trading, one whose support gave way sells from
	// whatever the ladder proposed on the add side, the dead zone included.
	if reason, hit := supportBreakReached(trade, st, price, now.UnixMilli(), ai.SmartTakeLoss); hit {
		return forced(result, reason)
	}
	if !st.active {
		return result
	}

	if reason, hit := sellTargetReached(trade, st, price, now.UnixMilli(), ai.SmartTakeLoss); hit {
		return forced(result, reason)
	}
	if onLastPermittedDepth {
		// The last permitted depth (LastPermittedDepthExit on), two rules
		// under one reason.
		//
		// Every add-side proposal — the next depth's arming, its re-anchor,
		// the fill itself, a force-trailing re-anchor — becomes the exit,
		// whatever the window says: it commits capital, and refusing it is
		// the whole point of permitting exactly PermittedDepths depths. The
		// arming sits under the tolerance line by the ladder's own
		// construction (−(percentage + tolerance) from the last fill); an
		// arming re-anchored above it after a sell → update_buy round trip is
		// refused under the same reason.
		//
		// The price rule takes BOTH conditions: the depth is spent AND the
		// window has no bar to the left of the price any more. The trade
		// activated with bars still under it; a ladder that merely sits near
		// the bottom of the window is not yet the trade this exit is for, so
		// the price has to be under everything the window holds before one
		// tolerance under the last fill sells.
		if addSide(position) {
			return forced(result, toleranceReason(trade.Inverse))
		}
		if noBarsLeft(trade, price, ai.SmartTakeLoss) && toleranceExitReached(trade, st, price) {
			return forced(result, toleranceReason(trade.Inverse))
		}
	}
	return result
}

func forced(result Result, reason string) Result {
	result.Position = "sellLoss"
	result.Reason = reason
	return result
}

// addSide: the proposals that would commit more capital, the ones this
// overlay exists to replace on the last permitted depth. buy is included
// because the buy chain runs no shouldHold — a trade left in stopLoss by an
// earlier block would otherwise fill past the permitted depth on the bounce.
func addSide(position string) bool {
	switch gates.PositionType(position) {
	case "stopLoss", "update_stopLoss", "buy":
		return true
	}
	return false
}

package smarttakeloss

import (
	"github.com/giovani-sirbu/mercury/trades/aggragates"
	"github.com/giovani-sirbu/mercury/trades/gates"
)

// Row is a trade-log row Apply asks the engine to write: the slow-decline
// marker, its cancel row or its reset row, or the indecision row, carrying
// the newest fill's price. The price is the fill, never trade.PositionPrice —
// rebuildState locates the fill a ladder is pending from by it.
type Row struct {
	Message string
	Price   float64
}

// Result is the overlay's answer. Position is the ladder's proposal, or
// "sellLoss" when a rule forced the exit — then Reason names the rule that
// sold (reasonSellBand, reasonCapitalProtection) for the engine's ExitMessage
// row, and is empty otherwise. SlowDecline is non-nil on the tick a watched
// ladder goes pending, on the tick a new fill on a pending ladder is judged —
// the marker again, or the cancel row — and on the first tick a depth
// priority holds a pending ladder — the reset row. Indecision is non-nil on
// the tick a ladder the indecision direction watches is latched
// (indecisionRow). The engine appends each with LogRow, the slow-decline row
// first. Capital protection hands back no row.
type Result struct {
	Position    string
	Reason      string
	SlowDecline *Row
	Indecision  *Row
}

// Apply overlays the smart take loss on the ladder's proposal for this tick.
// It returns the proposal untouched — and proposes no row — without the
// flag, on an impasse child, without a price or without settings, and on any
// trade no rule watches (slowDeclineWatched, capitalProtectionWatched,
// indecisionWatched).
//
// A ladder a depth priority holds (depthPriorityHeld) is paused: a pending
// exit is reset with one row (slowDeclineReset), and past that the proposal
// comes back untouched — no marker, no judgement of a new fill, no latch, no
// sale at either band — until the ladder's next fill ends the hold.
//
// The slow-decline row and the indecision row go out first, before the
// protected return: a ladder resting in its trailing take profit when the
// verdict arrives is pending all the same, a new fill on it is judged all the
// same (slowDeclineRow), and it is latched all the same (indecisionRow).
// Neither row changes the sale: the indecision direction sells nothing of its
// own, it moves the take profit (TakeProfitPercentage) and the chain of its
// trailing sale (SaleActions).
//
// The ladder's own closes are never replaced: a proposal in the protected set
// (protectedPosition) passes through, and so does every tick of a trade whose
// STATE already is a close — a trailing take profit proposes nothing between
// −tolerance and the trail and would otherwise be replaced by a limit at the
// tick, and a resting sellLoss limit is re-placed only by its own logic row,
// never by this overlay on every print under it. Past that, both sales are
// read from whatever the ladder proposed on the add side, the dead zone
// included: a pending ladder at its sell band first, then a ladder capital
// protection watches at its upper band, so the sell band names a sale both
// rules reach on one tick.
func Apply(trade aggragates.Trades, position string, price float64, ai aggragates.AIIndicators) Result {
	result := Result{Position: position}
	if !trade.Strategy.Params.SmartTakeLoss || trade.ParentID != 0 || price <= 0 {
		return result
	}
	if len(trade.StrategyPair.StrategySettings) == 0 {
		return result
	}

	st := rebuildState(trade)
	if !st.slowDeclineWatched && !st.capitalProtectionWatched && !st.indecisionWatched {
		return result
	}
	if st.depthPriorityHeld {
		_, result.SlowDecline = slowDeclineReset(trade, st)
		return result
	}
	st, result.SlowDecline = slowDeclineRow(trade, st, ai.SmartTakeLoss)
	st, result.Indecision = indecisionRow(trade, st, ai.SmartTakeLoss)
	if protectedPosition(gates.PositionType(position)) || protectedPosition(gates.PositionType(trade.PositionType)) {
		return result
	}

	// Under break even a pending ladder's take profit reads the average entry
	// price alone (TakeProfitPercentage), so the slow decline's one sale there
	// is its sell band; from break even up the ladder proposes its own take
	// profit, a close the protected return above passes through. A ladder
	// this tick cancelled is no longer pending here.
	if st.slowDeclinePending && sellBandReached(price, ai.SmartTakeLoss) {
		return forced(result, reasonSellBand)
	}
	if st.capitalProtectionWatched && capitalProtectionReached(price, ai.SmartTakeLoss) {
		return forced(result, reasonCapitalProtection)
	}
	return result
}

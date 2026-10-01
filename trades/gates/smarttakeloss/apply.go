package smarttakeloss

import (
	"github.com/giovani-sirbu/mercury/trades/aggragates"
	"github.com/giovani-sirbu/mercury/trades/gates"
)

// Row is a decision Apply asks the engine to write, in the two forms every
// gate execution takes: the log row an operator reads (Message at Price) and
// the strategy event beside it, the state a later tick's fold reads (Gate, the
// Event kind, Price and Reasons). Rows builds both from it, and the engine
// writes them under one stamp.
//
// The rows Apply hands back are the slow-decline pending row — going pending,
// or a new fill confirming the exit —, its cancelled row and its reset row,
// and the indecision latched row: PendingRow, CancelledRow, ResetRow and
// LatchedRow build them, for Apply and for a fixture alike, and ExitRow
// builds the sold row of a forced sale. A Row built by hand, without a Gate,
// files its event under no gate, which no fold reads, so callers build every
// Row with those builders. Message is human-readable text, byte-stable for cp
// and the notification filter, and nothing reads it back.
// Price is the newest fill's price, never trade.PositionPrice — rebuildState
// locates the fill a ladder is pending from by it — and the level of the sale
// on a sold row.
type Row struct {
	Message string
	Price   float64
	Gate    string
	Event   string
	Reasons []string
}

// Result is the overlay's answer. Position is the ladder's proposal, or
// "sellLoss" when a rule forced the exit — then Reason names the rule that
// sold (reasonSellBand, reasonCapitalProtection) for the engine's ExitRow,
// and is empty otherwise. SlowDecline is non-nil on the tick a watched
// ladder goes pending, on the tick a new fill on a pending ladder is judged —
// the pending row again, or the cancelled row — and on the first tick a depth
// priority holds a pending ladder — the reset row. Indecision is non-nil on
// the tick a ladder the indecision direction watches is latched
// (indecisionRow). The engine writes each through Rows, the slow-decline row
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
// A ladder a depth priority holds (depthPriorityHeld) is paused, the latch
// excepted: a pending exit is reset with one row (slowDeclineReset), a
// watched ladder the indecision direction reads is latched as on any other
// tick (indecisionRow), and past those two rows the proposal comes back
// untouched — no slow-decline marker, no judgement of a new fill, no sale at
// either band — until the ladder's next fill ends the hold.
//
// The slow-decline row and the indecision row go out first, before the
// protected return: a ladder resting in its trailing take profit when the
// verdict arrives is pending all the same, a new fill on it is judged all the
// same (slowDeclineRow), and it is latched all the same (indecisionRow).
// Neither row changes the sale: the indecision direction sells nothing of its
// own, it moves the take profit (TakeProfitPercentage) and nothing else.
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
		st, result.SlowDecline = slowDeclineReset(trade, st)
		_, result.Indecision = indecisionRow(trade, st, ai.SmartTakeLoss)
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

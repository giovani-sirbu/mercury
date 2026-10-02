package smarttakeloss

import (
	"github.com/giovani-sirbu/mercury/trades/aggragates"
	"github.com/giovani-sirbu/mercury/trades/gates"
)

// Row is a decision Apply asks the engine to write, in the two forms every
// gate execution takes: the log row an operator reads (Message at Price) and
// the strategy event beside it, the state a later tick's fold reads (Gate, the
// Event kind, Price, Features and Reasons). Rows builds both from it, and the
// engine writes them under one stamp.
//
// The rows Apply hands back are the slow decline gate's, one of its own per tick
// at most and after it the held sale's: PendingRow, ConfirmedRow, CancelledRow,
// ResetRow and LatchedRow build the first, SaleHeldRow the second, for Apply and
// for a fixture alike, and ExitRow builds the sold row of a forced sale. A Row
// built by hand, without a Gate, files its event under no gate, which no fold
// reads, so callers build every Row with those builders.
// Message is human-readable text, byte-stable for cp and the notification
// filter, and nothing reads it back. Price is the newest fill's price, never
// trade.PositionPrice — rebuildState locates the fill a ladder is pending from
// by it — and the level of the sale on a sold row.
type Row struct {
	Message  string
	Price    float64
	Gate     string
	Event    string
	Features []string
	Reasons  []string
}

// Result is the overlay's answer. Position is the ladder's proposal, or
// "sellLoss" when a rule forced the exit — then Reason names the rule that
// sold (reasonSellBand, reasonCapitalProtection) for the engine's ExitRow, and
// is empty otherwise. Rows are the rows to write, in order, each with its
// event: the slow decline gate's one row of the tick when it went pending,
// judged a new fill, was reset by a depth priority hold or latched the
// indecision direction, then the held sale's when the loss cap held the sale at
// the band for the first time in its episode, and none otherwise. The engines
// write them, then build the sold row with ExitRow when Reason is set. Capital
// protection hands back no row of its own.
type Result struct {
	Position string
	Reason   string
	Rows     []Row
}

// Apply overlays the smart take loss on the ladder's proposal for this tick.
// It returns the proposal untouched — and proposes no row — without the
// flag, on an impasse child, without a price or without settings, and on any
// trade no rule watches (slowDeclineWatched, capitalProtectionWatched,
// indecisionWatched).
//
// The slow decline gate's tick (slowDeclineTick) goes out first, before the
// protected return: a ladder resting in its trailing take profit when the
// verdict arrives is pending all the same, a new fill on it is judged all the
// same, and it is latched all the same. No row changes the sale: the
// indecision direction sells nothing of its own, it moves the take profit
// (TakeProfitPercentage) and nothing else.
//
// A ladder a depth priority holds (depthPriorityHeld) is paused: a pending gate
// is reset with one row, the sophos reading of the indecision latches as on any
// other tick, and past that row the proposal comes back untouched — no trigger,
// no judgement of a new fill, no sale at either band — until the ladder's next
// fill ends the hold.
//
// The ladder's own closes are never replaced: a proposal in the protected set
// (protectedPosition) passes through, and so does every tick of a trade whose
// STATE already is a close — a trailing take profit proposes nothing between
// −tolerance and the trail and would otherwise be replaced by a limit at the
// tick, and a resting sellLoss limit is re-placed only by its own logic row,
// never by this overlay on every print under it. Past that, both sales are
// read from whatever the ladder proposed on the add side, the dead zone
// included: a pending ladder at its sell band first (sellsAtTheBand), then a
// ladder capital protection watches at its upper band, so the sell band names
// a sale both rules reach on one tick. The sale at the sell band is held while
// its loss is over the cap (bandSaleLoss): that tick sells nothing at the band,
// writes the held sale's row once per episode, and capital protection's band is
// read all the same.
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
	st, row := slowDeclineTick(trade, st, ai.SmartTakeLoss)
	if row != nil {
		result.Rows = append(result.Rows, *row)
	}
	if st.depthPriorityHeld {
		return result
	}
	if protectedPosition(gates.PositionType(position)) || protectedPosition(gates.PositionType(trade.PositionType)) {
		return result
	}

	if sellsAtTheBand(trade, st, price, ai.SmartTakeLoss) {
		lossPct, over := bandSaleLoss(trade, price)
		if !over {
			return forced(result, reasonSellBand)
		}
		if st.saleHeldRowDue() {
			result.Rows = append(result.Rows, SaleHeldRow(st.lastFill().Price, lossPct, st.pendingFeatures...))
		}
	}
	if st.capitalProtectionWatched && capitalProtectionReached(price, ai.SmartTakeLoss) {
		return forced(result, reasonCapitalProtection)
	}
	return result
}

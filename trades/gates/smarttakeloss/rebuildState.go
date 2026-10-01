package smarttakeloss

import (
	"strings"

	"github.com/giovani-sirbu/mercury/trades/aggragates"
)

// state is what the trade's own rows say about the smart take loss: the
// entry fills, where the ladder stands with the quiet slow-decline exit,
// whether capital protection watches it, and where it stands with the
// indecision direction. It is rebuilt from trade.Logs and trade.History on
// every tick, the way cooldown.firstFillState rebuilds the first-fill gate:
// the rows are the only state. Nothing is kept in Redis, in a column or on
// trade.PositionPrice.
type state struct {
	fills []entryFill
	// slowDeclineWatched: the quiet slow-decline exit watches this ladder
	// (slowDeclineWatched). slowDeclinePending: the last slow-decline row of a
	// watched ladder is a marker, so Apply reads its band; a cancel or reset
	// row after it takes that away until the next marker.
	// slowDeclinePendingFrom is that marker's Price: the fill the ladder is
	// pending from, the one slowDeclineFillUnjudged compares its newest fill
	// with. Zero while not pending.
	slowDeclineWatched     bool
	slowDeclinePending     bool
	slowDeclinePendingFrom float64
	// capitalProtectionWatched: the capital protection exit watches this
	// ladder (capitalProtectionWatched).
	capitalProtectionWatched bool
	// indecisionWatched: the indecision direction watches this ladder
	// (indecisionWatched). indecision: a watched ladder carries an indecision
	// row, so it is latched; no row takes the latch away, and it holds until
	// the trade closes.
	indecisionWatched bool
	indecision        bool
	// depthPriorityHeld: a depth priority holds this ladder
	// (depthPriorityHeld), which pauses the quiet slow-decline exit and capital
	// protection on it until its next fill; the indecision direction goes on.
	depthPriorityHeld bool
}

// lastFill is the newest entry fill in slice order, zero when none filled.
func (st state) lastFill() entryFill {
	if len(st.fills) == 0 {
		return entryFill{}
	}
	return st.fills[len(st.fills)-1]
}

// rebuildState folds the rows in slice order — hermes loads them in id order,
// sisyphus appends them. A row is matched by its marker anywhere in the
// message (the rows carry gates.SaveHoldLog's "Hold …: " frame) and must carry
// a price. A slow-decline marker makes a watched ladder pending from the fill
// its price names, and a cancel row or a reset row makes it not pending — the
// last of them wins, and none touches a ladder the exit does not watch, so
// while QuietSlowDeclineExit is off every such row is ignored. The depth
// priority hold is read apart, on every trade, off the rows' stamps and the
// newest fill (depthPriorityHeld). An indecision row latches a ladder the
// indecision direction watches, and nothing takes the latch away; it touches
// no ladder that rule does not watch, so while IndecisionDirection is off
// every such row is ignored too. The two folds are independent: a ladder one
// rule watches folds that rule's rows whether or not the other watches it.
// Every other row is ignored, the activation and wait rows of the retired
// trend-reversal rule that older releases wrote included.
//
// Every watch is read off the fills already folded here: entryFills counts
// them the way ladder.CountFilledEntries does, row for row, so they agree
// with slowDeclineWatched, capitalProtectionWatched and indecisionWatched
// exactly.
func rebuildState(trade aggragates.Trades) state {
	fills := entryFills(trade)
	st := state{
		fills:                    fills,
		slowDeclineWatched:       quietSlowDeclineExit && !trade.Inverse && len(fills) >= SlowDeclineArmDepth,
		capitalProtectionWatched: capitalProtectionEligible(trade) && lastDepthFilled(trade, len(fills)),
		indecisionWatched:        indecisionEligible(trade) && len(fills) >= IndecisionArmDepth,
	}
	st.depthPriorityHeld = depthPriorityHeld(trade, st.lastFill())
	if !st.slowDeclineWatched && !st.indecisionWatched {
		return st
	}
	for _, row := range trade.Logs {
		if row.Price <= 0 {
			continue
		}
		if st.slowDeclineWatched {
			switch {
			case strings.Contains(row.Message, SlowDeclineMarker):
				st.slowDeclinePending = true
				st.slowDeclinePendingFrom = row.Price
			case strings.Contains(row.Message, SlowDeclineCancelMarker), strings.Contains(row.Message, SlowDeclineResetMarker):
				st.slowDeclinePending = false
				st.slowDeclinePendingFrom = 0
			}
		}
		if st.indecisionWatched && strings.Contains(row.Message, IndecisionMarker) {
			st.indecision = true
		}
	}
	return st
}

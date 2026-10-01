package smarttakeloss

import "github.com/giovani-sirbu/mercury/trades/aggragates"

// slowPatternLatchReason is the first reason the indecision latch names when
// the slow pattern decline latches a ladder: the rule that did it, ahead of
// the reasons of the window that held.
const slowPatternLatchReason = "slow pattern decline"

// slowPatternRows is the slow pattern's rows for a tick, with the state as
// they leave it, folded into the result Apply is building: the judgement of a
// pending ladder's new fill (slowPatternJudge), or else the pending row of a
// watched ladder going pending (slowPatternGoesPending), carrying its newest
// fill's price, in Result.SlowDecline — which every engine already writes —
// and, on a pending row, the indecision latch.
//
// The slow pattern shares the slot with the quiet slow decline: when that rule
// has produced its own row on this tick (Result.SlowDecline is set) the slot
// is kept and the pattern is not read, so it lands on a later tick — its read
// window is bars long, a pattern not yet pending cannot sell, and a pending
// pattern with an unjudged fill does not sell (slowPatternSells).
//
// A pending row — going pending or confirmed by a new fill — also latches a
// ladder the indecision direction watches that is not latched yet: one
// LatchedRow at the newest fill's price, naming slowPatternLatchReason and the
// reasons of the window that held, in Result.Indecision, and the ladder is
// latched from that tick on, so its take profit reads its position price too
// and a later reading of the indecision writes no second row. The latch is the
// ordinary one: nothing takes it away, and a cancelled pattern leaves it. A
// ladder the indecision direction does not watch — it is off, or the ladder is
// a futures one — is made pending without a latch.
func slowPatternRows(trade aggragates.Trades, st state, block aggragates.SmartTakeLossIndicators, result Result) (state, Result) {
	if !st.slowPatternWatched || result.SlowDecline != nil {
		return st, result
	}
	var row *Row
	if slowPatternFillUnjudged(trade, st) {
		st, row = slowPatternJudge(trade, st, block)
	} else if reasons, pending := slowPatternGoesPending(st, block); pending {
		st.slowPatternPending = true
		st.slowPatternPendingFrom = st.lastFill().Price
		pendingRow := SlowPatternPendingRow(trade.PositionType, st.slowPatternPendingFrom, reasons)
		row = &pendingRow
	}
	if row == nil {
		return st, result
	}
	result.SlowDecline = row
	if row.Event == EventPending && st.indecisionWatched && !st.indecision {
		st.indecision = true
		latch := LatchedRow(trade.PositionType, st.lastFill().Price, append([]string{slowPatternLatchReason}, row.Reasons...))
		result.Indecision = &latch
	}
	return st, result
}

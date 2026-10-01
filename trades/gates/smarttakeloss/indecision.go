package smarttakeloss

import (
	"github.com/giovani-sirbu/mercury/trades/aggragates"
	"github.com/giovani-sirbu/mercury/trades/ladder"
)

// IndecisionMarker is the text every indecision row carries. Like
// SlowDeclineMarker it is human-readable, byte-stable for cp and the
// notification filter, and never a schema: the latch is the latched event
// beside the row (EventLatched). It neither contains nor is contained in any
// other marker, so a filter that finds a row by its text finds it as the one
// it is.
const IndecisionMarker = "smartTakeLoss: indecision direction, take profit from the last buy"

// IndecisionMessage frames the marker exactly as SlowDeclineMessage frames
// its own — "Hold <positionType>: …" with the trade's raw PositionType — and
// names what sophos read after it when it served any
// (SlowDeclineBreakReasons: each reading that does not hold, then the count
// against the need, then the whole-table SMC trend reading when it held; that
// reading alone on a leg on and quiet). It is a marker row, not a hold:
// nothing is refused on that tick.
func IndecisionMessage(positionType string, reasons []string) string {
	return slowDeclineRowMessage(positionType, IndecisionMarker, reasons)
}

// LatchedRow is the indecision gate's latched row at fill: the marker framed with positionType, naming reasons.
func LatchedRow(positionType string, fill float64, reasons []string) Row {
	return Row{
		Message: IndecisionMessage(positionType, reasons),
		Price:   fill,
		Gate:    GateIndecision,
		Event:   EventLatched,
		Reasons: reasons,
	}
}

// indecisionWatched is whether the indecision direction watches a trade:
// indecisionEligible admits it and it holds IndecisionArmDepth filled entries
// or more. The latched event carries the newest fill's price, so the watch
// needs fills to carry. The flag and the parent check are Armed's and Apply's;
// rebuildState reads the same watch off the fills it has already folded, with
// the same bound. Fills never disappear, so a watched ladder stays watched for
// the rest of its life.
func indecisionWatched(trade aggragates.Trades) bool {
	return indecisionEligible(trade) && ladder.CountFilledEntries(trade) >= IndecisionArmDepth
}

// indecisionEligible is the part of the watch the fills do not decide:
// IndecisionDirection on, a long ladder and a spot one. A futures long keeps
// the take profit from its average entry price, as it does while the slow
// decline holds it pending (TakeProfitPercentage), and hermes' futures path
// never runs the overlay.
func indecisionEligible(trade aggragates.Trades) bool {
	return indecisionDirection && !trade.Inverse && trade.Strategy.TradeType != aggragates.Futures
}

// indecisionRow is the indecision row a tick hands back, if any, and the
// state as that row leaves it: a watched ladder not latched yet, on a tick
// sophos serves the indecision, from either reading (SlowDeclineIndecision:
// the vote's, or the whole SMC trend table bearish), gets the row —
// IndecisionMessage naming SlowDeclineBreakReasons, carrying the newest
// fill's price — and is latched from that tick on. The flag alone decides it,
// whatever SlowDeclineLegQuiet and SlowDeclineExit say: the ladder need not
// be pending, so one whose own smoothness fails, or whose newest fill is
// outside the fill window, latches all the same. A latched ladder gets no
// second row however often sophos serves the reading again, and nothing takes
// the latch away: it holds until the trade closes. A reading without the
// indecision, a ladder the rule does not watch and a ladder already latched
// get nothing. Apply asks it while a depth priority holds the ladder
// (depthPriorityHeld) all the same, so a ladder latches during the hold: the
// hold pauses the quiet slow-decline exit and capital protection, never this
// direction.
func indecisionRow(trade aggragates.Trades, st state, block aggragates.SmartTakeLossIndicators) (state, *Row) {
	if !st.indecisionWatched || st.indecision || !block.SlowDeclineIndecision {
		return st, nil
	}
	st.indecision = true
	row := LatchedRow(trade.PositionType, st.lastFill().Price, block.SlowDeclineBreakReasons)
	return st, &row
}

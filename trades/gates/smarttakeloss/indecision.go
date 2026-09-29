package smarttakeloss

import (
	"github.com/giovani-sirbu/mercury/trades/aggragates"
	"github.com/giovani-sirbu/mercury/trades/ladder"
)

// IndecisionMarker is the text every indecision row carries. Like
// SlowDeclineMarker it is the schema: rebuildState finds the row by this
// marker anywhere in the message (strings.Contains), never by parsing it, so
// the text must stay byte-stable across releases or the rows already written
// stop being found. It neither contains nor is contained in any other marker
// rebuildState reads, so a row is always found as the one it is.
const IndecisionMarker = "smartTakeLoss: indecision direction, take profit from the last buy"

// IndecisionMessage frames the marker exactly as SlowDeclineMessage frames
// its own — "Hold <positionType>: …" with the trade's raw PositionType — and
// names what sophos read after it when it served any
// (SlowDeclineBreakReasons: each reading that does not hold, then the count
// against the need). It is a marker row, not a hold: nothing is refused on
// that tick.
func IndecisionMessage(positionType string, reasons []string) string {
	return slowDeclineRowMessage(positionType, IndecisionMarker, reasons)
}

// indecisionWatched is whether the indecision direction watches a trade:
// indecisionEligible admits it and it holds IndecisionArmDepth filled entries
// or more. The row carries the newest fill's price, so the watch needs fills
// to carry. The flag and the parent check are Armed's and Apply's;
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
// sophos serves the indecision reading on (SlowDeclineIndecision), gets the
// row — IndecisionMessage naming SlowDeclineBreakReasons, carrying the newest
// fill's price — and is latched from that tick on. A latched ladder gets no
// second row however often sophos serves the reading again, and nothing takes
// the latch away: it holds until the trade closes. A reading without the
// indecision, a ladder the rule does not watch and a ladder already latched
// get nothing. Apply does not ask it while a depth priority holds the ladder
// (depthPriorityHeld), so no latch starts during the hold, and one latched
// before it keeps its row while its effects wait for the next fill.
func indecisionRow(trade aggragates.Trades, st state, block aggragates.SmartTakeLossIndicators) (state, *Row) {
	if !st.indecisionWatched || st.indecision || !block.SlowDeclineIndecision {
		return st, nil
	}
	st.indecision = true
	return st, &Row{
		Message: IndecisionMessage(trade.PositionType, block.SlowDeclineBreakReasons),
		Price:   st.lastFill().Price,
	}
}

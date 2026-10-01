package smarttakeloss

import (
	"github.com/giovani-sirbu/mercury/trades/aggragates"
	"github.com/giovani-sirbu/mercury/trades/gates/slowpattern"
	"github.com/giovani-sirbu/mercury/trades/ladder"
)

// SlowPatternMarker is the text every slow pattern pending row carries. Like
// SlowDeclineMarker it is human-readable, byte-stable for cp and the
// notification filter, which find the row by it, and never a schema: what a
// ladder pending from a fill is lives in the pending event beside the row
// (EventPending, under GateSlowPattern), and nothing in this package reads the
// text back. It neither contains nor is contained in any other marker, so a
// filter that finds a row by its text finds it as the one it is.
const SlowPatternMarker = "smartTakeLoss: slow pattern decline, sell at the bollinger band"

// SlowPatternMessage frames the marker the way SlowDeclineMessage frames its
// own — "Hold <positionType>: …" with the trade's raw PositionType — and names
// the reasons of the window that held after it (slowpattern.Reading.Reasons).
// It is a marker row, not a hold: nothing is refused on that tick.
func SlowPatternMessage(positionType string, reasons []string) string {
	return slowDeclineRowMessage(positionType, SlowPatternMarker, reasons)
}

// SlowPatternPendingRow is the slowPattern gate's pending row at fill: the
// marker framed with positionType, naming reasons.
func SlowPatternPendingRow(positionType string, fill float64, reasons []string) Row {
	return Row{
		Message: SlowPatternMessage(positionType, reasons),
		Price:   fill,
		Gate:    GateSlowPattern,
		Event:   EventPending,
		Reasons: reasons,
	}
}

// slowPatternEligible is the part of the watch the fills do not decide:
// SlowPatternDeclineExit on, a long ladder and a spot one — the quiet slow
// decline's reach and the indecision direction's together. A futures long
// keeps the take profit from its average entry price (TakeProfitPercentage),
// and hermes' futures path never runs the overlay. The parent check is
// Apply's and Armed's; an impasse ladder is not excluded.
func slowPatternEligible(trade aggragates.Trades) bool {
	return slowPatternDeclineExit && !trade.Inverse && trade.Strategy.TradeType != aggragates.Futures
}

// slowPatternWatched is whether the slow pattern decline watches a trade:
// slowPatternEligible admits it and it holds SlowPatternArmDepth filled
// entries or more. The pending event carries the newest fill's price, so the
// watch needs fills to carry, and it needs that many because the shape of a
// decline is read between two fills at least slowpattern.SlowPatternMinDepthsBetween
// depths apart. The flag and the parent check are Armed's and Apply's;
// rebuildState reads the same watch off the fills it has already folded, with
// the same bound. Fills never disappear, so a watched ladder stays watched for
// the rest of its life.
func slowPatternWatched(trade aggragates.Trades) bool {
	return slowPatternEligible(trade) && ladder.CountFilledEntries(trade) >= SlowPatternArmDepth
}

// patternFills are the ladder's entry fills as the detector reads them: price
// and stamp, in the order the ladder placed them (entryFills).
func patternFills(fills []entryFill) []slowpattern.Fill {
	read := make([]slowpattern.Fill, len(fills))
	for index, fill := range fills {
		read[index] = slowpattern.Fill{Price: fill.Price, At: fill.At}
	}
	return read
}

// slowPatternGoesPending is the tick a watched ladder goes pending on the
// shape of its decline, with the reasons its row names: a watched ladder not
// pending — never yet, or since a cancelled event — whose newest fill is
// stamped, whose bar sophos' series has closed
// (slowpattern.Series.LastBarOpen at or after slowpattern.BarOpen of the
// fill) and is at most slowpattern.SlowPatternReadBars bars past, and for
// which slowpattern.Trigger holds. Sophos serves no series for a window it
// could not read, and a ladder whose read bars got no tick, or that was
// already deep at deploy, never triggers late: the bound is on the trigger
// only, since the reading is deterministic and a dropped row gets the next
// tick. Apply asks it only on a tick that judges no new fill
// (slowPatternRows).
func slowPatternGoesPending(st state, block aggragates.SmartTakeLossIndicators) ([]string, bool) {
	if !st.slowPatternWatched || st.slowPatternPending {
		return nil, false
	}
	series := slowpattern.SeriesOf(block)
	readBar, lastBar := slowpattern.BarOpen(st.lastFill().At), series.LastBarOpen()
	if readBar == 0 || lastBar < readBar || lastBar-readBar > slowpattern.SlowPatternReadBars*slowpattern.SlowPatternBarMs {
		return nil, false
	}
	reading, holds := slowpattern.Trigger(patternFills(st.fills), series)
	if !holds {
		return nil, false
	}
	return reading.Reasons, true
}

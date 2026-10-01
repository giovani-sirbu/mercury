package smarttakeloss

import (
	"github.com/giovani-sirbu/mercury/trades/aggragates"
	"github.com/giovani-sirbu/mercury/trades/gates/slowpattern"
)

// SlowPatternCancelMarker is the text every row carries that cancels a
// pending ladder's slow pattern exit. Like SlowPatternMarker it is
// human-readable, byte-stable for cp and the notification filter, and never a
// schema: the cancellation is the cancelled event beside the row
// (EventCancelled, under GateSlowPattern). No marker contains another, so a
// filter that finds a row by its text finds it as the one it is.
const SlowPatternCancelMarker = "smartTakeLoss: slow pattern decline not read at the new fill, exit cancelled"

// SlowPatternCancelMessage frames the cancel marker exactly as
// SlowPatternMessage frames its own — "Hold <positionType>: …" with the
// trade's raw PositionType — and names what missed after it
// (slowpattern.Reading.Breaks). Nothing is refused on that tick either.
func SlowPatternCancelMessage(positionType string, reasons []string) string {
	return slowDeclineRowMessage(positionType, SlowPatternCancelMarker, reasons)
}

// SlowPatternCancelledRow is the slowPattern gate's cancelled row at fill: the
// cancel marker framed with positionType, naming what missed.
func SlowPatternCancelledRow(positionType string, fill float64, reasons []string) Row {
	return Row{
		Message: SlowPatternCancelMessage(positionType, reasons),
		Price:   fill,
		Gate:    GateSlowPattern,
		Event:   EventCancelled,
		Reasons: reasons,
	}
}

// slowPatternFillUnjudged is whether a pending ladder's newest fill (in
// history slice order, st.lastFill) is one the exit has not judged: its price
// is not the one the ladder is pending from (st.slowPatternPendingFrom).
//
// The fill that takes the ladder to its last depth (lastDepthFilled, on
// st.fills, counted the way ladder.CountFilledEntries counts them) is never
// judged: the ladder stays pending from where it was, its band live.
//
// The pending events hold prices, not fills, so a later fill at exactly the
// price the ladder is pending from reads as judged. That limit is accepted,
// as the quiet slow decline's is.
func slowPatternFillUnjudged(trade aggragates.Trades, st state) bool {
	if !st.slowPatternPending || st.lastFill().Price == st.slowPatternPendingFrom {
		return false
	}
	return !lastDepthFilled(trade, len(st.fills))
}

// slowPatternSells is whether the pattern's sell band is live: the ladder is
// pending and its newest fill is judged. A pending ladder whose new fill is not
// judged yet — the series not served, the fill's bar not closed, or the slot
// kept by the quiet slow decline's own row — sells nothing until it is.
func slowPatternSells(trade aggragates.Trades, st state) bool {
	return st.slowPatternPending && !slowPatternFillUnjudged(trade, st)
}

// slowPatternJudge judges a pending ladder's unjudged newest fill
// (slowPatternFillUnjudged) once the series sophos serves has closed the bar
// that holds it: the window between the fills, read again with the new fill
// as its newest (slowpattern.Trigger), CONFIRMS the exit when it holds — the
// pending row again, naming the window that held and carrying the new fill's
// price, and the ladder is pending from that fill — and CANCELS it when it
// does not: the cancelled row, naming what missed (slowpattern.Reading.Breaks)
// and carrying the new fill's price, the ladder watched and not pending; only
// slowPatternGoesPending makes it pending again. A window the series cannot
// read cancels too — the new fill's bar older than the series' first bar,
// a newest fill without a stamp — since the pattern is not read, and an unread
// pattern confirms nothing. A series not served, or one that has not closed the
// bar of the new fill yet, is waited on: no row, and the fill stays unjudged,
// which sells nothing (slowPatternSells). There is no freshness bound: the
// reading is deterministic, so a late judgement is the same one.
func slowPatternJudge(trade aggragates.Trades, st state, block aggragates.SmartTakeLossIndicators) (state, *Row) {
	series := slowpattern.SeriesOf(block)
	newest := st.lastFill()
	if !series.Served() || series.LastBarOpen() < slowpattern.BarOpen(newest.At) {
		return st, nil
	}
	reading, holds := slowpattern.Trigger(patternFills(st.fills), series)
	if holds {
		st.slowPatternPendingFrom = newest.Price
		row := SlowPatternPendingRow(trade.PositionType, newest.Price, reading.Reasons)
		return st, &row
	}
	st.slowPatternPending = false
	st.slowPatternPendingFrom = 0
	row := SlowPatternCancelledRow(trade.PositionType, newest.Price, reading.Breaks)
	return st, &row
}

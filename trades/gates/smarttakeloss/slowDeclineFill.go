package smarttakeloss

import "github.com/giovani-sirbu/mercury/trades/aggragates"

// SlowDeclineCancelMarker is the text every row carries that cancels a
// pending ladder's quiet slow-decline exit. Like SlowDeclineMarker it is the
// schema: rebuildState finds the row by this marker anywhere in the message
// (strings.Contains), never by parsing it, so the text must stay byte-stable
// across releases. No marker rebuildState reads contains another, so a row is
// always found as the one it is.
const SlowDeclineCancelMarker = "smartTakeLoss: quiet slow decline broken at the new fill, exit cancelled"

// SlowDeclineCancelMessage frames the cancel marker exactly as
// SlowDeclineMessage frames its own — "Hold <positionType>: …" with the
// trade's raw PositionType — and names what broke after it when sophos served
// any (SlowDeclineBreakReasons). Nothing is refused on that tick either.
func SlowDeclineCancelMessage(positionType string, reasons []string) string {
	return slowDeclineRowMessage(positionType, SlowDeclineCancelMarker, reasons)
}

// slowDeclineRow is the one slow-decline row a tick hands back, if any, and
// the state as that row leaves it: the judgement of a pending ladder's new
// fill (judgeTheNewestFill), or else the marker of a watched ladder going
// pending (slowDeclineGoesPending), carrying its newest fill's price.
func slowDeclineRow(trade aggragates.Trades, st state, block aggragates.SmartTakeLossIndicators) (state, *Row) {
	if judged, row := judgeTheNewestFill(trade, st, block); row != nil {
		return judged, row
	}
	reasons, pending := slowDeclineGoesPending(st, block)
	if !pending {
		return st, nil
	}
	st.slowDeclinePending = true
	st.slowDeclinePendingFrom = st.lastFill().Price
	return st, &Row{
		Message: SlowDeclineMessage(trade.PositionType, reasons),
		Price:   st.slowDeclinePendingFrom,
	}
}

// judgeTheNewestFill judges a pending ladder's unjudged newest fill
// (slowDeclineFillUnjudged) on the first tick sophos serves a reading on: a
// positive sell band. Sophos serves the band whenever it read the window,
// so a zero one is sophos down or no window cached, and the fill waits. Apply
// judges before it reads the band, so the band — served only with a reading
// — never sells on a fill that is not judged yet.
//
// The leg on and quiet, or the decline read recently for the ladder
// (slowDeclineConfirms), CONFIRMS the exit: the marker row again, with the
// reasons of the reading that confirmed it, carrying the new fill's price,
// and the ladder is pending from that fill. Anything else CANCELS it: the
// cancel row, naming what broke (SlowDeclineBreakReasons), carrying the new
// fill's price, and the ladder is watched and not pending; only
// slowDeclineGoesPending makes it pending again. The band stops selling on
// the cancel tick itself, because Apply reads it after the judgement. The
// take profit goes back to the average entry price alone from the next tick:
// the engines read TakeProfitPercentage before Apply, so the cancel row
// reaches it from the tick after the one that wrote it. The row goes back in
// Result.SlowDecline, which every engine already writes.
//
// The reading served on the fill's first tick is that of the last closed bar,
// so a flush that starts and fills inside the forming bar is judged on the
// bar before it. That limit is accepted. A bar among the last closed ones on
// which the verdict stood keeps confirming new fills for as long as sophos
// looks back over it, whatever the last closed bar reads.
func judgeTheNewestFill(trade aggragates.Trades, st state, block aggragates.SmartTakeLossIndicators) (state, *Row) {
	if block.SlowDeclineSellBand <= 0 || !slowDeclineFillUnjudged(trade, st) {
		return st, nil
	}
	newest := st.lastFill()
	if reasons, confirmed := slowDeclineConfirms(newest, block); confirmed {
		st.slowDeclinePendingFrom = newest.Price
		return st, &Row{
			Message: SlowDeclineMessage(trade.PositionType, reasons),
			Price:   newest.Price,
		}
	}
	st.slowDeclinePending = false
	st.slowDeclinePendingFrom = 0
	return st, &Row{
		Message: SlowDeclineCancelMessage(trade.PositionType, block.SlowDeclineBreakReasons),
		Price:   newest.Price,
	}
}

// slowDeclineConfirms is whether a pending ladder's new fill confirms its
// exit, and the reasons the marker row names then: the leg on and quiet on
// the last closed bar (SlowDeclineLegQuiet), naming SlowDeclineExitReasons,
// or else the decline read recently for the ladder from that fill
// (slowDeclineReadsRecently), naming SlowDeclineRecentReasons, whatever the
// ladder's first fill: a pending ladder already stood, and the first fill is
// read only on going pending (slowDeclinePendsRecently).
func slowDeclineConfirms(newest entryFill, block aggragates.SmartTakeLossIndicators) ([]string, bool) {
	if block.SlowDeclineLegQuiet {
		return block.SlowDeclineExitReasons, true
	}
	if slowDeclineReadsRecently(newest, block) {
		return block.SlowDeclineRecentReasons, true
	}
	return nil, false
}

// slowDeclineFillUnjudged is whether a pending ladder's newest fill (in
// history slice order, st.lastFill) is one the exit has not judged: its price
// is not the one the ladder is pending from (st.slowDeclinePendingFrom).
//
// The fill that takes the ladder to its last depth (lastDepthFilled, on
// st.fills, counted the way ladder.CountFilledEntries counts them) is never
// judged: the ladder stays pending from where it was, its band live — and
// capital protection watches it from that fill on.
//
// The rows hold prices, not fills, so a later fill at exactly the price the
// ladder is pending from reads as judged. That limit is accepted.
func slowDeclineFillUnjudged(trade aggragates.Trades, st state) bool {
	if !st.slowDeclinePending || st.lastFill().Price == st.slowDeclinePendingFrom {
		return false
	}
	return !lastDepthFilled(trade, len(st.fills))
}

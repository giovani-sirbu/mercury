package smarttakeloss

import "github.com/giovani-sirbu/mercury/trades/aggragates"

// recentFill is whether a watched ladder bought a depth recently enough for
// the quiet slow decline to make it pending (slowDeclineGoesPending): its
// newest fill, in slice order (st.lastFill), stamped at or after
// SlowDeclineFillFrom, the open time of the oldest of the last closed bars
// sophos serves it for (its SlowDeclineFillBars). Nothing bounds the stamp
// from above, so a fill in the bar still forming counts.
//
// It gates going pending alone, on both paths — the last closed bar's own
// reading and the look-back — and is asked before either. The judgement of a
// pending ladder's new fill does not ask it, since a new fill is recent by
// construction, and neither do the first-fill hold and the indecision latch.
// Once pending, a ladder stays pending however old its newest fill grows,
// until its sale, a cancelled or reset event.
//
// Switched off (SlowDeclineNeedsRecentFill) it always holds. Switched on, it
// fails closed: no fill window served — sophos down, an older sophos, a
// window it did not read — or a newest fill without a stamp keeps the ladder
// from going pending.
func recentFill(newest entryFill, block aggragates.SmartTakeLossIndicators) bool {
	if !slowDeclineNeedsRecentFill {
		return true
	}
	if block.SlowDeclineFillFrom <= 0 || newest.At.IsZero() {
		return false
	}
	return newest.At.UnixMilli() >= block.SlowDeclineFillFrom
}

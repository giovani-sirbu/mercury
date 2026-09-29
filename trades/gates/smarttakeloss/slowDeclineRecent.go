package smarttakeloss

import "github.com/giovani-sirbu/mercury/trades/aggragates"

// slowDeclineReadsRecently is whether the quiet slow decline reads RECENTLY
// for a ladder, given its newest fill: sophos served a bar among its last
// SlowDeclineRecentBars closed bars on which the verdict stood
// (SlowDeclineRecentAt), and the ladder bought a depth within those bars —
// its newest fill stamped at or after SlowDeclineRecentFrom, the open time of
// the oldest of them. Nothing bounds the stamp from above, so a fill in the
// bar still forming counts, and the bar the verdict stood on may come before
// the fill or after it.
//
// It is the second way the decline reads for a ladder, beside the last
// closed bar's own reading: judgeTheNewestFill confirms a pending ladder's
// new fill on either, so a fill that lands while such a bar is among the last
// closed ones never cancels the exit, and slowDeclineGoesPending marks a
// watched ladder pending on it only when that ladder already stood before
// the bar (slowDeclinePendsRecently) and bought a depth within the fill
// window (recentFill). No bar served, no oldest bar served, or a newest fill
// without a stamp, fails closed.
func slowDeclineReadsRecently(newest entryFill, block aggragates.SmartTakeLossIndicators) bool {
	if block.SlowDeclineRecentAt <= 0 || block.SlowDeclineRecentFrom <= 0 || newest.At.IsZero() {
		return false
	}
	return newest.At.UnixMilli() >= block.SlowDeclineRecentFrom
}

// slowDeclinePendsRecently is whether the decline read recently marks a
// watched ladder pending (slowDeclineGoesPending): it reads recently for the
// ladder (slowDeclineReadsRecently), and the ladder already stood when the
// verdict stood on the bar sophos served — its first fill, in slice order,
// stamped strictly before SlowDeclineRecentAt, that bar's open time. A ladder
// opened after that bar goes pending only on the last closed bar's own
// reading: after a sale at the band, a new ladder on the pair fills its first
// depths inside the same look-back, and would otherwise go pending on the bar
// the ladder before it went pending on, and sell out again at the band. A
// first fill without a stamp fails closed. The judgement of a pending
// ladder's new fill does not ask it (slowDeclineConfirms).
func slowDeclinePendsRecently(st state, block aggragates.SmartTakeLossIndicators) bool {
	if len(st.fills) == 0 || !slowDeclineReadsRecently(st.lastFill(), block) {
		return false
	}
	first := st.fills[0]
	return !first.At.IsZero() && first.At.UnixMilli() < block.SlowDeclineRecentAt
}

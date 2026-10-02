package smarttakeloss

import (
	"github.com/giovani-sirbu/mercury/trades/aggragates"
	"github.com/giovani-sirbu/mercury/trades/ladder"
)

// quietDeclineWatched is whether the quiet decline watches a trade:
// QuietDecline on and a long ladder from QuietDeclineArmDepth filled entries.
// The pending event carries the newest fill's price, so the watch needs fills
// to carry; it needs that many because a shallow ladder sold at the band only
// opens the pair to a new one. The flag and the parent check are Armed's;
// rebuildState reads the same watch off the fills it has already folded, with
// the same bound. Fills never disappear, so a watched ladder stays watched for
// the rest of its life.
func quietDeclineWatched(trade aggragates.Trades) bool {
	return quietDecline && !trade.Inverse && ladder.CountFilledEntries(trade) >= QuietDeclineArmDepth
}

// quietDeclineGoesPending is the reading that makes a watched ladder pending on
// the quiet decline, with the reasons the row names: a ladder the gate has not
// made pending — never yet, or since a cancelled or reset event — that bought a
// depth within the fill window sophos serves (recentFill), on a tick the quiet
// decline reads for it on the last closed bar (quietDeclineReadsForTheLadder),
// the row naming SlowDeclineExitReasons, or else reads for it recently with the
// ladder opened before the bar the verdict stood on (quietDeclinePendsRecently),
// the row naming SlowDeclineRecentReasons. The gate asks it with the swing
// decline's reading on every tick it is not pending (goesPending).
func quietDeclineGoesPending(st state, block aggragates.SmartTakeLossIndicators) ([]string, bool) {
	if !st.quietDeclineWatched || st.slowDeclinePending || !recentFill(st.lastFill(), block) {
		return nil, false
	}
	if quietDeclineReadsForTheLadder(st.lastFill(), block) {
		return block.SlowDeclineExitReasons, true
	}
	if quietDeclinePendsRecently(st, block) {
		return block.SlowDeclineRecentReasons, true
	}
	return nil, false
}

// quietDeclineReadsForTheLadder is whether the quiet decline reads for a
// ladder, given its newest fill: sophos' verdict — its leg on and still down
// sophos' SlowDeclineMinLegFallPct from its high close, and its vote passing
// with the smoothness over sophos' own span, which no ladder's own span
// starts before, so it reads for every ladder on the tick it is served — or,
// while the leg is on and quiet (the same leg, the vote passing with a
// ladder's own smoothness), the ladder's own span smooth: its newest fill
// stamped at or after SlowDeclineSmoothFrom, the open time of the earliest
// bar sophos reads the leg smooth from. A fast drop breaks the smooth trend,
// so the pairs of closes before the newest fill do not count, and every new
// fill restarts the count. The fall they are weighed against stays the leg's.
//
// The fill is also stamped strictly before SlowDeclineFillBefore, the open
// time of the bar sophos requires the fill to precede: its
// SlowDeclineMinBarsAfterFill closed bars after the bar that holds the fill,
// and at none the bar after the last closed one, so a fill in the bar still
// forming waits for that bar to close. Sophos serves both bars per symbol, so
// its cached reading stays one per symbol and the ladder's fill is applied
// here. No bar served, or a fill without a stamp, fails closed to the verdict
// alone.
func quietDeclineReadsForTheLadder(newest entryFill, block aggragates.SmartTakeLossIndicators) bool {
	if block.SlowDeclineExit {
		return true
	}
	if !block.SlowDeclineLegQuiet || block.SlowDeclineSmoothFrom <= 0 || block.SlowDeclineFillBefore <= 0 || newest.At.IsZero() {
		return false
	}
	stamp := newest.At.UnixMilli()
	return stamp >= block.SlowDeclineSmoothFrom && stamp < block.SlowDeclineFillBefore
}

// quietDeclineConfirms is whether a pending ladder's new fill holds the quiet
// decline, and the reasons the row names then: the leg on and quiet on the last
// closed bar (SlowDeclineLegQuiet), naming SlowDeclineExitReasons, or else the
// decline read recently for the ladder from that fill
// (quietDeclineReadsRecently), naming SlowDeclineRecentReasons, whatever the
// ladder's first fill: a pending ladder already stood, and the first fill is
// read only on going pending (quietDeclinePendsRecently).
func quietDeclineConfirms(newest entryFill, block aggragates.SmartTakeLossIndicators) ([]string, bool) {
	if block.SlowDeclineLegQuiet {
		return block.SlowDeclineExitReasons, true
	}
	if quietDeclineReadsRecently(newest, block) {
		return block.SlowDeclineRecentReasons, true
	}
	return nil, false
}

package smarttakeloss

import (
	"fmt"
	"strings"

	"github.com/giovani-sirbu/mercury/trades/aggragates"
	"github.com/giovani-sirbu/mercury/trades/ladder"
)

// SlowDeclineMarker is the text every slow-decline pending row carries. It is
// human-readable, byte-stable for cp and the notification filter, which find
// the row by it, and never a schema: what a ladder pending from a fill is
// lives in the pending event beside the row (EventPending), and nothing in
// this package reads the text back.
const SlowDeclineMarker = "smartTakeLoss: quiet slow decline, sell at the bollinger band"

// SlowDeclineMessage frames the marker the way gates.SaveHoldLog frames a
// hold — "Hold <positionType>: …" with the trade's raw PositionType, whatever
// it is — and names the verdict's reasons after it when sophos served any. It
// is a marker row, not a hold: nothing is refused on that tick.
func SlowDeclineMessage(positionType string, reasons []string) string {
	return slowDeclineRowMessage(positionType, SlowDeclineMarker, reasons)
}

// PendingRow is the slowDecline gate's pending row at fill: the marker framed with positionType, naming reasons.
func PendingRow(positionType string, fill float64, reasons []string) Row {
	return Row{
		Message: SlowDeclineMessage(positionType, reasons),
		Price:   fill,
		Gate:    GateSlowDecline,
		Event:   EventPending,
		Reasons: reasons,
	}
}

// slowDeclineRowMessage is the one frame of the slow-decline and indecision
// rows: "Hold <positionType>: <marker>", then the reasons joined in
// parentheses when there are any.
func slowDeclineRowMessage(positionType, marker string, reasons []string) string {
	message := fmt.Sprintf("Hold %s: %s", positionType, marker)
	if len(reasons) == 0 {
		return message
	}
	return message + " (" + strings.Join(reasons, ", ") + ")"
}

// slowDeclineWatched is whether the quiet slow-decline exit watches a trade:
// QuietSlowDeclineExit on and a long ladder from SlowDeclineArmDepth filled
// entries. The pending event carries the newest fill's price, so the watch
// needs fills to carry; it needs that many because a shallow ladder sold at
// the band only opens the pair to a new one. The flag and the parent check
// are Armed's; rebuildState reads the same watch off the fills it has already
// folded, with the same bound. Fills never disappear, so a watched ladder
// stays watched for the rest of its life.
func slowDeclineWatched(trade aggragates.Trades) bool {
	return quietSlowDeclineExit && !trade.Inverse && ladder.CountFilledEntries(trade) >= SlowDeclineArmDepth
}

// slowDeclineGoesPending is the tick a watched ladder goes pending, with the
// pending row, and the reasons that row names: a watched ladder not pending —
// never yet, or since a cancelled or reset event — that bought a depth within
// the fill window sophos serves (recentFill), on a tick the quiet slow
// decline reads for it on the last closed bar
// (slowDeclineReadsForTheLadder), the row naming SlowDeclineExitReasons, or
// else reads for it recently with the ladder opened before the bar the
// verdict stood on (slowDeclinePendsRecently), the row naming
// SlowDeclineRecentReasons. Apply asks it only on a tick that judges no new
// fill (slowDeclineRow).
func slowDeclineGoesPending(st state, block aggragates.SmartTakeLossIndicators) ([]string, bool) {
	if !st.slowDeclineWatched || st.slowDeclinePending || !recentFill(st.lastFill(), block) {
		return nil, false
	}
	if slowDeclineReadsForTheLadder(st.lastFill(), block) {
		return block.SlowDeclineExitReasons, true
	}
	if slowDeclinePendsRecently(st, block) {
		return block.SlowDeclineRecentReasons, true
	}
	return nil, false
}

// slowDeclineReadsForTheLadder is whether the quiet slow decline reads for a
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
func slowDeclineReadsForTheLadder(newest entryFill, block aggragates.SmartTakeLossIndicators) bool {
	if block.SlowDeclineExit {
		return true
	}
	if !block.SlowDeclineLegQuiet || block.SlowDeclineSmoothFrom <= 0 || block.SlowDeclineFillBefore <= 0 || newest.At.IsZero() {
		return false
	}
	stamp := newest.At.UnixMilli()
	return stamp >= block.SlowDeclineSmoothFrom && stamp < block.SlowDeclineFillBefore
}

// sellBandReached is a pending ladder's one sell target: the tick price at
// or over the sell band sophos serves (SlowDeclineSellBand). A zero band —
// sophos down, a window too short to compute it — sells nothing, and the
// ladder stays pending.
func sellBandReached(price float64, block aggragates.SmartTakeLossIndicators) bool {
	return block.SlowDeclineSellBand > 0 && price >= block.SlowDeclineSellBand
}

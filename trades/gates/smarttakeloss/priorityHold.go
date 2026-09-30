package smarttakeloss

import (
	"strings"

	"github.com/giovani-sirbu/mercury/trades/aggragates"
	"github.com/giovani-sirbu/mercury/trades/gates/cooldown"
)

// SlowDeclineResetMarker is the text every row carries that resets a pending
// ladder's quiet slow-decline exit because a depth priority holds the ladder
// (depthPriorityHeld). Like SlowDeclineMarker it is the schema: rebuildState
// finds the row by this marker anywhere in the message (strings.Contains),
// never by parsing it, so the text must stay byte-stable across releases. It
// neither contains nor is contained in any other marker rebuildState reads,
// nor cooldown.DepthPriorityHoldMarker, so a row is always found as the one
// it is.
const SlowDeclineResetMarker = "smartTakeLoss: quiet slow decline reset, depth priority holds this ladder"

// SlowDeclineResetMessage frames the reset marker exactly as
// SlowDeclineMessage frames its own — "Hold <positionType>: …" with the
// trade's raw PositionType. It names no reasons: the hold row beside it names
// the ladder the wallet is kept for. Nothing is refused on that tick.
func SlowDeclineResetMessage(positionType string) string {
	return slowDeclineRowMessage(positionType, SlowDeclineResetMarker, nil)
}

// depthPriorityHeld is whether a depth priority holds the ladder, which
// pauses the smart take loss on it (Apply, TakeProfitPercentage):
// DepthPriorityHoldPausesSmartTakeLoss on, and the trade
// carries a row naming cooldown.DepthPriorityHoldMarker stamped strictly
// after its newest entry fill — newest in slice order, as rebuildState folds
// the fills.
//
// The comparison is the row's CreatedAt against the fill's history stamp
// (entryFill.At). gates.SaveHoldLog stamps the row with the tick clock, and
// the gate holds only an entry that is not placed, so the next fill is
// stamped after the row: the hold lasts until the ladder's next fill, and
// nothing else ends it — not the gate letting the entry through, nor the
// ladder the wallet was kept for closing. The fill's stamp is the order's
// placement tick in backtesting, its reconciliation time in production and
// the wall clock in live-testing; in backtesting a fill of an order placed
// before the row therefore reads as older than the row, the accepted limit. A
// row or a newest fill without a stamp holds nothing, and neither does a
// ladder with no fill.
//
// The rule is literal: a ladder that has filled its last depth and is held
// on the next entry it proposes is held too, so capital protection sells
// nothing on it until it fills again or closes.
func depthPriorityHeld(trade aggragates.Trades, newest entryFill) bool {
	if !depthPriorityHoldPauses || newest.At.IsZero() {
		return false
	}
	for _, row := range trade.Logs {
		if row.CreatedAt.After(newest.At) && strings.Contains(row.Message, cooldown.DepthPriorityHoldMarker) {
			return true
		}
	}
	return false
}

// slowDeclineReset is the one row a held tick hands back, if any, and the
// state as that row leaves it: a pending ladder is reset — the reset row,
// carrying its newest fill's price, and the ladder watched and not pending —
// exactly as a cancel row leaves it, so only slowDeclineGoesPending makes it
// pending again once the hold has ended. A ladder not pending gets nothing,
// so the reset is written once. The row goes back in Result.SlowDecline,
// which every engine already writes.
func slowDeclineReset(trade aggragates.Trades, st state) (state, *Row) {
	if !st.slowDeclinePending {
		return st, nil
	}
	st.slowDeclinePending = false
	st.slowDeclinePendingFrom = 0
	return st, &Row{
		Message: SlowDeclineResetMessage(trade.PositionType),
		Price:   st.lastFill().Price,
	}
}

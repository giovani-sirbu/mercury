package smarttakeloss

import (
	"math"
	"strings"

	"github.com/giovani-sirbu/mercury/trades/aggragates"
)

// TakeProfitPercentage is the move the `buy` row's take profit reads: the
// profitPercentage an engine hands strategies.GetPosition, after its inverse
// negation — the move against the average entry price. At or over break even
// — that move zero or more — two rules raise it, each behind its own switch
// and each read off the trade's own rows:
//
//   - a trade rebuildState reads pending — its last slow-decline row a
//     marker, not a cancel row (QuietSlowDeclineExit) — is also measured from
//     its newest entry fill;
//   - a trade rebuildState reads latched — it carries an indecision row
//     (IndecisionDirection) — is also measured from its position price,
//     trade.PositionPrice: the engines' own `percentage`, the move against
//     the last buy.
//
// The reading is the largest of the moves that apply, so the take profit
// arms at whichever price the tick reaches first, never later than from the
// average entry price alone and never under break even. Under break even the
// input comes back unchanged, exactly as on every other trade — no row, a
// cancelled exit, no fill, an inverse ladder, a child, a futures trade, a
// strategy without the flag, a rule switched off, no price: the `buy` row is
// the spot ladder's, and sisyphus backtesting reads it for its futures trades
// too.
//
// It reads the rows alone, never sophos: a new fill Apply has not judged yet
// leaves the trade pending here, and only a cancel or reset row ends it;
// nothing ends a latch. The engines read it before Apply, so a row Apply
// hands back reaches it from the tick after the one that wrote it. While a
// depth priority holds the ladder (depthPriorityHeld) neither rule moves it:
// the input comes back unchanged, a latch included, until the next fill.
//
// Under break even nothing reads the newest fill or the position price, here
// or in Apply: a pending trade's one sale there is the sell band. A take
// profit armed under break even could only be refused — hasProfit refuses
// every close under the minimum profit — and a proposal it refuses is still
// a close Apply never replaces (protectedPosition), so that refused,
// protected proposal would silence both bands — the sell band and capital
// protection's — on every such tick, and sisyphus backtesting's profit block
// would skip the prints after it.
//
// Only the arming moves. hasProfit in the takeProfit chain still refuses a
// close under the minimum profit, and the trade then stays in `buy`; the
// trailing take profit's sale runs hasProfit as well, latched or not, so a
// close it refuses leaves the trade in `takeProfit` to retry on the next
// print. Between break even and the price the fees and the minimum profit
// clear, that refusal remains: the take profit is proposed and refused there,
// and a `sell` hasProfit refuses leaves the trade in `takeProfit`, a
// protected position too (protectedPosition) that Apply never replaces. The
// exits above go unread on those ticks, and sisyphus backtesting's profit
// block skips the trade's prints between its next depth and that price (the
// trail anchor, for a `sell`) until the block lifts — a known window,
// accepted. The newest fill is the one
// rebuildState folds (entryFills, in history slice order), never
// Position.Price, which a re-anchor moves; the latched rule reads the
// position price on purpose, re-anchor and all, as the `buy` row's own
// `percentage` does.
func TakeProfitPercentage(trade aggragates.Trades, price, profitPercentage float64) float64 {
	if !trade.Strategy.Params.SmartTakeLoss || trade.ParentID != 0 || trade.Inverse || price <= 0 {
		return profitPercentage
	}
	if trade.Strategy.TradeType == aggragates.Futures || profitPercentage < 0 {
		return profitPercentage
	}
	if !carriesTakeProfitMarker(trade) {
		return profitPercentage
	}
	st := rebuildState(trade)
	if st.depthPriorityHeld {
		return profitPercentage
	}
	reading := profitPercentage
	if st.slowDeclinePending {
		fromNewestFill := (price - st.lastFill().Price) / price * 100
		reading = math.Max(reading, fromNewestFill)
	}
	if st.indecision {
		fromPositionPrice := (price - trade.PositionPrice) / price * 100
		reading = math.Max(reading, fromPositionPrice)
	}
	return reading
}

// carriesTakeProfitMarker is the cheap half of the pending and the latched
// tests, asked first because the engines read the take profit on every price
// print of every trade: rebuildState reads a marker row only when it carries
// a price, so a trade with no such row naming SlowDeclineMarker while
// QuietSlowDeclineExit is on, or IndecisionMarker while IndecisionDirection
// is on, can be neither pending nor latched, and the fold is skipped. One
// pass over the rows answers for both markers and stops at the first such
// row.
func carriesTakeProfitMarker(trade aggragates.Trades) bool {
	for _, row := range trade.Logs {
		if row.Price <= 0 {
			continue
		}
		if quietSlowDeclineExit && strings.Contains(row.Message, SlowDeclineMarker) {
			return true
		}
		if indecisionDirection && strings.Contains(row.Message, IndecisionMarker) {
			return true
		}
	}
	return false
}

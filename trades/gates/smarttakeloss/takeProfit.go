package smarttakeloss

import (
	"math"

	"github.com/giovani-sirbu/mercury/trades/aggragates"
)

// TakeProfitPercentage is the move the `buy` row's take profit reads: the
// profitPercentage an engine hands strategies.GetPosition, after its inverse
// negation — the move against the average entry price. At or over break even
// — that move zero or more — two rules raise it, each behind its own switch
// and each read off the trade's own strategy events:
//
//   - a trade rebuildState reads pending — its last slow-decline event a
//     pending event, not a cancelled or reset one (QuietSlowDeclineExit), or
//     its last slow-pattern event a pending event, not a cancelled one
//     (SlowPatternDeclineExit) — is also measured from its newest entry fill;
//   - a trade rebuildState reads latched — it carries a latched event
//     (IndecisionDirection) — is also measured from its position price,
//     trade.PositionPrice: the engines' own `percentage`, the move against
//     the last buy.
//
// The reading is the largest of the moves that apply, so the take profit
// arms at whichever price the tick reaches first, never later than from the
// average entry price alone and never under break even. Under break even the
// input comes back unchanged, exactly as on every other trade — no event, a
// cancelled exit, no fill, an inverse ladder, a child, a futures trade, a
// strategy without the flag, a rule switched off, no price: the `buy` row is
// the spot ladder's, and sisyphus backtesting reads it for its futures trades
// too.
//
// It reads the events alone, never sophos and never the text of a log row: a
// new fill Apply has not judged yet leaves the trade pending here, and only a
// cancelled or reset event ends it; nothing ends a latch. The engines read it
// before Apply, so a row Apply hands back reaches it from the tick after the
// one that wrote its event. While a depth priority holds the ladder
// (depthPriorityHeld) the pending reading — the move from the newest fill,
// whichever rule made the ladder pending — waits for the next fill, while the
// latched reading — the move from the position price — applies all the same:
// the hold pauses the pending exits, never the indecision direction.
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
	if !carriesTakeProfitEvent(trade) {
		return profitPercentage
	}
	st := rebuildState(trade)
	reading := profitPercentage
	if (st.slowDeclinePending || st.slowPatternPending) && !st.depthPriorityHeld {
		fromNewestFill := (price - st.lastFill().Price) / price * 100
		reading = math.Max(reading, fromNewestFill)
	}
	if st.indecision {
		fromPositionPrice := (price - trade.PositionPrice) / price * 100
		reading = math.Max(reading, fromPositionPrice)
	}
	return reading
}

// carriesTakeProfitEvent is the cheap half of the pending and the latched
// tests, asked first because the engines read the take profit on every price
// print of every trade: a trade with no smartTakeLoss event on the
// slow-decline gate while QuietSlowDeclineExit is on, on the slow-pattern gate
// while SlowPatternDeclineExit is on, or on the indecision gate while
// IndecisionDirection is on, can be neither pending nor latched, and the fold
// is skipped. One pass over the events answers for all three gates by their
// filing alone — no document is decoded — and stops at the first such event.
func carriesTakeProfitEvent(trade aggragates.Trades) bool {
	for _, event := range trade.StrategyEvents {
		if event.Param != aggragates.StrategyParamSmartTakeLoss {
			continue
		}
		if quietSlowDeclineExit && event.Gate == GateSlowDecline {
			return true
		}
		if slowPatternDeclineExit && event.Gate == GateSlowPattern {
			return true
		}
		if indecisionDirection && event.Gate == GateIndecision {
			return true
		}
	}
	return false
}

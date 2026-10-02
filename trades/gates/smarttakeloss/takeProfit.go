package smarttakeloss

import (
	"math"

	"github.com/giovani-sirbu/mercury/trades/aggragates"
)

// TakeProfitPercentage is the move the `buy` row's take profit reads: the
// profitPercentage an engine hands strategies.GetPosition, after its inverse
// negation — the move against the average entry price. At or over break even
// — that move zero or more — two readings raise it, each behind its own switch
// and each read off the trade's own strategy events:
//
//   - a trade rebuildState reads pending — the slow decline gate's last pending
//     event not followed by a cancelled or reset one, on a feature that is
//     switched on (QuietDecline, SwingDecline) — is also measured from its
//     newest entry fill, whichever feature made it pending;
//   - a trade rebuildState reads latched — it carries an event that latched it
//     (Indecision) — is also measured from its position price,
//     trade.PositionPrice: the engines' own `percentage`, the move against
//     the last buy.
//
// The reading is the largest of the moves that apply, so the take profit
// arms at whichever price the tick reaches first, never later than from the
// average entry price alone and never under break even. Under break even the
// input comes back unchanged, exactly as on every other trade — no event, a
// cancelled exit, no fill, an inverse ladder, a child, a futures trade, a
// strategy without the flag, a feature switched off, no price: the `buy` row is
// the spot ladder's, and sisyphus backtesting reads it for its futures trades
// too.
//
// It reads the events alone, never sophos and never the text of a log row: a
// new fill Apply has not judged yet leaves the trade pending here, and only a
// cancelled or reset event ends it; nothing ends a latch. The engines read it
// before Apply, so a row Apply hands back reaches it from the tick after the
// one that wrote its event. While a depth priority holds the ladder
// (depthPriorityHeld) the pending reading — the move from the newest fill —
// waits for the next fill, while the latched reading — the move from the
// position price — applies all the same: the hold pauses the gate, never the
// indecision direction.
//
// Under break even nothing reads the newest fill or the position price, here
// or in Apply: a pending trade's one sale there is the sell band. A take
// profit armed under break even could only be refused — hasProfit refuses
// every close under the minimum profit — and a proposal it refuses is still
// a close Apply never replaces (protectedPosition), so that refused,
// protected proposal would silence both bands — the sell band and capital
// protection's — on every such tick, and sisyphus backtesting's profit
// block would skip the prints after it.
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
	if st.slowDeclinePending && !st.depthPriorityHeld {
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
// print of every trade: a trade with no smartTakeLoss event on the gate that
// could make it pending or latched can be neither, and the fold is skipped. The
// slow decline gate's events count while any of its features is on, the events
// of the gates it replaced while the feature they drove is on (gateSlowPattern
// for SwingDecline, gateIndecision for Indecision). One pass over the events
// answers for all of them by their filing alone — no document is decoded — and
// stops at the first such event.
func carriesTakeProfitEvent(trade aggragates.Trades) bool {
	for _, event := range trade.StrategyEvents {
		if event.Param != aggragates.StrategyParamSmartTakeLoss {
			continue
		}
		switch event.Gate {
		case GateSlowDecline:
			if quietDecline || swingDecline || indecision {
				return true
			}
		case gateSlowPattern:
			if swingDecline {
				return true
			}
		case gateIndecision:
			if indecision {
				return true
			}
		}
	}
	return false
}

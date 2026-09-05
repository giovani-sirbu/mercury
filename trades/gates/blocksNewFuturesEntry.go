package gates

import "github.com/giovani-sirbu/mercury/trades/aggragates"

// BlocksNewFuturesEntry is the futures pre-chain skip: whether the engine must
// refuse to open a position on this tick.
//
// It answers two questions, and the first one is unconditional. A futures entry
// has NO direction of its own — the verdict is the side createFuturesOrders
// submits — so without a LONG or a SHORT there is nothing to open. That matters
// far beyond a wasted tick: the first action of the futures entry chain,
// CheckOldFuturesOrders, cancels every open order on the symbol and
// market-closes any position there, and it runs before CreateFuturesOrders ever
// reaches its own empty-side guard. A strategy with no sophos flags fetches no
// verdict at all, and a strategy whose sophos leg fails degrades to an empty
// one; both used to walk straight into that.
//
// The second question is the flagged HOLD veto: an explicit "stay out" from the
// route whose flag is on.
func BlocksNewFuturesEntry(params aggragates.StrategyParams, ai aggragates.AIIndicators) bool {
	if ai.AIAction != aggragates.ActionLong && ai.AIAction != aggragates.ActionShort {
		return true
	}
	if params.UseAI && ai.AIAction == aggragates.ActionHold {
		return true
	}
	if params.UsePatterns && ai.PatternAction == aggragates.ActionHold {
		return true
	}

	return false
}

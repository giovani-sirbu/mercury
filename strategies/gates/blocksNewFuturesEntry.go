package gates

import "github.com/giovani-sirbu/mercury/trades/aggragates"

// BlocksNewFuturesEntry is the futures pre-chain skip: a verdict with no LONG or SHORT side opens
// nothing (CheckOldFuturesOrders would otherwise cancel and close on the symbol first), and a HOLD
// from a route whose flag is on vetoes the entry.
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

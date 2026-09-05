package gates

import "github.com/giovani-sirbu/mercury/trades/aggragates"

// BlocksNewFuturesEntry is the futures pre-chain HOLD skip: ML HOLD and
// pattern HOLD each veto a first fill when their flag is on. Futures has no
// cooldown lens, so this is the one first-fill pattern read that stays.
func BlocksNewFuturesEntry(params aggragates.StrategyParams, ai aggragates.AIIndicators) bool {
	if params.UseAI && ai.AIAction == aggragates.ActionHold {
		return true
	}
	if params.UsePatterns && ai.PatternAction == aggragates.ActionHold {
		return true
	}
	return false
}

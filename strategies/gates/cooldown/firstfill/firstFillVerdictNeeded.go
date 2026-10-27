package firstfill

import "github.com/giovani-sirbu/mercury/trades/aggragates"

// FirstFillVerdictNeeded is whether the tick has to fetch the sophos /cooldown verdict: only a new
// trade; on every tick on futures; otherwise on spot only until the hold activates or the entry was
// released, since price decides from then on.
func FirstFillVerdictNeeded(trade aggragates.Trades, position string) bool {
	if position != "new" {
		return false
	}
	if trade.Strategy.TradeType == aggragates.Futures {
		return true
	}
	state := firstFillState(trade)
	return !state.activated && !state.enteredAbove
}

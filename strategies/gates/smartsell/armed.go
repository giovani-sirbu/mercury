package smartsell

import "github.com/giovani-sirbu/mercury/trades/aggragates"

// Armed is whether the smart sell watches this trade (the flag on, a parent, one rule
// watching); the engines ask it before their empty-position early return, since the overlay acts
// on ticks where the ladder proposes nothing. A depth priority pause keeps it armed.
func Armed(trade aggragates.Trades) bool {
	if !trade.Strategy.Params.SmartSell || trade.ParentID != 0 {
		return false
	}
	return slowDeclineWatched(trade) || capitalProtectionWatched(trade) || earlyTakeProfitWatched(trade) ||
		flatMarketWatched(trade) || freeFallPreventionWatched(trade)
}

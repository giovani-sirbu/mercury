package smarttakeloss

import "github.com/giovani-sirbu/mercury/trades/aggragates"

// Armed is whether the smart take loss watches this trade (the flag on, a parent, one rule
// watching); the engines ask it before their empty-position early return, since the overlay acts
// on ticks where the ladder proposes nothing. A depth priority pause keeps it armed.
func Armed(trade aggragates.Trades) bool {
	if !trade.Strategy.Params.SmartTakeLoss || trade.ParentID != 0 {
		return false
	}
	return slowDeclineWatched(trade) || capitalProtectionWatched(trade) || earlyTakeProfitWatched(trade) ||
		flatMarketWatched(trade) || freeFallPreventionWatched(trade)
}

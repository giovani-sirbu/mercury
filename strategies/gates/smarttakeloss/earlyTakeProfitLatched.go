package smarttakeloss

import "github.com/giovani-sirbu/mercury/trades/aggragates"

// EarlyTakeProfitLatched is whether the early take profit stands latched on the trade's events,
// which nothing takes away; nothing on a trade without the flag, a child or with
// earlytakeprofit.Enabled off.
func EarlyTakeProfitLatched(trade aggragates.Trades) bool {
	if !trade.Strategy.Params.SmartTakeLoss || trade.ParentID != 0 {
		return false
	}
	return rebuildState(trade).earlyTakeProfit
}

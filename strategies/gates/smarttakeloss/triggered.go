package smarttakeloss

import "github.com/giovani-sirbu/mercury/trades/aggragates"

// Triggered is whether a gate stands triggered on the trade's events: the slow decline gate pending,
// or a flat market, free fall prevention or capital protection episode open at the newest fill; a
// flat market or free fall prevention episode with its reset off (flatmarket.ResetOnNewDepth,
// freefallprevention.ResetOnNewDepth) at any fill.
func Triggered(trade aggragates.Trades) bool {
	if !trade.Strategy.Params.SmartTakeLoss || trade.ParentID != 0 {
		return false
	}
	st := rebuildState(trade)
	fill := st.lastFill().Price
	return st.slowDeclinePending ||
		(st.flatMarketActive && (st.flatMarketFrom == fill || !flatMarketResetOnNewDepth)) ||
		(st.freeFallPreventionActive && (st.freeFallPreventionFrom == fill || !freeFallPreventionResetOnNewDepth)) ||
		(st.capitalProtectionActive && st.capitalProtectionFrom == fill)
}

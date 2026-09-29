package smarttakeloss

import "github.com/giovani-sirbu/mercury/trades/aggragates"

// Armed reports whether the smart take loss watches this trade: the flag is
// on, the trade is a parent (impasse children belong to the impasse chain),
// and one of the three rules watches the ladder — the quiet slow decline a
// long ladder from SlowDeclineArmDepth filled entries (slowDeclineWatched),
// capital protection a long spot ladder outside an impasse strategy from its
// last configured depth (capitalProtectionWatched), the indecision direction
// a long spot ladder from IndecisionArmDepth filled entries
// (indecisionWatched). A rule switched off watches no ladder. hermes and
// live-testing ask it before their empty-position early return, because the
// exits fire exactly on the ticks where the ladder proposes nothing: a bounce
// into the sell band or the upper band lands in the dead zone between the
// next depth and the take profit, and so can the tick a watched ladder goes
// pending or is latched by the indecision direction. Those engines therefore
// run their full tick path on every print for such a trade, not only on the
// ticks the ladder proposes something. Fills never disappear, so a watched
// ladder stays armed; there is no separate predicate to ask. A ladder a depth
// priority holds (depthPriorityHeld) stays armed on purpose: the pause is
// Apply's to answer — it writes the reset row on the first held tick, the
// dead zone included, and sells nothing — and the pause ends with the
// ladder's next fill, from which the rules read it again on every print.
func Armed(trade aggragates.Trades) bool {
	if !trade.Strategy.Params.SmartTakeLoss || trade.ParentID != 0 {
		return false
	}
	return slowDeclineWatched(trade) || capitalProtectionWatched(trade) || indecisionWatched(trade)
}

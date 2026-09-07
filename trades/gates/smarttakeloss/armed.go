package smarttakeloss

import "github.com/giovani-sirbu/mercury/trades/aggragates"

// Armed reports whether the smart take loss watches this trade: the flag is
// on, the trade is a parent (impasse children belong to the impasse chain)
// and the ladder holds at least ArmDepth filled entries. hermes and
// live-testing ask it before their empty-position early return, because the
// exits fire exactly on the ticks where the ladder proposes nothing: a
// bounce into the line or the band lands in the dead zone between the next
// depth and the take profit. Fills never disappear, so an activated trade is
// always armed; there is no separate "active" predicate to ask.
func Armed(trade aggragates.Trades) bool {
	return trade.Strategy.Params.SmartTakeLoss && trade.ParentID == 0 && armed(trade)
}

package smarttakeloss

import (
	"github.com/giovani-sirbu/mercury/trades/aggragates"
	"github.com/giovani-sirbu/mercury/trades/ladder"
)

// capitalProtectionWatched is whether the capital protection exit watches a
// trade: a ladder capitalProtectionEligible admits that has filled its last
// configured depth (lastDepthFilled). The flag and the parent check are
// Armed's and Apply's; rebuildState reads the same watch off the fills it has
// already folded. Fills never disappear, so a watched ladder stays watched
// for the rest of its life.
func capitalProtectionWatched(trade aggragates.Trades) bool {
	return capitalProtectionEligible(trade) && lastDepthFilled(trade, ladder.CountFilledEntries(trade))
}

// capitalProtectionEligible is the part of the watch the fills do not decide:
// CapitalProtectionExit on, a long ladder, a spot one — hermes' futures path
// never runs the overlay, and a forced sellLoss on a futures trade in a
// backtest writes its row and sells nothing — and a strategy without Impasse,
// whose parent reaches `buy` again through sellParent and update_buy while it
// still has children that a forced sale would orphan.
func capitalProtectionEligible(trade aggragates.Trades) bool {
	return capitalProtectionExit &&
		!trade.Inverse &&
		trade.Strategy.TradeType != aggragates.Futures &&
		!trade.Strategy.Params.Impasse
}

// lastDepthFilled is whether a ladder holding filled entries has filled its
// last configured depth: ladder.ConfiguredDepths known, and at least that
// many entries filled. A partial fill of an entry order counts, as in
// ladder.CountFilledEntries.
func lastDepthFilled(trade aggragates.Trades, filled int) bool {
	depths := ladder.ConfiguredDepths(trade)
	return depths > 0 && filled >= depths
}

// capitalProtectionReached is a watched ladder's one sale: sophos' SMC trend
// reading bearish (CapitalProtectionSmcBearish) and the tick price at or over
// the upper band it serves beside it (CapitalProtectionUpperBB). There is no
// tolerance and no memory: both are read afresh on every tick, so a trend
// that stops reading bearish before the price reaches the band sells nothing.
// A zero band — sophos down, a window too short to compute it — sells
// nothing either.
func capitalProtectionReached(price float64, block aggragates.SmartTakeLossIndicators) bool {
	return block.CapitalProtectionSmcBearish &&
		block.CapitalProtectionUpperBB > 0 &&
		price >= block.CapitalProtectionUpperBB
}

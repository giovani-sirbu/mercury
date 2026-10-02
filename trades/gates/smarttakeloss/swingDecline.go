package smarttakeloss

import (
	"github.com/giovani-sirbu/mercury/trades/aggragates"
	"github.com/giovani-sirbu/mercury/trades/gates/swingdecline"
	"github.com/giovani-sirbu/mercury/trades/ladder"
)

// swingDeclineEligible is the part of the watch the fills do not decide:
// SwingDecline on, a long ladder and a spot one — the quiet decline's reach and
// the indecision direction's together. A futures long keeps the take profit
// from its average entry price (TakeProfitPercentage), and hermes' futures path
// never runs the overlay. The parent check is Apply's and Armed's; an impasse
// ladder is not excluded.
func swingDeclineEligible(trade aggragates.Trades) bool {
	return swingDecline && !trade.Inverse && trade.Strategy.TradeType != aggragates.Futures
}

// swingDeclineWatched is whether the swing decline watches a trade:
// swingDeclineEligible admits it and it holds SwingDeclineArmDepth filled
// entries or more. The pending event carries the newest fill's price, so the
// watch needs fills to carry, and it needs that many because the shape of a
// decline is read between two fills at least
// swingdecline.SwingDeclineMinDepthsBetween depths apart. The flag and the
// parent check are Armed's and Apply's; rebuildState reads the same watch off
// the fills it has already folded, with the same bound. Fills never disappear,
// so a watched ladder stays watched for the rest of its life.
func swingDeclineWatched(trade aggragates.Trades) bool {
	return swingDeclineEligible(trade) && ladder.CountFilledEntries(trade) >= SwingDeclineArmDepth
}

// swingFills are the ladder's entry fills as the detector reads them: price and
// stamp, in the order the ladder placed them (entryFills).
func swingFills(fills []entryFill) []swingdecline.Fill {
	read := make([]swingdecline.Fill, len(fills))
	for index, fill := range fills {
		read[index] = swingdecline.Fill{Price: fill.Price, At: fill.At}
	}
	return read
}

// swingDeclineReads is the swing decline's reading of a watched ladder at its
// newest fill, and whether it holds: the newest fill is stamped, its bar
// is closed in sophos' series (swingdecline.Series.LastBarOpen at or after
// swingdecline.BarOpen of the fill) and is at most
// swingdecline.SwingDeclineReadBars bars past, and swingdecline.Trigger holds:
// a window that holds on its shape and the SMC trend dashboard's Trend
// Direction (swingdecline.TrendOf) down on enough rows. Sophos serves no series
// for a window it could not read, and a ladder whose read bars got no tick, or
// that was already deep at deploy, never triggers late: the bound is on the
// trigger only, since the shape is deterministic and a dropped row gets the
// next tick. The trend is the tick's own, as of the series' last bar, so a
// trend that turns down after the fill's bar still triggers inside the read
// bars, and one sophos did not serve never does. The gate asks it for a ladder
// it has not made pending (goesPending) and, for the indecision latch, for one
// pending on a judged fill (swingReadsOnAJudgedFill); the judgement of a new
// fill reads the same window without the bound (newFillReadings).
func swingDeclineReads(st state, block aggragates.SmartTakeLossIndicators) (swingdecline.Reading, bool) {
	if !st.swingDeclineWatched {
		return swingdecline.Reading{}, false
	}
	series := swingdecline.SeriesOf(block)
	readBar, lastBar := swingdecline.BarOpen(st.lastFill().At), series.LastBarOpen()
	if readBar == 0 || lastBar < readBar || lastBar-readBar > swingdecline.SwingDeclineReadBars*swingdecline.SwingDeclineBarMs {
		return swingdecline.Reading{}, false
	}
	return swingdecline.Trigger(swingFills(st.fills), series, swingdecline.TrendOf(block))
}

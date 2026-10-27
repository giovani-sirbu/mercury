package smarttakeloss

import (
	"github.com/giovani-sirbu/mercury/strategies/gates/smarttakeloss/series"
	"github.com/giovani-sirbu/mercury/strategies/gates/smarttakeloss/swingdecline"
	"github.com/giovani-sirbu/mercury/trades/aggragates"
	"github.com/giovani-sirbu/mercury/trades/ladder"
)

// swingDeclineEligible is the part of the watch the fills do not decide: swingdecline.Enabled and a
// long spot ladder.
func swingDeclineEligible(trade aggragates.Trades) bool {
	return swingDecline && !trade.Inverse && trade.Strategy.TradeType != aggragates.Futures
}

// swingDeclineWatched is whether the swing decline watches a trade: swingDeclineEligible and at
// least swingdecline.ArmDepth filled entries.
func swingDeclineWatched(trade aggragates.Trades) bool {
	return swingDeclineEligible(trade) && ladder.CountFilledEntries(trade) >= swingdecline.ArmDepth
}

// swingFills are the entry fills as the detector reads them, in placement order.
func swingFills(fills []entryFill) []series.Fill {
	read := make([]series.Fill, len(fills))
	for index, fill := range fills {
		read[index] = series.Fill{Price: fill.Price, At: fill.At}
	}
	return read
}

// swingDeclineReads is the swing decline's reading of the ladder as of the tick
// (swingdecline.TriggerAtLastBar under the tick's Trend Direction, asked first), remembered per
// ladder, depth, bar and trend (swingMemo); it fails closed, and its callers check the watch or the arm depth.
func swingDeclineReads(st state, block aggragates.SmartTakeLossIndicators) (swingdecline.Reading, bool) {
	trend := series.TrendOf(block)
	if !swingdecline.MeetsTheNeed(trend) {
		return swingdecline.Reading{}, false
	}
	return swingMemo.answer(st.fills, series.SeriesOf(block), trend)
}

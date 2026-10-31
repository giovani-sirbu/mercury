package smartsell

import (
	"github.com/giovani-sirbu/mercury/strategies/gates/smartsell/lineardecline"
	"github.com/giovani-sirbu/mercury/strategies/gates/smartsell/series"
	"github.com/giovani-sirbu/mercury/trades/aggragates"
	"github.com/giovani-sirbu/mercury/trades/ladder"
)

// linearDeclineEligible is the part of the read the fills do not decide: lineardecline.Enabled and
// a long spot ladder, as earlyTakeProfitEligible.
func linearDeclineEligible(trade aggragates.Trades) bool {
	return linearDecline && !trade.Inverse && trade.Strategy.TradeType != aggragates.Futures
}

// linearDeclineWatched is whether the linear decline reads a trade: linearDeclineEligible and at
// least lineardecline.ArmDepth filled entries.
func linearDeclineWatched(trade aggragates.Trades) bool {
	return linearDeclineEligible(trade) && ladder.CountFilledEntries(trade) >= lineardecline.ArmDepth
}

// linearDeclineReads is the linear decline's reading for the early take profit latch (linearDeclineTriggers)
// of a ladder the latch watches, on a tick with no row and not held, with earlytakeprofit.LinearDecline
// on, whatever lineardecline.Enabled says.
func linearDeclineReads(st state, block aggragates.SmartSellIndicators, found tick) (lineardecline.Reading, bool) {
	if found.kind != rowNone || st.depthPriorityHeld || !earlyTakeProfitLinearDecline || !st.earlyTakeProfitWatched {
		return lineardecline.Reading{}, false
	}
	return linearDeclineTriggers(st, block)
}

// linearDeclineTriggers is the linear decline's reading of a ladder holding lineardecline.ArmDepth
// fills as of the tick (lineardecline.TriggerAt under the SMC trend, asked first) and whether it
// holds; nothing for a shallower ladder or an unstamped newest fill.
func linearDeclineTriggers(st state, block aggragates.SmartSellIndicators) (lineardecline.Reading, bool) {
	if len(st.fills) < lineardecline.ArmDepth || st.lastFill().At.IsZero() {
		return lineardecline.Reading{}, false
	}
	trend := series.TrendOf(block)
	if !lineardecline.MeetsTheNeed(trend) {
		return lineardecline.Reading{}, false
	}
	first, previous, newest := linearDeclineFills(st.fills)
	return lineardecline.TriggerAt(first, previous, newest, len(st.fills), series.SeriesOf(block), trend)
}

// linearDeclineFills are the three fills the rule reads, the window's first (lineardecline.Window
// back from the newest), the one before the newest and the newest, without allocating; zero fills
// for a ladder shallower than the window.
func linearDeclineFills(fills []entryFill) (first, previous, newest series.Fill) {
	start := len(fills) - lineardecline.Window
	if start < 0 {
		return series.Fill{}, series.Fill{}, series.Fill{}
	}
	last := len(fills) - 1
	return series.Fill(fills[start]), series.Fill(fills[last-1]), series.Fill(fills[last])
}

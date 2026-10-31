package smartsell

import (
	"github.com/giovani-sirbu/mercury/strategies/gates/smartsell/flatmarket"
	"github.com/giovani-sirbu/mercury/strategies/gates/smartsell/series"
	"github.com/giovani-sirbu/mercury/trades/aggragates"
)

// earlyTakeProfitFlatMarketReads is the flat market's latch reading (flatmarket.LatchTrigger: the
// range on the bars alone, the line under the trend) on a ladder holding flatmarket.ArmDepth fills,
// whatever flatmarket.Enabled says, and whether it holds. The side is long: earlyTakeProfitEligible
// admits no other.
func earlyTakeProfitFlatMarketReads(trade aggragates.Trades, st state, block aggragates.SmartSellIndicators, found tick) (flatmarket.Reading, bool) {
	if found.kind != rowNone || st.depthPriorityHeld || !earlyTakeProfitFlatMarket || len(st.fills) < flatmarket.ArmDepth {
		return flatmarket.Reading{}, false
	}
	reading := flatmarket.LatchTrigger(flatmarket.BarsOf(block), basePercentage(trade), aggragates.SideLong, series.TrendOf(block))
	return reading, reading.Holds
}

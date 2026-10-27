package ladder

import "github.com/giovani-sirbu/mercury/trades/aggragates"

// DepthOf is the wallet view of one trade with every configured depth
// costed: DepthOfOffset at 0.
func DepthOf(trade aggragates.Trades) aggragates.LadderDepth {
	return DepthOfOffset(trade, 0)
}

// DepthOfOffset is the wallet view of one trade: how deep its ladder has
// filled, how deep it may fill, and what the depths it has left still cost in
// the asset it spends, priced from its position and from its last fill. The
// costs stop offset depths short of the ceiling. Everything is measured on
// the rows the ladder trades (tradedSettings), and every surface builds the
// view through this helper so a replay and production agree on a depth.
func DepthOfOffset(trade aggragates.Trades, offset int) aggragates.LadderDepth {
	// Counted once, handed to both costs.
	settings := tradedSettings(trade)
	filled := CountFilledEntries(trade)
	ceiling := ceilingOf(settings, filled)
	costed := ceiling - offset

	asset, remainingCost := remainingCostAt(trade, settings, filled, costed)

	return aggragates.LadderDepth{
		TradeID:              trade.ID,
		Symbol:               trade.Symbol,
		Depth:                filled,
		MaxDepth:             ceiling,
		Asset:                asset,
		RemainingCost:        remainingCost,
		PlannedRemainingCost: plannedRemainingCostAt(trade, settings, filled, costed),
	}
}

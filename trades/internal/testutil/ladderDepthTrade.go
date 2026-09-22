package testutil

import "github.com/giovani-sirbu/mercury/trades/aggragates"

// LadderDepthTrade is a long under the Cooldown flag that has filled the
// given number of entries, one distinct exchange order each, on a pair
// configured for maxDepths of them. One settings row, so it governs every
// depth.
//
// The history rows carry no stamps, on purpose. The wallet gate reads depths
// and never a clock, and an unstamped ladder is exactly what makes depth
// spacing fail open — so a test of the one gate can never be answered by the
// other.
func LadderDepthTrade(id uint, symbol string, depth int, maxDepths float64) aggragates.Trades {
	trade := NewHoldTrade("stopLoss", false)
	trade.ID = id
	trade.Symbol = symbol
	trade.Status = aggragates.Active
	trade.Strategy.Params.Cooldown = true
	trade.StrategyPair.StrategySettings = []aggragates.StrategySettings{
		{Percentage: 2.5, Tolerance: 0.15, TrailingTakeProfit: 0.75, Multiplier: 2.2, Depths: maxDepths},
	}

	for index := 0; index < depth; index++ {
		trade.History = append(trade.History, aggragates.TradesHistory{
			Type:     "BUY",
			Quantity: 1,
			Price:    100 - float64(index),
			OrderId:  int64(index + 1),
		})
	}

	return trade
}

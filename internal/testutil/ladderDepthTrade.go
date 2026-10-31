package testutil

import "github.com/giovani-sirbu/mercury/trades/aggragates"

// ladderDepthMultiplier grows each fixture entry over the one before, in the
// settings row and in the filled quantities alike, so the ladder prices as planned.
const ladderDepthMultiplier = 2

// ladderDepthFirstQuantity is the base quantity of the fixture's first entry.
const ladderDepthFirstQuantity = 1

// ladderDepthPrice is where the fixture's position sits; a long entry is costed at it.
const ladderDepthPrice = 100

// LadderDepthTrade is a long under the Cooldown flag that has filled depth
// entries, one exchange order each, on a pair configured for maxDepths of
// them on one settings row. The fills grow by the row's multiplier so the
// ladder has a real cost, and the history rows carry no stamps so depth
// spacing fails open and never answers for the wallet gate.
func LadderDepthTrade(id uint, symbol string, depth int, maxDepths float64) aggragates.Trades {
	trade := NewHoldTrade(aggragates.PositionTrailingBuy, false)
	trade.ID = id
	trade.Symbol = symbol
	trade.Status = aggragates.Active
	trade.PositionPrice = ladderDepthPrice
	trade.Strategy.Params.Cooldown = true
	trade.StrategyPair.TradeFilters = DefaultTradeFilters()
	trade.StrategyPair.StrategySettings = []aggragates.StrategySettings{
		{Percentage: 2.5, Tolerance: 0.15, TrailingTakeProfit: 0.75, Multiplier: ladderDepthMultiplier, Depths: maxDepths},
	}

	quantity := float64(ladderDepthFirstQuantity)
	for index := 0; index < depth; index++ {
		trade.History = append(trade.History, aggragates.TradesHistory{
			Type:     "BUY",
			Quantity: quantity,
			Price:    ladderDepthPrice - float64(index),
			OrderId:  int64(index + 1),
		})
		quantity *= ladderDepthMultiplier
	}

	return trade
}

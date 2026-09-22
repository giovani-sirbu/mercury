package testutil

import "github.com/giovani-sirbu/mercury/trades/aggragates"

// ladderDepthMultiplier is how much each entry of the fixture ladder grows
// over the one before it, in both the settings row and the quantities already
// filled. The two have to agree: the wallet gate prices the entries a ladder
// has left by multiplying its LAST filled quantity forward, so a fixture
// whose fills do not follow its own multiplier would be priced against a
// ladder that never existed.
const ladderDepthMultiplier = 2

// ladderDepthFirstQuantity is the base quantity of the fixture's first entry.
const ladderDepthFirstQuantity = 1

// ladderDepthPrice is where the fixture's position sits. Every long entry is
// costed at the position price, so the fixture needs one for its ladder to
// cost anything at all.
const ladderDepthPrice = 100

// LadderDepthTrade is a long under the Cooldown flag that has filled the
// given number of entries, one distinct exchange order each, on a pair
// configured for maxDepths of them. One settings row, so it governs every
// depth.
//
// The fills grow by the row's own multiplier and the trade carries a position
// price and the pair's minimum, so the ladder has a real cost: what it has
// left to spend, and what its next entry would spend, are the two numbers the
// wallet reserve is decided on. A flat fixture would make every ladder cost
// the same and hide the rule behind its depths.
//
// The history rows carry no stamps, on purpose. The wallet gate reads depths
// and costs and never a clock, and an unstamped ladder is exactly what makes
// depth spacing fail open — so a test of the one gate can never be answered
// by the other.
func LadderDepthTrade(id uint, symbol string, depth int, maxDepths float64) aggragates.Trades {
	trade := NewHoldTrade("stopLoss", false)
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

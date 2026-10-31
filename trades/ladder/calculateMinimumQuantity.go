package ladder

import "github.com/giovani-sirbu/mercury/trades/aggragates"

func CalculateMinimumQuantity(trade aggragates.Trades) float64 {
	strategySettings := trade.StrategyPair.StrategySettings[0]
	depth := int(strategySettings.MinDepths)
	initial := trade.StrategyPair.TradeFilters.MinNotional
	percent := strategySettings.Percentage

	// Base-asset rungs double undiscounted — see CalculateInitialBid.
	if trade.Inverse {
		percent = 0
	}

	latestSum := initial * (1 + (percent / 100))
	var neededSum float64 = 0

	for i := 1; i < depth; i++ {
		latestSum = (latestSum - (latestSum * (percent / 100))) * 2
		neededSum += latestSum
	}

	// increase amount by 5%
	neededSum *= 1.05

	// handle inverse
	if trade.Inverse {
		neededSum /= trade.PositionPrice
	}

	return neededSum
}

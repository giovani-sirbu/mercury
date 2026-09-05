package quantities

import (
	"math"

	"github.com/giovani-sirbu/mercury/helpers"
	"github.com/giovani-sirbu/mercury/trades/aggragates"
)

// CalculateMinOrderQty returns the minimum amount based on lotSize (decimal places) and minNotional
func CalculateMinOrderQty(trade aggragates.Trades) float64 {
	if trade.StrategyPair.TradeFilters.MinNotional == 0 ||
		trade.StrategyPair.TradeFilters.LotSize == 0 ||
		trade.PositionPrice == 0 {
		return 0
	}

	quantity := trade.StrategyPair.TradeFilters.MinNotional / trade.PositionPrice

	if !trade.Inverse {
		quantity += math.Pow(10, -float64(trade.StrategyPair.TradeFilters.LotSize))
	}

	return helpers.ToFixed(quantity, int(trade.StrategyPair.TradeFilters.LotSize))
}

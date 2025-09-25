package actions

import (
	"github.com/giovani-sirbu/mercury/trades/aggragates"
)

func CalculateMinProfit(trade aggragates.Trades) float64 {
	profit := trade.StrategyPair.TradeFilters.MinNotional * (trade.StrategyPair.StrategySettings[0].Percentage / 100)
	if trade.Inverse {
		profit /= trade.PositionPrice
	}
	return profit
}

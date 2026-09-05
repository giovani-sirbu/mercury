package testutil

import "github.com/giovani-sirbu/mercury/trades/aggragates"

// DeepTrade holds four distinct filled entries at 100/98/96/94, one unit
// each: 388 quote invested, average entry 97. Four is crashguard.DeRiskMinDepth.
func DeepTrade(crashGuard bool) aggragates.Trades {
	trade := aggragates.Trades{
		Symbol:        "BTC/USDT",
		PositionType:  "stopLoss",
		PositionPrice: 94,
		Strategy: aggragates.Strategies{
			Params: aggragates.StrategyParams{CrashGuard: crashGuard},
		},
		StrategyPair: aggragates.StrategiesPairs{
			StrategySettings: []aggragates.StrategySettings{
				{Percentage: 2, Tolerance: 0.5},
			},
		},
	}
	prices := []float64{100, 98, 96, 94}
	for i, price := range prices {
		trade.History = append(trade.History, aggragates.TradesHistory{
			Type: "BUY", Quantity: 1, Price: price, OrderId: int64(i + 1),
		})
	}
	return trade
}

package testutil

import "github.com/giovani-sirbu/mercury/trades/aggragates"

// DeepLadderTrade holds N distinct filled entries, one unit each, on a
// single-row ladder sized for 9 depths. Long entries step down from 100 by
// 2 (8 entries: 744 quote invested, break even 93); inverse entries are
// SELLs stepping up from 100 by 2 (8 entries: 856 quote held, break even 107).
func DeepLadderTrade(entries int, inverse bool) aggragates.Trades {
	trade := aggragates.Trades{
		Symbol:       "BTC/USDT",
		PositionType: "stopLoss",
		Inverse:      inverse,
		Strategy: aggragates.Strategies{
			Params: aggragates.StrategyParams{SmartTakeLoss: true},
		},
		StrategyPair: aggragates.StrategiesPairs{
			StrategySettings: []aggragates.StrategySettings{
				{Percentage: 2, Tolerance: 0.5, Depths: 9},
			},
		},
	}
	side, step := "BUY", -2.0
	if inverse {
		side, step = "SELL", 2.0
	}
	for i := 0; i < entries; i++ {
		price := 100 + step*float64(i)
		trade.History = append(trade.History, aggragates.TradesHistory{
			Type: side, Quantity: 1, Price: price, OrderId: int64(i + 1),
		})
		trade.PositionPrice = price
	}
	return trade
}

package testutil

import "github.com/giovani-sirbu/mercury/trades/aggragates"

// MakeTrade creates a trade with sane defaults for symbol, filters, and settings.
func MakeTrade(symbol string, positionPrice float64, inverse bool, history []aggragates.TradesHistory) aggragates.Trades {
	return aggragates.Trades{
		Symbol:        symbol,
		PositionPrice: positionPrice,
		Inverse:       inverse,
		History:       history,
		StrategyPair: aggragates.StrategiesPairs{
			TradeFilters:     DefaultTradeFilters(),
			StrategySettings: DefaultStrategySettings(),
		},
	}
}

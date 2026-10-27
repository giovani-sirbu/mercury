// Package testutil holds the assertions and trade fixtures shared by the tests
// of the strategies and trades packages. It imports only events and
// aggragates, so any package under strategies or trades may use it from its
// internal tests without a cycle.
package testutil

import "github.com/giovani-sirbu/mercury/trades/aggragates"

// NewHoldTrade is the minimal trade the hold-gate tests start from: one
// ladder row, which governs every depth, and the pair's price precision,
// both read by the cooldown first-fill gate.
func NewHoldTrade(positionType string, inverse bool) aggragates.Trades {
	return aggragates.Trades{
		Symbol:       "BTC/USDT",
		PositionType: positionType,
		Inverse:      inverse,
		StrategyPair: aggragates.StrategiesPairs{
			TradeFilters: aggragates.TradeFilters{PriceFilter: 4},
			StrategySettings: []aggragates.StrategySettings{
				{Percentage: 2.5, Tolerance: 0.15, TrailingTakeProfit: 0.75, Multiplier: 2.2, Depths: 8},
			},
		},
	}
}

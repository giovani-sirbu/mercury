// Package testutil holds the assertions and trade fixtures shared by the tests
// of the trades packages. It imports only events and aggragates, so any
// package under trades may use it from its internal tests without a cycle.
package testutil

import "github.com/giovani-sirbu/mercury/trades/aggragates"

// NewHoldTrade is the minimal BTC/USDT trade the hold-gate tests start from.
//
// It carries one ladder row and the pair's price precision because the
// cooldown first-fill gate reads both: percentage, tolerance and trailing
// take profit set the levels the hold waits for, and PriceFilter is how those
// levels are printed in the hold row. The row is the HBAR/USDT shape the rule
// was specified against (2.5 / 0.15 / 0.75), on a single row so it governs
// every depth.
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

package testutil

import "github.com/giovani-sirbu/mercury/trades/aggragates"

// DefaultStrategySettings returns standard strategy settings for testing.
func DefaultStrategySettings() []aggragates.StrategySettings {
	return []aggragates.StrategySettings{
		{MinDepths: 6, Depths: 8, Percentage: 2, Multiplier: 2, Tolerance: 0.25, InitialBid: 0.5},
	}
}

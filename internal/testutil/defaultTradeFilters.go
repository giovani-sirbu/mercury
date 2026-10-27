package testutil

import "github.com/giovani-sirbu/mercury/trades/aggragates"

// DefaultTradeFilters returns standard trade filters for testing.
func DefaultTradeFilters() aggragates.TradeFilters {
	return aggragates.TradeFilters{LotSize: 2, MinNotional: 5, PriceFilter: 3}
}

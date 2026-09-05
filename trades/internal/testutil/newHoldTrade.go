// Package testutil holds the assertions and trade fixtures shared by the tests
// of the trades packages. It imports only events and aggragates, so any
// package under trades may use it from its internal tests without a cycle.
package testutil

import "github.com/giovani-sirbu/mercury/trades/aggragates"

// NewHoldTrade is the minimal BTC/USDT trade the hold-gate tests start from.
func NewHoldTrade(positionType string, inverse bool) aggragates.Trades {
	return aggragates.Trades{
		Symbol:       "BTC/USDT",
		PositionType: positionType,
		Inverse:      inverse,
	}
}

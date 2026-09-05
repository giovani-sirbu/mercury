package testutil

import (
	"time"

	"github.com/giovani-sirbu/mercury/trades/aggragates"
)

// DepthTrade is a long under the Cooldown flag whose ladder was placed at the
// given stamps, one distinct exchange order each.
func DepthTrade(placements ...time.Time) aggragates.Trades {
	trade := NewHoldTrade("stopLoss", false)
	trade.Strategy.Params.Cooldown = true
	for index, placement := range placements {
		trade.History = append(trade.History, aggragates.TradesHistory{
			Type:      "BUY",
			Quantity:  1,
			Price:     100 - float64(index),
			OrderId:   int64(index + 1),
			CreatedAt: placement,
		})
	}
	return trade
}

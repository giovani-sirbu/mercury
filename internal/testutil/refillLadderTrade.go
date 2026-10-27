package testutil

import "github.com/giovani-sirbu/mercury/trades/aggragates"

// RefillLadderTrade is LadderDepthTrade with the two readings of what a
// ladder has left set apart: firstQuantity scales the planned cost the gate
// ranks level ladders on, positionPrice moves the reserve it keeps and nothing
// it ranks on, so a test can tell which of the two the gate reads.
func RefillLadderTrade(id uint, symbol string, depth int, maxDepths, firstQuantity, positionPrice float64) aggragates.Trades {
	trade := LadderDepthTrade(id, symbol, depth, maxDepths)
	trade.PositionPrice = positionPrice

	quantity := firstQuantity
	for index := range trade.History {
		trade.History[index].Quantity = quantity
		quantity *= ladderDepthMultiplier
	}

	return trade
}

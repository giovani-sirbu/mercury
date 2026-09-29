package testutil

import "github.com/giovani-sirbu/mercury/trades/aggragates"

// RefillLadderTrade is LadderDepthTrade with the two readings of what a ladder
// has left set apart, the way a wallet refill finds two ladders at the same
// depth.
//
// firstQuantity sizes the first entry, and every fill after it grows from it
// by the row's multiplier, so it scales everything the ladder planned from
// its fills — the key the depth-priority gate ranks level ladders on.
// positionPrice is where the position sits now, which a re-arm or a trail
// moves while the fills stay where they are — so it moves the reserve the
// gate keeps and nothing it ranks on. Chosen together they can put one ladder
// ahead on the plan and behind on the reserve, which is the only way a test
// can tell which of the two the gate reads.
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

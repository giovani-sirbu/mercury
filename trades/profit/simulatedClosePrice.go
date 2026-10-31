package profit

import (
	"github.com/giovani-sirbu/mercury/helpers"
	"github.com/giovani-sirbu/mercury/trades/aggragates"
)

// ArmHaircutTolerances is how many tolerances under its anchor an armed takeProfit's close is valued:
// one down to the sale line, one for the first print that gaps under it.
const ArmHaircutTolerances = 2.0

// SimulatedClosePrice is the price the profit gates value the close at: the price itself in a chain
// that sells on this pass, the anchor less ArmHaircutTolerances tolerances in one that arms.
func SimulatedClosePrice(trade aggragates.Trades) float64 {
	if closesOnThisPass(trade) {
		return helpers.ToFixed(trade.PositionPrice, int(trade.StrategyPair.TradeFilters.PriceFilter))
	}

	return subtractToleranceFromPrice(trade)
}

// closesOnThisPass reports whether the chain places the closing order on this
// pass (sell, sellParent and takeLoss run Sell beside their gate) rather than
// arming an exit a later tick closes (takeProfit). The engines set
// PositionType to the position being taken, without the update_ prefix.
func closesOnThisPass(trade aggragates.Trades) bool {
	switch trade.PositionType {
	case aggragates.PositionSell, aggragates.PositionSellParent, aggragates.PositionTakeLoss:
		return true
	}

	return false
}

// subtractToleranceFromPrice is the anchor moved ArmHaircutTolerances tolerances against the trade, floored to the price filter.
func subtractToleranceFromPrice(trade aggragates.Trades) float64 {
	toleranceAmount := trade.PositionPrice * (ArmHaircutTolerances * trade.StrategyPair.StrategySettings[0].Tolerance / 100)
	if trade.Inverse {
		trade.PositionPrice += toleranceAmount
	} else {
		trade.PositionPrice -= toleranceAmount
	}

	return helpers.ToFixed(trade.PositionPrice, int(trade.StrategyPair.TradeFilters.PriceFilter))
}

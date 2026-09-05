package profit

import (
	"github.com/giovani-sirbu/mercury/helpers"
	"github.com/giovani-sirbu/mercury/trades/aggragates"
)

// SimulatedClosePrice is the price the profit gates value the close at.
//
// The tolerance haircut forecasts ONE price move: the drop between arming a
// takeProfit and the exit that follows it, which the strategy only triggers
// once the price has fallen `tolerance` below the anchor (GetLogic's takeProfit
// row). Charging it while arming is right — the exit really will land that much
// lower. Charging it again in the exit chain is not: by then the drop has
// already happened and is already in PositionPrice, and Sell submits its order
// AT PositionPrice (sell.go), not at a haircut price. The trade paid tolerance
// twice for one move, so an exit needed 2*tolerance + fees of headroom over its
// average cost while the ladder only guarantees one — closing depths whose
// whole sell window sat below break-even and re-arming them into `buy` instead.
// ParentTradeHasProfit already prices its own leg at the raw PositionPrice.
func SimulatedClosePrice(trade aggragates.Trades) float64 {
	if closesOnThisPass(trade) {
		return helpers.ToFixed(trade.PositionPrice, int(trade.StrategyPair.TradeFilters.PriceFilter))
	}

	return subtractToleranceFromPrice(trade)
}

// closesOnThisPass reports whether the chain the gate is running in will place
// the closing order on this very pass, rather than arming a position that will
// close later. GetActionsByPosition is the authority: "sell", "sellParent" and
// "sellLoss" all run Sell in the same chain as their gate, while "takeProfit"
// and "update_takeProfit" only re-anchor and hand the exit to a later tick.
//
// The engines set Trade.PositionType to the position being taken before the
// chain starts (hermes handleTrade, sisyphus ManageTrade), stripping any
// "update_" prefix, so this reads the position the chain is acting on and not
// the one it came from.
func closesOnThisPass(trade aggragates.Trades) bool {
	switch trade.PositionType {
	case "sell", "sellParent", "sellLoss":
		return true
	}

	return false
}

// deduct strategy settings tolerance from position price to simulate unrealised PnL
func subtractToleranceFromPrice(trade aggragates.Trades) float64 {
	toleranceAmount := trade.PositionPrice * (trade.StrategyPair.StrategySettings[0].Tolerance / 100)
	if trade.Inverse {
		trade.PositionPrice += toleranceAmount
	} else {
		trade.PositionPrice -= toleranceAmount
	}

	return helpers.ToFixed(trade.PositionPrice, int(trade.StrategyPair.TradeFilters.PriceFilter))
}

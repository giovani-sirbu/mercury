package smarttakeloss

import (
	"math"

	"github.com/giovani-sirbu/mercury/trades/aggragates"
)

// TakeProfitPercentage raises the buy row's take profit move (against the average entry) at or over
// break even by the readings that apply: a pending ladder also measures from its newest fill, a
// latched one from its position price, the largest winning; otherwise the input comes back unchanged.
func TakeProfitPercentage(trade aggragates.Trades, price, profitPercentage float64) float64 {
	if !trade.Strategy.Params.SmartTakeLoss || trade.ParentID != 0 || trade.Inverse || price <= 0 {
		return profitPercentage
	}
	if trade.Strategy.TradeType == aggragates.Futures || profitPercentage < 0 {
		return profitPercentage
	}
	if !carriesTakeProfitEvent(trade) {
		return profitPercentage
	}
	st := rebuildState(trade)
	reading := profitPercentage
	if st.slowDeclinePending && !st.depthPriorityHeld {
		fromNewestFill := (price - st.lastFill().Price) / price * 100
		reading = math.Max(reading, fromNewestFill)
	}
	if st.earlyTakeProfit {
		fromPositionPrice := (price - trade.PositionPrice) / price * 100
		reading = math.Max(reading, fromPositionPrice)
	}
	return reading
}

// carriesTakeProfitEvent is the cheap test before the fold, asked on every print: the trade carries
// an event of a gate that could make it pending or latched while that feature is on.
func carriesTakeProfitEvent(trade aggragates.Trades) bool {
	for _, event := range trade.StrategyEvents {
		if event.Param != aggragates.StrategyParamSmartTakeLoss {
			continue
		}
		switch event.Gate {
		case GateSlowDecline:
			if quietDecline || swingDecline || linearDecline || earlyTakeProfit {
				return true
			}
		case gateSlowPattern:
			if swingDecline {
				return true
			}
		case gateIndecision:
			if earlyTakeProfit {
				return true
			}
		}
	}
	return false
}

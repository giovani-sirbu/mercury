package smartsell

import "github.com/giovani-sirbu/mercury/trades/aggragates"

// activationState is the state an activation moves buy to by the sign of the profit it measured:
// takeProfit at or over break even, trailingBuy under it.
func activationState(profitPct float64) string {
	if profitPct >= 0 {
		return aggragates.PositionTakeProfit
	}
	return aggragates.PositionTrailingBuy
}

// activationStep is the activation's step by the sign of the profit it measured (activationState).
func activationStep(profitPct float64) saleStep {
	if activationState(profitPct) == aggragates.PositionTakeProfit {
		return stepActivateTakeProfit
	}
	return stepActivateTrailingBuy
}

// saleTerminal is the state a sale at price ends in by its realised result (saleProfit at a zero
// trigger): sell at or over break even, takeLoss under it or with nothing used.
func saleTerminal(trade aggragates.Trades, price float64) string {
	if _, standing := saleProfit(trade, price, 0); standing == reached {
		return aggragates.PositionSell
	}
	return aggragates.PositionTakeLoss
}

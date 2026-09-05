// Package ladder is the DCA ladder's arithmetic: how it is sized (the initial
// bid, the minimum quantity), which settings row a depth reads, how many
// depths are filled and what the fills hold. It imports only aggragates, so
// every trades package and every service may depend on it.
package ladder

import (
	"fmt"
	"math"

	"github.com/giovani-sirbu/mercury/trades/aggragates"
)

// InitialBidReservePercent is the wallet slice held out of the depth ladder's
// budget. Fees, LotSize rounding and the minimum-quantity bump all charge the
// wallet outside the ladder math; without a reserve they defund the last rung.
const InitialBidReservePercent = 20.0

func CalculateInitialBid(amount float64, trade aggragates.Trades, strategyIndex int) (float64, error) {
	var initialBid float64
	var initialBidInQuote float64
	var depth float64
	isEligible := false
	strategySettings := trade.StrategyPair.StrategySettings[strategyIndex]

	maxDepths := strategySettings.Depths
	minDepths := strategySettings.MinDepths
	decimalDecrease := 0.5

	// Size the ladder against a haircut budget, never the full wallet, so the
	// deepest rung stays fundable after fees and rounding.
	budget := amount * (1 - InitialBidReservePercent/100)

	// An inverse ladder spends the base asset, and every rung doubles the base
	// quantity with the bare multiplier — the percentage discount only shapes
	// the quote proceeds. Sizing inverse with the discounted ratio plans ~88%
	// of the ladder's real cost and blocks every max-depth trade on its final
	// rung.
	percentage := strategySettings.Percentage
	if trade.Inverse {
		percentage = 0
	}

	for depthMultiplied := maxDepths * 100; depthMultiplied >= minDepths*100; depthMultiplied-- {
		if math.Mod(depthMultiplied/10, decimalDecrease*10) != 0 {
			continue
		}
		if isEligible {
			continue
		}
		depth = depthMultiplied / 100

		// rewrite depth if impasse is active
		if trade.ParentID != 0 {
			depth = strategySettings.ImpasseDepth
		}
		initialBid = GetInitialBidByDepth(budget, depth, strategySettings.Multiplier, percentage)
		initialBidInQuote = initialBid

		// update initialBid on inverse
		if trade.Inverse {
			initialBidInQuote *= trade.PositionPrice
		}

		if initialBidInQuote > trade.StrategyPair.TradeFilters.MinNotional {
			isEligible = true
		}
	}

	if initialBidInQuote < trade.StrategyPair.TradeFilters.MinNotional {
		msg := fmt.Sprintf("Insufficient funds (%f) to start trading for %s. Starting qty (%f) is lower than minimum required qty (%f) based on %f depths.", amount, trade.Symbol, initialBidInQuote, trade.StrategyPair.TradeFilters.MinNotional, depth)
		return initialBid, fmt.Errorf(msg)
	}

	return initialBid, nil
}

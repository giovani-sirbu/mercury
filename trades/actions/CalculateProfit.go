package actions

import (
	"github.com/giovani-sirbu/mercury/trades/aggragates"
	"strings"
)

func CalculateProfit(trade aggragates.Trades) float64 {
	var fee float64

	sellTotal, buyTotal := GetProfit(trade.History)
	profit := sellTotal - buyTotal                                      // Get profit from history
	feeInBase, feeInQuote := CalculateFees(trade.History, trade.Symbol) // Calculate fees
	fee = feeInQuote

	// Get profit and fees for inverse case
	if trade.Inverse {
		fee = feeInBase
		sellTotal, buyTotal = GetProfitInBase(trade.History)
		profit = buyTotal - sellTotal
		split := strings.Split(trade.Symbol, "/")
		trade.ProfitAsset = split[0]
	}

	return profit - fee
}

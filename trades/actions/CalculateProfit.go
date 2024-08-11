package actions

import (
	"github.com/giovani-sirbu/mercury/log"
	"github.com/giovani-sirbu/mercury/trades/aggragates"
	"strings"
)

func CalculateProfit(trade aggragates.Trades) float64 {
	log.Debug("----- CalculateProfit start -----")
	var fee float64

	log.Debug("trade history - ", trade.History)

	sellTotal, buyTotal := GetProfit(trade.History)
	profit := sellTotal - buyTotal                                      // Get profit from history
	feeInBase, feeInQuote := CalculateFees(trade.History, trade.Symbol) // Calculate fees
	fee = feeInQuote

	log.Debug("profit - ", profit, sellTotal, buyTotal, fee)

	// Get profit and fees for inverse case
	if trade.Inverse {
		fee = feeInBase
		sellTotal, buyTotal = GetProfitInBase(trade.History)
		profit = buyTotal - sellTotal
		split := strings.Split(trade.Symbol, "/")
		trade.ProfitAsset = split[0]

		log.Debug("profit inverse - ", profit, buyTotal, sellTotal, fee)
	}

	log.Debug("----- CalculateProfit end -----")

	return profit - fee
}

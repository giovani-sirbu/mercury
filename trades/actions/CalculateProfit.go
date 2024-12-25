package actions

import (
	"github.com/giovani-sirbu/mercury/log"
	"github.com/giovani-sirbu/mercury/trades/aggragates"
	"strings"
)

func CalculateProfit(trade aggragates.Trades) float64 {
	var fee float64
	var dust float64

	sellTotal, buyTotal := GetProfit(trade.History)
	profit := sellTotal - buyTotal                                      // Get profit from history
	feeInBase, feeInQuote := CalculateFees(trade.History, trade.Symbol) // Calculate fees
	fee = feeInQuote
	dust = trade.Dust * trade.PositionPrice
	log.Debug("Dust:", dust)

	// Get profit and fees for inverse case
	if trade.Inverse {
		fee = feeInBase
		sellTotal, buyTotal = GetProfitInBase(trade.History)
		profit = buyTotal - sellTotal
		split := strings.Split(trade.Symbol, "/")
		trade.ProfitAsset = split[0]
		dust = trade.Dust
		log.Debug("Inverse dust:", dust)
	}

	log.Debug("Total profit", profit+dust-fee)
	log.Debug("Profit info", profit, dust, fee)

	return profit + dust - fee
}

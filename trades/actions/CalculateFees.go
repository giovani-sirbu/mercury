package actions

import (
	"github.com/giovani-sirbu/mercury/trades/aggragates"
	"strings"
)

func CalculateFees(history []aggragates.TradesHistory, symbol string) (float64, float64) {
	var feeInBase = 0.0
	var feeInQuote = 0.0

	for _, historyData := range history {
		baseSymbol := strings.Split(symbol, "/")[0]
		quoteSymbol := strings.Split(symbol, "/")[1]

		if len(historyData.Fees) > 0 {
			for _, feeDetail := range historyData.Fees {
				if baseSymbol == feeDetail.Asset {
					feeInBase += feeDetail.Fee
				}
				if quoteSymbol == feeDetail.Asset {
					feeInQuote += feeDetail.Fee
				}
			}
		}
	}

	return feeInBase, feeInQuote
}

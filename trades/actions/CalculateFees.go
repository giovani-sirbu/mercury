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

		if len(historyData.Fees) > 0 {
			for _, feeDetail := range historyData.Fees {
				feeInQuote = feeInQuote + feeDetail.FeeInQuote
				if baseSymbol == feeDetail.Asset {
					feeInBase = feeInBase + feeDetail.Fee
				}
			}
		}
	}

	return feeInBase, feeInQuote
}

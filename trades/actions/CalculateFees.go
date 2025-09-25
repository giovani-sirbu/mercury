package actions

import (
	"github.com/giovani-sirbu/mercury/events"
	"strings"
)

// CalculateFeesOld processes trading history and calculates fees in base and quote assets.
func CalculateFeesOld(event events.Events) (float64, float64) {
	var feeInBase = 0.0
	var feeInQuote = 0.0

	for _, historyData := range event.Trade.History {
		baseSymbol := strings.Split(event.Trade.Symbol, "/")[0]
		quoteSymbol := strings.Split(event.Trade.Symbol, "/")[1]

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

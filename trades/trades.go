package trades

import (
	"github.com/giovani-sirbu/mercury/trades/aggragates"
	"strings"
)

func GetQuantities(history []aggragates.History) (float64, float64) {
	var buyTotal float64
	var sellTotal float64

	for _, historyData := range history {
		if strings.ToLower(historyData.Type) == "buy" {
			buyTotal += historyData.Quantity
		} else {
			sellTotal += historyData.Quantity
		}
	}

	return buyTotal, sellTotal
}

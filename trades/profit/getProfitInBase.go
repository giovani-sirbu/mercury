package profit

import (
	"strings"

	"github.com/giovani-sirbu/mercury/trades/aggragates"
)

// GetProfitInBase sums the trade's fills by side in the base asset and
// returns (sellTotal, buyTotal).
func GetProfitInBase(history []aggragates.TradesHistory) (float64, float64) {
	var buyTotal float64
	var sellTotal float64
	for _, historyData := range history {
		if strings.ToLower(historyData.Type) == "buy" {
			buyTotal += historyData.Quantity
		} else {
			sellTotal += historyData.Quantity
		}
	}
	return sellTotal, buyTotal
}

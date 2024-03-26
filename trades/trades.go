package trades

import (
	"github.com/giovani-sirbu/mercury/trades/actions"
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

// GetDefaultActions get default trades events functions
func GetDefaultActions() map[string]func([]byte) {
	var newActions = make(map[string]func([]byte))
	newActions["updateTrade"] = actions.UpdateTrade
	newActions["cancelPendingOrder"] = actions.CancelPendingOrder
	newActions["hasFunds"] = actions.HasFunds
	newActions["buy"] = actions.Buy
	newActions["sell"] = actions.Sell
	newActions["duplicateTrade"] = actions.DuplicateTrade
	return newActions
}

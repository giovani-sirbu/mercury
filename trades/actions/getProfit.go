package actions

import (
	"github.com/giovani-sirbu/mercury/trades/aggragates"
	"strings"
)

func GetProfit(history []aggragates.History) (float64, float64) {
	var buyTotal float64
	var sellTotal float64
	for _, historyData := range history {
		if strings.ToLower(historyData.Type) == "buy" {
			buyPerHistory := historyData.Price * historyData.Quantity
			buyTotal += buyPerHistory
		} else {
			sellPerHistory := historyData.Price * historyData.Quantity
			sellTotal += sellPerHistory
		}
	}
	return sellTotal, buyTotal
}

func GetProfitInBase(history []aggragates.History) (float64, float64) {
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

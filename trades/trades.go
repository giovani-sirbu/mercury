package trades

import (
	"github.com/giovani-sirbu/mercury/trades/aggragates"
	"strings"
)

func GetQuantities(history []aggragates.TradesHistory) (float64, float64) {
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

func GetQuantityByHistory(history []aggragates.TradesHistory, inverse bool) float64 {
	if len(history) == 0 {
		return 0
	}

	var buyQty float64
	var sellQty float64

	for _, historyData := range history {
		if strings.ToLower(historyData.Type) == "sell" {
			sellQty = sellQty + historyData.Quantity
		} else {
			buyQty = buyQty + historyData.Quantity
		}
	}

	if !inverse && strings.ToLower(history[len(history)-1].Type) == "sell" {
		return buyQty - sellQty
	}

	if inverse && strings.ToLower(history[len(history)-1].Type) == "buy" {
		return sellQty - buyQty
	}

	return history[len(history)-1].Quantity
}

func GetQuantityInQuote(history []aggragates.TradesHistory) float64 {
	var quantity float64
	for _, historyData := range history {
		quantity = quantity + historyData.Quantity*historyData.Price
	}
	return quantity
}

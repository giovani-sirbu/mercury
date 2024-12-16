package trades

import (
	"github.com/giovani-sirbu/mercury/trades/aggragates"
	"math"
	"sort"
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

func GetLatestQuantityByHistory(history []aggragates.TradesHistory, historyType string) float64 {
	if len(history) == 0 {
		return 0
	}

	// sort older history first
	sort.SliceStable(history, func(i, j int) bool {
		if history[i].Type != historyType {
			return true
		}
		return history[i].Quantity < history[j].Quantity
	})

	return history[len(history)-1].Quantity
}

func GetQuantityInQuote(history []aggragates.TradesHistory, typeFilter string) float64 {
	var quantity float64
	for _, historyData := range history {
		if historyData.Type != typeFilter {
			quantity = quantity + historyData.Quantity*historyData.Price
		}
	}
	return quantity
}

func GetInitialBid(amount float64, minDepth float64, multiplier float64, percentage float64) float64 {
	if multiplier <= 0 {
		return 0
	}
	if percentage < 0 || percentage >= 100 {
		return 0
	}

	// Calculate the adjusted ratio
	reductionFactor := 1 - (percentage / 100)
	ratio := multiplier * reductionFactor

	// Compute the first term (initial bid)
	numerator := amount * (1 - ratio)
	denominator := 1 - math.Pow(ratio, minDepth)
	initialBid := numerator / denominator

	/*
		//Generate the sequence
		sequence := make([]float64, int(minDepth))
		sequence[0] = initialBid

		// Calculate each depth's value
		for i := 1; i < int(minDepth); i++ {
			sequence[i] = sequence[i-1] * ratio
		}
	*/

	return initialBid
}

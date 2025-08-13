package trades

import (
	"fmt"
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

func GetInitialBidByDepth(amount float64, depth float64, multiplier float64, percentage float64) float64 {
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
	denominator := 1 - math.Pow(ratio, depth)
	initialBid := numerator / denominator

	/*
		//Generate the sequence
		sequence := make([]float64, int(depth))
		sequence[0] = initialBid

		// Calculate each depth's value
		for i := 1; i < int(depth); i++ {
			sequence[i] = sequence[i-1] * ratio
		}
	*/

	return initialBid
}

func CalculateMinimumQuantity(depth int, initial, percent float64) float64 {
	latestSum := initial * (1 + (percent / 100))
	var neededSum float64 = 0

	for i := 1; i < depth; i++ {
		latestSum = (latestSum - (latestSum * (percent / 100))) * 2
		neededSum += latestSum
	}

	// increase amount by 5%
	neededSum *= 1.05

	return neededSum
}

func CalculateInitialBid(amount float64, trade aggragates.Trades, strategyIndex int) (float64, error) {
	var initialBid float64
	var initialBidInQuote float64
	var depth float64
	isEligible := false
	strategySettings := trade.StrategyPair.StrategySettings[strategyIndex]

	maxDepths := strategySettings.Depths
	minDepths := strategySettings.MinDepths
	decimalDecrease := 0.5

	for depthMultiplied := maxDepths * 100; depthMultiplied >= minDepths*100; depthMultiplied-- {
		if math.Mod(depthMultiplied/10, decimalDecrease*10) != 0 {
			continue
		}
		if isEligible {
			continue
		}
		depth = depthMultiplied / 100

		// rewrite depth if impasse is active
		if trade.ParentID != 0 {
			depth = strategySettings.ImpasseDepth
		}
		initialBid = GetInitialBidByDepth(amount, depth, strategySettings.Multiplier, strategySettings.Percentage)
		initialBidInQuote = initialBid

		// update initialBid on inverse
		if trade.Inverse {
			initialBidInQuote *= trade.PositionPrice
		}

		if initialBidInQuote > trade.StrategyPair.TradeFilters.MinNotional {
			isEligible = true
		}
	}

	if initialBidInQuote < trade.StrategyPair.TradeFilters.MinNotional {
		msg := fmt.Sprintf("Insufficient funds (%f) to start trading for %s. Starting qty (%f) is lower than minimum required qty (%f) based on %f depths.", amount, trade.Symbol, initialBidInQuote, trade.StrategyPair.TradeFilters.MinNotional, depth)
		return initialBid, fmt.Errorf(msg)
	}

	return initialBid, nil
}

// GetLatestTradePrice retrieves the price of the most recent trade matching the specified trade type.
// It returns the price and a nil error if found, or an error if no matching trade exists or the input is invalid.
// The input history slice is not modified.
func GetLatestTradePrice(history []aggragates.TradesHistory, tradeType string) float64 {
	if len(history) == 0 {
		return 0
	}

	if tradeType == "" {
		tradeType = "BUY"
	}

	var latestTrade *aggragates.TradesHistory
	for i := range history {
		if history[i].Type == tradeType {
			if latestTrade == nil || history[i].CreatedAt.After(latestTrade.CreatedAt) {
				latestTrade = &history[i]
			}
		}
	}

	if latestTrade == nil {
		return 0
	}

	return latestTrade.Price
}

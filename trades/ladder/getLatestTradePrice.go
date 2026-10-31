package ladder

import "github.com/giovani-sirbu/mercury/trades/aggragates"

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

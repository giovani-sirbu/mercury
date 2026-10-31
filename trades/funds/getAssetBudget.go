package funds

import (
	"strconv"

	"github.com/giovani-sirbu/mercury/exchange/aggregates"
)

// GetAssetBudget returns the free balance the wallet holds in assetSymbol,
// 0 when the asset is missing or its balance does not parse.
func GetAssetBudget(assets []aggregates.UserAssetRecord, assetSymbol string) float64 {
	var remainedQuantity float64 // Init needed quantity

	// Check if account has remaining balance for pair
	for _, balance := range assets {
		if balance.Asset == assetSymbol {
			floatQuantity, _ := strconv.ParseFloat(balance.Free, 64)
			remainedQuantity = floatQuantity
		}
	}

	return remainedQuantity
}

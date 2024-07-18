package actions

import (
	"fmt"
	"github.com/giovani-sirbu/mercury/exchange/aggregates"
	"strings"
)

const fiatConversionSymbol = "USDT"

func CalculateUSDProfit(client aggregates.Actions, profit float64, profitAsset string) float64 {
	if strings.Contains(profitAsset, "USD") {
		return profit
	}

	// calculate usd profit
	price, err := client.GetPrice(profitAsset + "/" + fiatConversionSymbol)

	if err != nil {
		fmt.Println("could not calculate price")
		return 0
	}

	return profit * price
}

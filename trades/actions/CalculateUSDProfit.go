package actions

import (
	"fmt"
	"github.com/giovani-sirbu/mercury/exchange/aggregates"
	"strings"
)

const fiatConversionSymbol = "USDT"

func CalculateUSDProfit(client aggregates.Actions, profit float64, symbol string) float64 {
	split := strings.Split(symbol, "/")
	quoteAsset := split[1]

	if strings.Contains(quoteAsset, "USD") {
		return profit
	}

	// calculate usd profit
	usdSymbol := strings.Replace(symbol, quoteAsset, fiatConversionSymbol, 1)
	price, err := client.GetPrice(usdSymbol)

	if err != nil {
		fmt.Println("could not calculate price")
		return 0
	}

	return profit * price
}

package profit

import (
	"strings"

	"github.com/giovani-sirbu/mercury/events"
	"github.com/giovani-sirbu/mercury/log"
)

// fiatConversionSymbol is the quote asset non-USD profit is converted into
// (getSymbolPrice in fees converts into USDC: an inconsistency to resolve apart).
const fiatConversionSymbol = "USDT"

// CalculateUSDProfit converts trade.Profit into USD: verbatim for a USD-pegged
// profit asset, else through the event's wallet prices, then the exchange's
// GetPrice. Returns 0 on a lookup error, logged; a closed trade with zero
// USDProfit stays valid downstream.
func CalculateUSDProfit(event events.Events) float64 {
	trade := event.Trade

	if strings.Contains(trade.ProfitAsset, "USD") {
		return trade.Profit
	}

	symbol := trade.ProfitAsset + "/" + fiatConversionSymbol

	if p, ok := event.WsPrices[symbol]; ok && p > 0 {
		return trade.Profit * p
	}

	client, clientErr := event.Exchange.Client()
	if clientErr != nil {
		log.Error(clientErr.Error(), "Client", "CalculateUSDProfit")
		return 0
	}
	price, priceErr := client.GetPrice(symbol)
	if priceErr != nil {
		log.Error(priceErr.Error(), "GetPrice", "CalculateUSDProfit")
		return 0
	}
	return trade.Profit * price
}

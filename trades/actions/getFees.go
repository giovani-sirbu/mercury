package actions

import (
	"fmt"
	"github.com/giovani-sirbu/mercury/events"
	"github.com/giovani-sirbu/mercury/log"
	"slices"
	"strings"
)

var wsPrices = make(map[string]float64)

// GetFees processes trading history and calculates fees in base and quote assets.
func GetFees(event events.Events) float64 {
	var fees float64
	var feesInBase, feesInQuote float64
	baseSymbol, quoteSymbol := splitSymbol(event.Trade.Symbol)

	for _, data := range event.Trade.History {
		if len(data.Fees) == 0 {
			continue
		}

		for _, fee := range data.Fees {
			if fee.Fee <= 0 {
				continue
			}

			switch fee.Asset {
			case baseSymbol:
				feesInBase += fee.Fee
				feesInQuote += fee.Fee * data.Price
				continue
			case quoteSymbol:
				feesInQuote += fee.Fee
				feesInBase += fee.Fee / data.Price
				continue
			default:
				// handle price for fees like BNB
				if !slices.Contains([]string{baseSymbol, quoteSymbol}, fee.Asset) {
					feeAssetPrice, _ := getSymbolPrice(event, fee.Asset)
					if feeAssetPrice > 0 {
						feesInQuote += fee.Fee * feeAssetPrice
					}

					profitAssetPrice, _ := getSymbolPrice(event, event.Trade.ProfitAsset)
					if profitAssetPrice > 0 {
						feesInBase += feesInQuote / profitAssetPrice
					}
				}
			}
		}
	}

	fees = feesInQuote

	if event.Trade.Inverse {
		fees = feesInBase
	}

	log.Debug(fmt.Sprintf("GetFees: fees(%f), feesInBase(%f), feesInQuote(%f), inverse(%t)", fees, feesInBase, feesInQuote, event.Trade.Inverse))

	return fees
}

// getSymbolPrice return symbol real time price
func getSymbolPrice(event events.Events, asset string) (float64, error) {
	if slices.Contains([]string{"USDT", "USDC"}, asset) {
		return event.Trade.PositionPrice, nil
	}

	symbol := fmt.Sprintf("%s/USDC", asset)

	// get ws prices from cache
	event.Storage.Get("ws-symbols-price", &wsPrices)

	// default price fetched from cache
	price := wsPrices[symbol]

	// fallback: fetch price from exchange if cache price no available
	if wsPrices[symbol] == 0 {
		client, clientErr := event.Exchange.Client()
		if clientErr != nil {
			return 0, clientErr
		}
		clientPrice, priceErr := client.GetPrice(symbol)

		fmt.Println("client.GetPrice for: ", symbol)

		if priceErr != nil {
			return 0, priceErr
		}

		price = clientPrice
	}

	// format price
	price = ToFixed(price, int(event.Trade.StrategyPair.TradeFilters.PriceFilter))

	// set price to cache
	if wsPrices[symbol] == 0 {
		wsPrices[symbol] = price
	}

	return price, nil
}

// splitSymbol splits a trading pair symbol into base and quote symbols.
func splitSymbol(symbol string) (string, string) {
	parts := strings.Split(symbol, "/")
	if len(parts) != 2 {
		return "", ""
	}
	return parts[0], parts[1]
}

package actions

import (
	"fmt"
	"slices"
	"strings"

	"github.com/giovani-sirbu/mercury/events"
	"github.com/giovani-sirbu/mercury/log"
)

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
				// handle price for fees paid in a third asset (e.g. BNB)
				if !slices.Contains([]string{baseSymbol, quoteSymbol}, fee.Asset) {
					feeAssetPrice, err := getSymbolPrice(event, fee.Asset)
					if err != nil {
						log.Error(err.Error(), "getSymbolPrice", "GetFees")
					}
					if feeAssetPrice > 0 {
						feesInQuote += fee.Fee * feeAssetPrice
					}

					profitAssetPrice, err := getSymbolPrice(event, event.Trade.ProfitAsset)
					if err != nil {
						log.Error(err.Error(), "getSymbolPrice", "GetFees")
					}
					if profitAssetPrice > 0 {
						feesInBase += fee.Fee * feeAssetPrice / profitAssetPrice
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

// getSymbolPrice returns the real-time price of `asset` quoted in USDC.
//
// Lookup order (cheapest first):
//  1. event.WsPrices — in-process snapshot. Hermes populates it from its own WS
//     map; agora populates it by calling hermes' GET /prices (TTL-cached in
//     agora/helpers/getWsPrices.go). Callers that do not populate WsPrices (e.g.
//     sisyphus backtesting on a virtual exchange) fall through to step 2.
//  2. exchange API — last-resort network call via event.Exchange.Client().
//
// A prior version held a package-level `var wsPrices` map to memoise results across calls.
// That variable was shared by every concurrent goroutine calling GetFees and was never
// protected by a lock, producing a real race condition on a financial code path. It has
// been removed; the stateful cache now lives where it belongs — on the event.
//
// A second prior version read from Dragonfly at key "ws-symbols-price" as an
// intermediate fallback. That key has no producer since hermes stopped publishing
// the snapshot to the shared cache (/prices HTTP handoff replaces it), so the
// branch was always a miss and has been removed.
func getSymbolPrice(event events.Events, asset string) (float64, error) {
	if slices.Contains([]string{"USDT", "USDC"}, asset) {
		return event.Trade.PositionPrice, nil
	}

	symbol := fmt.Sprintf("%s/USDC", asset)
	precision := int(event.Trade.StrategyPair.TradeFilters.PriceFilter)

	// 1. In-process snapshot.
	if p, ok := event.WsPrices[symbol]; ok && p > 0 {
		return ToFixed(p, precision), nil
	}

	// 2. Exchange API fallback.
	client, err := event.Exchange.Client()
	if err != nil {
		return 0, err
	}
	price, err := client.GetPrice(symbol)
	if err != nil {
		return 0, err
	}
	return ToFixed(price, precision), nil
}

// splitSymbol splits a trading pair symbol into base and quote symbols.
func splitSymbol(symbol string) (string, string) {
	parts := strings.Split(symbol, "/")
	if len(parts) != 2 {
		return "", ""
	}
	return parts[0], parts[1]
}

package fees

import (
	"slices"

	"github.com/giovani-sirbu/mercury/events"
	"github.com/giovani-sirbu/mercury/helpers"
	"github.com/giovani-sirbu/mercury/log"
)

// GetFeesBaseQuote processes trading history and returns the aggregated fees
// expressed in both the trade's base and quote denominations.
//
// Fees paid in the base asset are added to feeInBase directly and converted into
// quote via the fill price. Fees paid in the quote asset are added to feeInQuote
// directly and converted into base via the fill price. Fees paid in a third
// asset (e.g. BNB) are priced via getSymbolPrice and contributed to both totals.
//
// This is the source of truth for fee aggregation. GetFees is a thin wrapper
// that selects one of the two values based on event.Trade.Inverse.
func GetFeesBaseQuote(event events.Events) (feeInBase, feeInQuote float64) {
	baseSymbol, quoteSymbol := helpers.SplitSymbol(event.Trade.Symbol)

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
				feeInBase += fee.Fee
				feeInQuote += fee.Fee * data.Price
				continue
			case quoteSymbol:
				feeInQuote += fee.Fee
				feeInBase += fee.Fee / data.Price
				continue
			default:
				// handle price for fees paid in a third asset (e.g. BNB)
				if !slices.Contains([]string{baseSymbol, quoteSymbol}, fee.Asset) {
					feeAssetPrice, err := getSymbolPrice(event, fee.Asset)
					if err != nil {
						log.Error(err.Error(), "getSymbolPrice", "GetFeesBaseQuote")
					}
					if feeAssetPrice > 0 {
						feeInQuote += fee.Fee * feeAssetPrice
					}

					profitAssetPrice, err := getSymbolPrice(event, event.Trade.ProfitAsset)
					if err != nil {
						log.Error(err.Error(), "getSymbolPrice", "GetFeesBaseQuote")
					}
					if profitAssetPrice > 0 {
						feeInBase += fee.Fee * feeAssetPrice / profitAssetPrice
					}
				}
			}
		}
	}

	return feeInBase, feeInQuote
}

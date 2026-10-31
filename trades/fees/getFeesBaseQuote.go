package fees

import (
	"slices"

	"github.com/giovani-sirbu/mercury/events"
	"github.com/giovani-sirbu/mercury/helpers"
	"github.com/giovani-sirbu/mercury/log"
)

// GetFeesBaseQuote returns the history's fees in both the base and the quote
// denomination: a base fee is converted into quote at its fill price and a
// quote fee into base, while a third-asset fee (the BNB discount) is priced
// through getSymbolPrice into both. GetFees picks one of the two by Inverse.
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
				// A fee paid in a third asset.
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

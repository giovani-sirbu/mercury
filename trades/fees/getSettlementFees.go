package fees

import (
	"fmt"
	"github.com/giovani-sirbu/mercury/helpers"
	"slices"

	"github.com/giovani-sirbu/mercury/events"
	"github.com/giovani-sirbu/mercury/log"
)

// GetSettlementFees returns the fees still to charge against the trade's
// realized profit, in the profit denomination (quote for spot, base for
// inverse). The exchange takes each leg's commission from the asset received,
// so the opening leg's fee is already embodied in the fills the closing leg
// could afford and GetProfit's gross figure has paid it once; charging it
// again double-counts it. What still reduces profit: the fees taken in the
// profit asset itself, which no later fill embodies, and the fees paid from a
// separate wallet (the BNB discount), priced as GetFeesBaseQuote prices them.
func GetSettlementFees(event events.Events) float64 {
	baseSymbol, quoteSymbol := helpers.SplitSymbol(event.Trade.Symbol)

	var fees float64
	for _, data := range event.Trade.History {
		for _, fee := range data.Fees {
			if fee.Fee <= 0 {
				continue
			}

			switch fee.Asset {
			case baseSymbol:
				if event.Trade.Inverse {
					fees += fee.Fee
				}
			case quoteSymbol:
				if !event.Trade.Inverse {
					fees += fee.Fee
				}
			default:
				if slices.Contains([]string{baseSymbol, quoteSymbol}, fee.Asset) {
					continue
				}
				feeAssetPrice, err := getSymbolPrice(event, fee.Asset)
				if err != nil {
					log.Error(err.Error(), "getSymbolPrice", "GetSettlementFees")
				}
				if feeAssetPrice <= 0 {
					continue
				}
				if !event.Trade.Inverse {
					fees += fee.Fee * feeAssetPrice
					continue
				}
				profitAssetPrice, err := getSymbolPrice(event, event.Trade.ProfitAsset)
				if err != nil {
					log.Error(err.Error(), "getSymbolPrice", "GetSettlementFees")
				}
				if profitAssetPrice > 0 {
					fees += fee.Fee * feeAssetPrice / profitAssetPrice
				}
			}
		}
	}

	log.Debug(fmt.Sprintf("GetSettlementFees: fees(%f), inverse(%t)", fees, event.Trade.Inverse))

	return fees
}

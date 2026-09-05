package fees

import (
	"fmt"
	"github.com/giovani-sirbu/mercury/helpers"
	"slices"

	"github.com/giovani-sirbu/mercury/events"
	"github.com/giovani-sirbu/mercury/log"
)

// GetSettlementFees returns the fees that still have to be charged against the
// trade's realized profit, in the profit denomination: quote for spot, base
// for inverse.
//
// The exchange takes each leg's commission from the asset the account
// RECEIVES, so the opening side's commission is already embodied in the fills
// the closing side could afford: a spot buy's base-asset fee leaves less base
// to sell, an inverse sell's quote-asset fee leaves less quote to buy back
// with. GetProfit's gross figure has therefore already paid the opening leg
// once, and subtracting its cross-converted value again (as profit math did
// via GetFees) charged every round trip roughly 0.3% instead of the real
// 0.2% — on backtest 76 that alone turned the marginal wins 12840 and 12885
// into booked losses.
//
// What still reduces profit explicitly:
//   - fees taken in the profit asset itself (spot: quote fees, which come off
//     the sell proceeds; inverse: base fees, which come off the buy-back) —
//     no later fill embodies them;
//   - fees paid from a separate wallet (the BNB discount): they never touch
//     the trade's own assets, so they are priced into the profit denomination
//     the same way GetFeesBaseQuote prices them.
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

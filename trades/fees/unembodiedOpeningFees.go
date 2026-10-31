package fees

import (
	"fmt"
	"slices"

	"github.com/giovani-sirbu/mercury/events"
	"github.com/giovani-sirbu/mercury/helpers"
	"github.com/giovani-sirbu/mercury/log"
)

// UnembodiedOpeningFees returns the commissions already on the trade's history
// that the simulated close quantity could not absorb, in the profit
// denomination (quote for spot, base for inverse). The profit gates simulate
// a close with quantities.SimulatedCloseQuantity and charge one closing leg
// (GetFees); that is a faithful round trip only for the fees the exchange took
// out of the asset received, which the smaller quantity already pays. A fee
// paid from a separate wallet (the BNB discount) and, on spot, a quote fee
// taken off a past sell are embodied nowhere, so the gate charges them here.
// The embodiment rule mirrors SimulatedCloseQuantity rather than calling it,
// since quantities imports this package; GetSettlementFees answers the same
// question for GetProfit's gross figure and counts the base fees differently.
func UnembodiedOpeningFees(event events.Events) float64 {
	baseSymbol, quoteSymbol := helpers.SplitSymbol(event.Trade.Symbol)

	var fees float64
	for _, data := range event.Trade.History {
		for _, fee := range data.Fees {
			if fee.Fee <= 0 {
				continue
			}

			switch fee.Asset {
			case baseSymbol:
				// Embodied on both sides.
				continue
			case quoteSymbol:
				if !event.Trade.Inverse {
					fees += fee.Fee
				}
				// Inverse embodies it before the buy-back is priced.
			default:
				if slices.Contains([]string{baseSymbol, quoteSymbol}, fee.Asset) {
					continue
				}
				fees += thirdAssetFeeInProfitAsset(event, fee.Asset, fee.Fee)
			}
		}
	}

	log.Debug(fmt.Sprintf("UnembodiedOpeningFees: fees(%f), inverse(%t)", fees, event.Trade.Inverse))

	return fees
}

// thirdAssetFeeInProfitAsset prices a fee paid in neither base nor quote into
// the profit denomination, as GetFeesBaseQuote does; an unpriceable fee contributes nothing.
func thirdAssetFeeInProfitAsset(event events.Events, asset string, fee float64) float64 {
	feeAssetPrice, err := getSymbolPrice(event, asset)
	if err != nil {
		log.Error(err.Error(), "getSymbolPrice", "UnembodiedOpeningFees")
	}
	if feeAssetPrice <= 0 {
		return 0
	}
	if !event.Trade.Inverse {
		return fee * feeAssetPrice
	}

	profitAssetPrice, err := getSymbolPrice(event, event.Trade.ProfitAsset)
	if err != nil {
		log.Error(err.Error(), "getSymbolPrice", "UnembodiedOpeningFees")
	}
	if profitAssetPrice <= 0 {
		return 0
	}

	return fee * feeAssetPrice / profitAssetPrice
}

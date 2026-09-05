package fees

import (
	"fmt"
	"slices"

	"github.com/giovani-sirbu/mercury/events"
	"github.com/giovani-sirbu/mercury/helpers"
	"github.com/giovani-sirbu/mercury/log"
)

// UnembodiedOpeningFees returns the commissions already on the trade's history
// that the SIMULATED CLOSE QUANTITY could not absorb, in the profit
// denomination: quote for spot, base for inverse.
//
// The profit gates simulate a close with quantities.SimulatedCloseQuantity and
// then charge one closing leg (GetFees). That is only a faithful round trip
// for commissions the exchange took out of the asset the account received,
// because those leave a smaller quantity behind and are paid by the smaller
// fill. SimulatedCloseQuantity subtracts exactly:
//
//	spot     the base-asset fees (a buy's commission comes out of the base)
//	inverse  the quote-asset fees AND the base-asset fees
//
// Everything else was paid from somewhere the quantity never sees, so nothing
// pays it unless the gate does:
//
//   - a fee paid from a separate wallet (the BNB discount, or any third asset)
//     never touches the trade's own balances at all. This is the case the
//     one-leg rule got wrong: with the discount on, NOTHING is embodied, and a
//     round trip that owes two legs was charged one. On a clean all-BNB ladder
//     the gate read 0.165 USDC better than the truth where the old `* 2` rule
//     read 0.025 better — six times the error, and always in the optimistic
//     direction, so trades closed under their real break even.
//   - on a spot trade, a quote-asset fee: it comes off the proceeds of a sell
//     that has already happened, and no later fill carries it.
//
// It is deliberately NOT GetSettlementFees, which answers the same question
// for GetProfit's GROSS figure and therefore counts a spot base fee and an
// inverse base fee differently from this one. The embodiment rule below has
// to mirror quantities.SimulatedCloseQuantity, and mirrors it rather than
// calling it because quantities imports this package.
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
				// Embodied on both sides: spot subtracts it from the base it
				// has left to sell, inverse from the base it buys back.
				continue
			case quoteSymbol:
				if !event.Trade.Inverse {
					fees += fee.Fee
				}
				// Inverse embodies it: the quote fee is taken off before the
				// buy-back is priced.
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

// thirdAssetFeeInProfitAsset prices a fee paid in neither the base nor the
// quote asset into the trade's profit denomination, the same way
// GetFeesBaseQuote and GetSettlementFees price it. An unpriceable fee
// contributes nothing rather than a guess.
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

package fees

import (
	"github.com/giovani-sirbu/mercury/events"
	"github.com/giovani-sirbu/mercury/helpers"
)

// CalculateFees returns the fees paid literally in the trade's base and quote
// assets, with no cross-conversion: a third-asset fee (the BNB discount) is
// excluded from both. It is the primitive of the execution math (Sell,
// HasFunds), which derives the remaining wallet balance from the history;
// converting a fee the wallet never paid in base or quote would over-deduct
// and leave artificial dust. Profit accounting uses GetFeesBaseQuote instead.
func CalculateFees(event events.Events) (feeInBase, feeInQuote float64) {
	baseSymbol, quoteSymbol := helpers.SplitSymbol(event.Trade.Symbol)

	for _, data := range event.Trade.History {
		for _, fee := range data.Fees {
			if fee.Fee <= 0 {
				continue
			}
			switch fee.Asset {
			case baseSymbol:
				feeInBase += fee.Fee
			case quoteSymbol:
				feeInQuote += fee.Fee
			}
		}
	}

	return feeInBase, feeInQuote
}

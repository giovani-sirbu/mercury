package actions

import (
	"github.com/giovani-sirbu/mercury/events"
)

// CalculateFees returns the fees paid literally in the trade's base and
// quote assets. No cross-conversion is performed: a USDC fee on a BTC/USDC
// trade contributes only to feeInQuote (never to feeInBase), and a third-asset
// fee (BNB, etc.) is intentionally excluded from both buckets.
//
// This is the correct primitive for trade-execution math (Sell, HasFunds) which
// computes "remaining wallet balance" from history. Binance's default fee
// behavior takes BUY-side fees from the base asset received and SELL-side fees
// from the quote asset received; with the BNB discount, fees come from the
// user's separate BNB wallet and never touch base or quote balances. Cross-
// converting non-base/non-quote fees into a base-denominated equivalent would
// over-deduct from the wallet and leave artificial dust.
//
// For profit accounting that legitimately needs the total cost of all fees
// (including BNB) expressed in a single denomination, use GetFeesBaseQuote or
// the GetFees wrapper.
func CalculateFees(event events.Events) (feeInBase, feeInQuote float64) {
	baseSymbol, quoteSymbol := splitSymbol(event.Trade.Symbol)

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

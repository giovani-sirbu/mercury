package helpers

import "github.com/giovani-sirbu/mercury/trades/aggragates"

// FindUsedAmount returns the amount already committed in the given quote
// currency, 0 when that currency has no entry. Non-inverse trades subtract it
// from their budget so both sides of the same wallet cannot spend the same
// quote twice.
func FindUsedAmount(usedAmounts []aggragates.UsedAmountResult, asset string) float64 {
	for _, usedAmount := range usedAmounts {
		if usedAmount.QuoteCurrency == asset {
			return usedAmount.UsedAmount
		}
	}
	return 0
}

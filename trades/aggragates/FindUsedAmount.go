package aggragates

func FindUsedAmount(usedAmounts []UsedAmountResult, asset string) float64 {
	for _, usedAmount := range usedAmounts {
		if usedAmount.QuoteCurrency == asset {
			return usedAmount.UsedAmount
		}
	}
	return 0
}

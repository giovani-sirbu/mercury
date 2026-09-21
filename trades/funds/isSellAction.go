package funds

// IsSellAction reports whether the trade's next order closes the position
// instead of adding to it. The two sides spend opposite assets — a close
// spends what the position holds, an entry spends the budget — so the funds
// check reads a different wallet asset for each, and every caller that
// branches on the side has to split it the same way.
func IsSellAction(positionType string) bool {
	switch positionType {
	case "sell", "takeProfit", "sellParent":
		return true
	}

	return false
}

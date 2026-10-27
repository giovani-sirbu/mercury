package funds

// IsSellAction reports whether the trade's next order closes the position
// instead of adding to it; the two sides spend opposite assets.
func IsSellAction(positionType string) bool {
	switch positionType {
	case "sell", "takeProfit", "sellParent":
		return true
	}

	return false
}

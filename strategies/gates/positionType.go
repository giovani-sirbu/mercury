package gates

// PositionType maps a force-trailing re-anchor onto the position it re-arms (stopLoss or
// takeProfit) so every gate keys on it; the state machine and tradelog.SaveHoldLog keep the raw name.
func PositionType(positionType string) string {
	switch positionType {
	case "forceTrailingStopLoss":
		return "stopLoss"
	case "forceTrailingTakeProfit":
		return "takeProfit"
	}
	return positionType
}

package gates

// PositionType maps a force-trailing re-anchor onto the rung family it
// re-arms so every gate has power on it: those states only move
// PositionPrice and hand the rung back to stopLoss / takeProfit, and a gate
// keyed on the raw name saw nothing to hold. Only the gates use it; the state
// machine, SaveHoldLog (the message keeps the raw name) and the capitulation
// episode stay on the raw position.
func PositionType(positionType string) string {
	switch positionType {
	case "forceTrailingStopLoss":
		return "stopLoss"
	case "forceTrailingTakeProfit":
		return "takeProfit"
	}
	return positionType
}

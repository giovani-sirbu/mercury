package smarttakeloss

import "github.com/giovani-sirbu/mercury/strategies/gates"

// protectedPosition is whether the ladder decided a close (hasProfit's, impasse's or sellLoss) the overlay passes through.
func protectedPosition(position string) bool {
	switch position {
	case "sell", "takeProfit", "update_takeProfit", "sellParent", "impasse", "sellLoss":
		return true
	}

	return false
}

// yieldsToActivation is whether a take profit proposed in buy runs the triggers, only the activation replacing it.
func yieldsToActivation(position, state string) bool {
	return gates.PositionType(position) == "takeProfit" && gates.PositionType(state) == "buy"
}

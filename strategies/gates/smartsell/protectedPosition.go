package smartsell

import (
	"github.com/giovani-sirbu/mercury/strategies/gates"
	"github.com/giovani-sirbu/mercury/trades/aggragates"
)

// protectedPosition is whether the ladder decided a close (hasProfit's, impasse's or takeLoss) the
// overlay passes through, a takeProfit an activation committed aside (committedIn).
func protectedPosition(position string) bool {
	switch position {
	case "sell", "takeProfit", "update_takeProfit", "sellParent", "impasse", "takeLoss":
		return true
	}

	return false
}

// yieldsToActivation is whether a take profit proposed in buy runs the triggers, only the activation replacing it.
func yieldsToActivation(position, state string) bool {
	return gates.PositionType(position) == aggragates.PositionTakeProfit && gates.PositionType(state) == aggragates.PositionBuy
}

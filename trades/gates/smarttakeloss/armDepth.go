package smarttakeloss

import (
	"math"

	"github.com/giovani-sirbu/mercury/trades/aggragates"
	"github.com/giovani-sirbu/mercury/trades/ladder"
)

// armed is the depth arming rule: filled >= max(2, Floor(Depths) − offset),
// Depths read from the row the next fill would use. A fund block does not
// arm by itself any more: hermes ticks no blocked trade and backtesting
// skips them before the overlay, so the rule reads only what the fills say.
func armed(trade aggragates.Trades) bool {
	settings := trade.StrategyPair.StrategySettings
	if len(settings) == 0 {
		return false
	}
	filled := ladder.CountFilledEntries(trade)
	row := ladder.SettingsIndexOrBase(settings, filled)
	maxDepths := int(math.Floor(settings[row].Depths))
	return maxDepths >= 2 && filled >= ArmDepth(maxDepths)
}

// ArmDepth is the filled-entry count from which a ladder sized for maxDepths
// arms the smart take loss: Depths − ArmDepthOffset, floored at minArmDepth
// and kept one rung short of the last configured depth.
func ArmDepth(maxDepths int) int {
	armDepth := maxDepths - ArmDepthOffset
	if armDepth < minArmDepth {
		armDepth = minArmDepth
	}
	// Never arm at or past the row's Depths: the arm depth is where the exit
	// lens starts watching the ladder, and a ladder already at its last
	// configured rung has no add left to protect — keep one rung of margin.
	if maxDepths > 0 && armDepth >= maxDepths {
		armDepth = maxDepths - 1
	}
	if armDepth < minArmDepth {
		armDepth = minArmDepth
	}
	return armDepth
}

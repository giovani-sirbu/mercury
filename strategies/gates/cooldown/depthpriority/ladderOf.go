package depthpriority

import (
	"github.com/giovani-sirbu/mercury/trades/aggragates"
	"github.com/giovani-sirbu/mercury/trades/ladder"
)

// LadderOf is the wallet view of one trade as this gate reads it, costed DepthOffset depths short
// of its ceiling (ladder.DepthOfOffset): the one builder every engine's view and the gate's own
// row go through, so a reserve means the same on every surface.
func LadderOf(trade aggragates.Trades) aggragates.LadderDepth {
	return ladder.DepthOfOffset(trade, depthOffset)
}

// reserveCeiling is the depth the reserve of a ladder is costed through: its ceiling less
// DepthOffset. A ladder at or past it is full for the gate.
func reserveCeiling(candidate aggragates.LadderDepth) int {
	return candidate.MaxDepth - depthOffset
}

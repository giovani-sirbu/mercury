package smarttakeloss

import (
	"github.com/giovani-sirbu/mercury/strategies/gates/smarttakeloss/freefallprevention"
	"github.com/giovani-sirbu/mercury/trades/aggragates"
)

// earlyTakeProfitFreeFallPreventionReads is the free fall prevention's reading for the latch
// (freefallprevention.Trigger: the last closed bar or the window's share) on a ladder holding
// freefallprevention.ArmDepth fills, whatever freefallprevention.Enabled says, and whether it holds.
// The side is long: earlyTakeProfitEligible admits no other.
func earlyTakeProfitFreeFallPreventionReads(st state, block aggragates.SmartTakeLossIndicators, found tick) (freefallprevention.Reading, bool) {
	if found.kind != rowNone || st.depthPriorityHeld || !earlyTakeProfitFreeFallPrevention || len(st.fills) < freefallprevention.ArmDepth {
		return freefallprevention.Reading{}, false
	}
	reading := freefallprevention.Trigger(block, aggragates.SideLong)
	return reading, reading.Holds
}

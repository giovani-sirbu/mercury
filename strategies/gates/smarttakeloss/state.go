package smarttakeloss

import (
	"slices"
	"time"
)

// lastFill is the newest entry fill in slice order, zero when none filled.
func (st state) lastFill() entryFill {
	if len(st.fills) == 0 {
		return entryFill{}
	}
	return st.fills[len(st.fills)-1]
}

// saleEpisodeOpenBesides is whether a sale gate other than gate holds an open episode (the slow
// decline gate pending, a flat market or free fall prevention episode): one at a time opens.
func (st state) saleEpisodeOpenBesides(gate string) bool {
	return (gate != GateSlowDecline && st.slowDeclinePending) ||
		(gate != GateFlatMarket && st.flatMarketActive) ||
		(gate != GateFreeFallPrevention && st.freeFallPreventionActive)
}

// pendsOn is whether a new fill reads feature: one the ladder is pending on, or any on a ladder
// pending on no named feature.
func (st state) pendsOn(feature string) bool {
	return len(st.pendingFeatures) == 0 || slices.Contains(st.pendingFeatures, feature)
}

// pendingOn is the state with the ladder pending from its newest fill on the findings' decline
// features, the sale's memory, reached stamp and gap stamp cleared.
func (st state) pendingOn(findings []Finding) state {
	st.slowDeclinePending = true
	st.slowDeclinePendingFrom = st.lastFill().Price
	st.saleHeld = false
	st.saleHeldUnder = heldTrigger{}
	st.saleReachedAt = time.Time{}
	st.slowDeclineActivated, st.slowDeclineGapHeldAt = false, time.Time{}
	st.pendingFeatures = nil
	for _, finding := range findings {
		if declineFeature(finding.Feature) {
			st.pendingFeatures = append(st.pendingFeatures, finding.Feature)
		}
	}
	return st
}

// notPending is the state with the ladder watched and not pending, the sale's memory, reached stamp and gap stamp cleared.
func (st state) notPending() state {
	st.slowDeclinePending = false
	st.slowDeclinePendingFrom = 0
	st.saleHeld = false
	st.saleHeldUnder = heldTrigger{}
	st.saleReachedAt = time.Time{}
	st.slowDeclineActivated, st.slowDeclineGapHeldAt = false, time.Time{}
	st.pendingFeatures = nil
	return st
}

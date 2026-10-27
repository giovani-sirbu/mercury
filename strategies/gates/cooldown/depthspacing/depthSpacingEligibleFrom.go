package depthspacing

import (
	"sort"
	"time"

	"github.com/giovani-sirbu/mercury/trades/aggragates"
)

// depthSpacingEligibleFrom folds the fills and the activations into the instant the next depth may
// arm: every depth is held from its fill for the current hold, and the step grows on a cascade
// (depthSpacingCascades), else back to the starting step (depthSpacingStartStep); no fills leaves
// the state zero and the gate open.
func depthSpacingEligibleFrom(spacing []aggragates.TradesStrategyEvents, fills []depthFill, now time.Time) depthSpacingState {
	if len(fills) == 0 {
		return depthSpacingState{}
	}

	current := len(fills)
	activations := depthSpacingActivations(spacing)
	if _, written := activations[current]; !written {
		activations[current] = now.UTC()
	}
	depths := make([]int, 0, len(activations))
	for depth := range activations {
		if depth <= current {
			depths = append(depths, depth)
		}
	}
	sort.Ints(depths)

	var state depthSpacingState
	var expiry time.Time
	for _, depth := range depths {
		filled := fills[depth-1].At
		if depthSpacingCascades(state.step, expiry, activations[depth], filled) {
			state.step++
		} else {
			state.step = depthSpacingStartStep(depth)
		}
		state.hold = depthSpacingHoldFor(state.step)
		expiry = filled.Add(state.hold)
	}
	state.eligibleFrom = expiry
	return state
}

// depthSpacingCascades is whether an activation continues the cascade: a step before it, its depth
// filled strictly before the previous hold's expiry (only the price release does that) and the
// activation less than Window past it.
func depthSpacingCascades(step int, expiry, activated, filled time.Time) bool {
	if step == 0 {
		return false
	}
	if activated.Sub(expiry) >= Window {
		return false
	}
	return filled.Before(expiry)
}

// depthSpacingStartStep is the step a cascade starts at: 1, or with startStepFromDepth the
// activation's depth less startStepOffset, never under 1.
func depthSpacingStartStep(depth int) int {
	if !startStepFromDepth {
		return 1
	}
	return max(1, depth-startStepOffset)
}

// depthSpacingHoldFor is BaseHold * factor^(step-1) clamped to maxHold, scaled step by step through
// float64 so a long cascade cannot overflow.
func depthSpacingHoldFor(step int) time.Duration {
	if step < 1 {
		return 0
	}
	hold := BaseHold
	for i := 1; i < step; i++ {
		if float64(hold) >= float64(maxHold)/factor {
			return maxHold
		}
		hold = time.Duration(float64(hold) * factor)
	}
	if hold > maxHold {
		return maxHold
	}
	return hold
}

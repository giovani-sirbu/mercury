package cooldown

import (
	"sort"
	"time"

	"github.com/giovani-sirbu/mercury/trades/aggragates"
)

// depthSpacingEligibleFrom folds a trade's ladder into the instant the next
// depth may arm.
//
// EVERY depth carries a hold from its own fill, whether or not the ladder is
// cascading: the rule is a minimum spacing and no two entries land closer than
// the current hold. What escalates is `step`, and it counts ACTIVATIONS of this
// gate, not fills: a depth the gate never held does not deepen the cascade, and
// a depth it held does, however long after the fill the hold was noticed. The
// activations are read from the trade's depth-spacing events
// (depthSpacingActivations), the only state the gate keeps.
//
// Step is 1 at the first activation and at the first one after a reset. The
// next activation adds one when it lands less than DepthSpacingWindow past the
// previous activation's hold expiry, measured from the fill of the depth that
// activated, not from the event. Measured from the previous fill the gap would
// be >= the hold by construction and the rule could never escalate. An
// activation DepthSpacingWindow or more past that expiry is a genuine pause:
// the count goes back to 1. It is never zero and never capped; only the hold
// is, by depthSpacingHoldFor.
//
// The tick being evaluated is itself the activation of the newest depth: its
// time is the stamp of the earliest event written for that depth, else `now`.
// Folding it in means the first held tick already reports the step its event
// will keep, so the message stays byte-stable while the hold stands.
//
// Unreadable history (no fills) leaves the state zero and the gate open.
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
		at := activations[depth]
		if state.step == 0 || at.Sub(expiry) >= DepthSpacingWindow {
			state.step = 1
		} else {
			state.step++
		}
		state.hold = depthSpacingHoldFor(state.step)
		expiry = fills[depth-1].At.Add(state.hold)
	}
	state.eligibleFrom = expiry
	return state
}

// depthSpacingHoldFor is base * factor^(step-1), clamped. It scales in a
// loop and returns at the ceiling rather than computing the power, so a long
// cascade cannot overflow the duration on its way to a value that would have
// been clamped anyway.
//
// The factor may be fractional, so each step goes through float64: a Duration
// is an integer count of nanoseconds and cannot be scaled by a fractional
// factor directly. The truncation that conversion costs is nanoseconds.
func depthSpacingHoldFor(step int) time.Duration {
	if step < 1 {
		return 0
	}
	hold := DepthSpacingBaseHold
	for i := 1; i < step; i++ {
		if float64(hold) >= float64(depthSpacingMaxHold)/depthSpacingFactor {
			return depthSpacingMaxHold
		}
		hold = time.Duration(float64(hold) * depthSpacingFactor)
	}
	if hold > depthSpacingMaxHold {
		return depthSpacingMaxHold
	}
	return hold
}

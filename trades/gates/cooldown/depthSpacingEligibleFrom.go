package cooldown

import "time"

// depthSpacingEligibleFrom folds a trade's ladder into the instant the next
// depth may arm.
//
// EVERY depth carries a hold, starting from the first fill — not only the ones
// that follow a fast pair. The earlier shape seeded the fold at fills[0], so
// the second depth could never be held: the gate had nothing to measure until
// two entries existed, and the first fast pair was spent proving the ladder
// was cascading rather than being stopped. That unheld second depth is the
// fill the rest of a cascade is built on. Seeding at fills[0]+base makes the
// rule a minimum spacing: no two entries closer than the current hold.
//
// This costs nothing on a slow ladder — depths naturally spaced wider than the
// hold are already past the expiry — and bites exactly where it was meant to.
//
// The escalation still measures against the PREVIOUS hold's expiry, not the
// previous fill: a depth that lands the instant a hold lifts is still part of
// the same drop, so the next hold doubles. Measured from the previous fill the
// gap would be >= the hold by construction and the rule could never escalate.
//
// A genuine pause RESETS the escalation. `step` means "how deep into one
// cascade are we", and a ladder that waited out a full window is no longer in
// that cascade; carrying the count forever would hand an escalated hold to a
// trade whose only fast pair happened long before.
func depthSpacingEligibleFrom(fills []depthFill) depthSpacingState {
	if len(fills) == 0 {
		return depthSpacingState{}
	}

	state := depthSpacingState{
		step:         1,
		hold:         depthSpacingHoldFor(1),
		eligibleFrom: fills[0].At.Add(depthSpacingHoldFor(1)),
	}
	for _, fill := range fills[1:] {
		if fill.At.Sub(state.eligibleFrom) < DepthSpacingWindow {
			// Landed at or right after the expiry: the drop is still running.
			state.step++
		} else {
			state.step = 1
		}
		state.hold = depthSpacingHoldFor(state.step)
		state.eligibleFrom = fill.At.Add(state.hold)
	}
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

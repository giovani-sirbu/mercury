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
// next activation adds one only when its depth filled WHILE the previous
// activation's hold still stood, and the activation itself lands less than
// DepthSpacingWindow past that hold's expiry. Both are measured from the fill
// of the depth that activated, not from the event, and against the expiry, not
// the previous fill: measured from the previous fill the gap would be >= the
// hold by construction and the rule could never escalate. The count goes back
// to 1 on either of two resets, and is never zero and never capped; only the
// hold is, by depthSpacingHoldFor.
//
//   - A depth that filled at or after the previous expiry waited that hold out:
//     the hold ended by TIME and its price release never bought the depth. The
//     gate refuses every stopLoss proposal while a hold stands unless the price
//     release lifts it (depthSpacingPriceReleased), so the only way a depth arms
//     inside a hold is the market paying the price the hold asked for, and that
//     is the cascade the step counts. A ladder that waits every hold out is the
//     spacing doing its job and never deepens.
//   - An activation DepthSpacingWindow or more past that expiry is a genuine
//     pause, however its depth arrived.
//
// Fills are chronological, so when activations skip depths (a depth filled with
// no hold) the newest depth filling before the previous expiry still means the
// release bought it and every depth between.
//
// The fill stamp is the same clock the header of depthSpacingHoldReason.go
// describes. In sisyphus backtesting TradesHistory.CreatedAt is the placement
// tick, the arm time, exact; in hermes it is the fill time, the bounce a
// tolerance past the arm, so a hold released seconds before its expiry can read
// as released by time there. That is the lenient direction: a reset, never a
// spurious escalation.
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
		filled := fills[depth-1].At
		if depthSpacingCascades(state.step, expiry, activations[depth], filled) {
			state.step++
		} else {
			state.step = 1
		}
		state.hold = depthSpacingHoldFor(state.step)
		expiry = filled.Add(state.hold)
	}
	state.eligibleFrom = expiry
	return state
}

// depthSpacingCascades reports whether an activation continues the cascade the
// previous activation's hold, expiring at `expiry`, was part of. `step` is the
// count that activation left, zero before the first, which has nothing to
// continue. The depth must have filled strictly before the expiry, which only
// the price release can do, and the activation must land inside
// DepthSpacingWindow past it; see depthSpacingEligibleFrom.
func depthSpacingCascades(step int, expiry, activated, filled time.Time) bool {
	if step == 0 {
		return false
	}
	if activated.Sub(expiry) >= DepthSpacingWindow {
		return false
	}
	return filled.Before(expiry)
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

package cooldown

import "time"

// FirstFillMaxHold is how long the first-fill gate may hold an entry before
// it gives up and lets the trade in at the tick price. Zero disables the cap
// and restores the release-by-price-only gate.
//
// The cap exists for one case. The gate is priced as a band around the
// reference — up(R) above, arm(R) below — so a market that drifts sideways
// inside that band never releases, and the band is proportional to the ladder
// step while volatility is not. A rally is not that case: it crosses up(R)
// and releases on the tick it does. So the cap bounds the sideways hold and
// nothing else, and what it costs is an unconditional buy at the tick price
// every time it fires.
//
// The clock is the tick clock, measured from the FIRST waiting row: the hold
// activates on the tick sophos refuses the verdict, and that row is stamped
// with it (gates.SaveHoldLog). An unknown clock never expires — the whole
// gate would silently switch off on an engine that does not stamp its rows,
// which is the opposite of every other fail-open in this package, where
// failing open means not holding.
//
// The operator-facing copy of this behaviour lives in
// cp/constants/strategy-params.ts and has to move with it.
const FirstFillMaxHold = 4 * time.Hour

// Depth-spacing tunables — the Cooldown flag's gate on an open position: the
// one that keeps a ladder from cascading through every depth in one drop.
// What the gate does with them, and which clock each engine measures with, is
// on depthSpacingHoldReason.go.
//
// Read the boundary they compute together, not any one of them. A depth is
// held for the current hold, and the escalation measures the next fill
// against that hold's EXPIRY, so what counts as "the same drop" is the hold
// plus the window; the window alone is only its tail. Together they are a
// brake on how fast capital is committed, and the price release is what keeps
// that brake from being a pure delay.
const (
	// DepthSpacingWindow is the grace the escalation allows PAST a hold: a
	// depth that lands within this of the previous hold's expiry is still the
	// same drop, and a depth that lands later resets the count. It is
	// therefore not by itself the line between a ladder and a cascade — that
	// line is the standing hold plus this window.
	DepthSpacingWindow = 4 * time.Hour
	// DepthSpacingBaseHold is what the first fast depth costs the ladder, and
	// the value the escalation starts from.
	DepthSpacingBaseHold = 4 * time.Hour
	// depthSpacingFactor is the escalation. A drop that keeps filling depths
	// the instant the previous hold lifts is falling faster than the grid was
	// built for, so the gate grows the wait as the drop consumes depths.
	// depthSpacingHoldFor scales through float64, so a fractional factor
	// stays valid here without a code change.
	depthSpacingFactor = 1.5
	// depthSpacingMaxHold caps one hold, and is what a fully escalated ladder
	// is left with. Past it the gate stops being a gate and becomes an
	// outage: the trade sits out the bottom of the very move it was slowed
	// down for, and that bottom depth is the one that pays for the rest of
	// the ladder. The price release is what bounds the damage beyond it.
	depthSpacingMaxHold = 4 * time.Hour
)

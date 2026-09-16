package cooldown

import "time"

// FirstFillMaxHold is how long the first-fill gate may hold an entry before
// it gives up and lets the trade in at the tick price. Zero disables the cap
// and restores the release-by-price-only gate.
//
// The gate is priced as a band around the reference — up(R) above, arm(R)
// below — so a market that goes sideways inside that band holds forever, and
// the band is proportional to the ladder step while volatility is not. BTC
// (percentage 1.75, so a band of roughly ±1.9%) held trade 56980 of backtest
// 140 for NINE DAYS, from 12 to 22 September 2025, on a market that never
// left 115,324-117,448. It was not a stuck trade — the ten rows are
// gates.SaveHoldLog re-logging one standing hold once a day, and it did
// finally arm and fill 2.9% under its reference — but nine days of a pair
// not trading is a cost the gate never priced.
//
// This is the eight-hour cap that was removed on 2026-09-05 coming back,
// wider. It was removed because a rally used to make the trade enter HIGHER
// for having waited; that is no longer the shape of the gate — a rally now
// releases the entry the moment it crosses up(R), on that tick — so the cap
// is left with only the sideways case, which is the one it is being asked to
// bound.
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
const FirstFillMaxHold = 12 * time.Hour

// Depth-spacing tunables — the Cooldown flag's gate on an open position. The
// two populations in trade 25858 separate cleanly: the gaps that emptied the
// budget were 7-14 minutes, the gaps that followed a real pullback were 30m
// and 90m. Read the boundary the code computes, described on
// DepthSpacingBaseHold, not the individual numbers. What the gate does with
// them, and which clock each engine measures with, is on
// depthSpacingHoldReason.go.
//
// The settings below are the 2026-09-07 calibration and they are FAR wider
// than the trade they were first shaped on. Measured over backtest 136's 2397
// adds (median gap 3h17m): the base hold alone stands in front of 57% of them,
// and the escalation's window is wider than either hold, so a reset needs a
// gap of 16h at step 1 and 18h past it — which few adds clear. In practice a
// ladder escalates on most depths and settles at the ceiling one fast depth
// in. That is a deliberate brake on how fast capital is committed, not a
// cascade filter any more, and the price release below is what keeps it from
// being a pure delay.
const (
	// DepthSpacingWindow is the grace the escalation allows PAST a hold:
	// a depth that lands within this of the previous hold's expiry is still
	// the same drop, and a depth that lands later resets the count. It is
	// therefore not by itself the line between a ladder and a cascade —
	// that line is the standing hold plus this window.
	DepthSpacingWindow = 12 * time.Hour
	// DepthSpacingBaseHold is what the first fast depth costs the ladder.
	// The escalation compares a fill against the previous hold's expiry, so
	// what counts as "the same drop" is base + window at step 1 (16h), then
	// hold + window at every step after it (18h). The window alone is only
	// the tail of that.
	DepthSpacingBaseHold = 4 * time.Hour
	// depthSpacingFactor is the escalation. A drop that keeps filling depths
	// the instant the previous hold lifts is falling faster than the grid
	// was built for, so the gate grows the wait as the drop consumes depths.
	// With the base and ceiling below the sequence is 4h then 6h, clamped
	// from the second fast depth on — the escalation is one step, and the
	// ceiling does the rest of the work. depthSpacingHoldFor scales through
	// float64, so a fractional factor stays valid here without a code change.
	depthSpacingFactor = 2
	// depthSpacingMaxHold caps one hold. The cap is what a fully escalated
	// ladder is left with, and past it the gate stops being a gate and
	// becomes an outage: the trade sits out the bottom of the very move it
	// was slowed down for, and that bottom depth is the one that pays for the
	// rest of the ladder. Six hours is just past the slowest timeframe any
	// gate in this package reads — the price release is what bounds the
	// damage beyond that.
	depthSpacingMaxHold = 6 * time.Hour
)

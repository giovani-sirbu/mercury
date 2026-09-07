package cooldown

import "time"

// Depth-spacing tunables — the Cooldown flag's gate on an open position. The
// two populations in trade 25858 separate cleanly: the gaps that emptied the
// budget were 7-14 minutes, the gaps that followed a real pullback were 30m
// and 90m. Both sit inside the settings below, which are deliberately wider
// than that trade needs — read the boundary the code computes, described on
// DepthSpacingBaseHold, not the individual numbers. What the gate does with
// them, and which clock each engine measures with, is on
// depthSpacingHoldReason.go.
const (
	// DepthSpacingWindow is the grace the escalation allows PAST a hold:
	// a depth that lands within this of the previous hold's expiry is still
	// the same drop, and a depth that lands later resets the count. It is
	// therefore not by itself the line between a ladder and a cascade —
	// that line is the standing hold plus this window.
	DepthSpacingWindow = 60 * time.Minute
	// DepthSpacingBaseHold is what the first fast depth costs the ladder.
	// It is LARGER than the window, so the two thresholds are not the same
	// number and must not be read as one: the escalation compares a fill
	// against the previous hold's expiry, so what counts as "the same drop"
	// is base + window at step 1 (150m), then hold + window at every step
	// after it (240m, 300m). The window alone is only the tail of that.
	DepthSpacingBaseHold = 90 * time.Minute
	// depthSpacingFactor is the escalation. A drop that keeps filling depths
	// the instant the previous hold lifts is falling faster than the grid
	// was built for, so the gate has to grow faster than the drop consumes
	// depths: a linear backoff still let trade 25858's seven depths through
	// inside two hours, doubling does not. From the base hold the sequence
	// is 90m, 3h, 4h, and it is clamped from the third fast depth on.
	depthSpacingFactor = 2
	// depthSpacingMaxHold caps one hold. Past four hours the gate stops
	// being a gate and becomes an outage: the trade sits out the bottom of
	// the very move it was slowed down for, and that bottom depth is the one
	// that pays for the rest of the ladder. Four hours is also the slowest
	// timeframe any gate in this package reads, so nothing here reasons
	// about a horizon longer than that. With the base above, the third
	// consecutive fast depth already reaches it.
	depthSpacingMaxHold = 4 * time.Hour
)

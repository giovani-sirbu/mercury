package depthspacing

import "time"

// The depth-spacing gate's schedule (depthSpacingHoldFor, depthSpacingCascades): a depth is held
// from its fill for the current hold, the next activation escalates when its depth filled before
// that hold's expiry and landed less than Window past it, and the price release lifts a hold early.
const (
	// factor is the escalation of the hold per step; it goes through float64, so it may be fractional.
	factor = 2
	// BaseHold is the hold of step 1, where the escalation starts.
	BaseHold = 2 * time.Hour
	// maxHold caps one hold; the price release bounds the damage beyond it.
	maxHold = 3 * time.Hour
	// Window is the pause that resets the escalation: an activation this long or more past the
	// previous hold's expiry starts the count over, whatever its fill.
	Window = 5 * time.Hour
)

// disabledWhenActiveSmartSell lifts the hold while a smart sell gate stands triggered on the trade
// (smartsell.Triggered); a variable only so the tests can move it.
var disabledWhenActiveSmartSell = false

// disabledWhenEarlyTakeProfit lifts the hold while the early take profit stands latched on the
// trade (smartsell.EarlyTakeProfitLatched), for the rest of the ladder's life; a variable only so
// the tests can move it.
var disabledWhenEarlyTakeProfit = false

// startStepFromDepth starts a cascade at the activation's depth less startStepOffset instead of 1
// (depthSpacingStartStep); a variable only so the tests can move it.
var startStepFromDepth = false

// startStepOffset is how far under the depth the start sits, read only while startStepFromDepth is
// on; a variable only so the tests can move it.
var startStepOffset = 0

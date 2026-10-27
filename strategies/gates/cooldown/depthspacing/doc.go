// Package depthspacing is the Cooldown flag's depth-spacing gate: DepthSpacingHold refuses a
// stopLoss proposal while the hold the last fast depth earned still stands, unless the market paid
// the price release, a smart take loss gate stands triggered on the trade (disabledWhenActiveSTL,
// smarttakeloss.Triggered) or the early take profit stands latched on it
// (disabledWhenEarlyTakeProfit, smarttakeloss.EarlyTakeProfitLatched). It folds the fills and its own
// events (depthSpacingActivations) into
// the next arming instant with an escalating hold (depthSpacingEligibleFrom; BaseHold, factor,
// maxHold, Window; a cascade starts at the activation's depth less the offset with
// startStepFromDepth, startStepOffset) and prices the early lift (depthSpacingReleasePrice); it
// fails open without a tick clock.
//
// Vocabulary: depth, fill, ladder, arming, hold, escalation, release, event, row, trigger.
package depthspacing

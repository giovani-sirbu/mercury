// Package cooldown is the Cooldown flag's shared vocabulary: the gate names of its strategy events
// (GateFirstFill, GateDepthPriority, GateDepthSpacing), the first-fill event kinds, the event
// documents (FirstFillEvent, DepthPriorityEvent, DepthSpacingEvent) with their constructors, the
// first-fill row prefixes and DepthPriorityHoldMarker. The events are the gates' state; the rows
// are byte-stable text. The holds and their settings live in the leaf packages firstfill,
// depthspacing and depthpriority, which import this one; trades/actions composes them into the
// chain (ShouldHold).
//
// Vocabulary: first fill, depth, ladder, hold, wallet, event, row.
package cooldown

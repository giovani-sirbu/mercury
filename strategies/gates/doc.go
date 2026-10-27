// Package gates is the root of the strategy params' gates: one folder per param (cooldown,
// smarttakeloss, dynamicparams) and one per market flag (patterns, ai), each gate with its own
// constants.go. The root keeps only what every gate shares and imports none of them: Hold (a gate's
// refusal) and EventHeld, PositionType (a force-trailing re-anchor reads as the position it re-arms)
// and BlocksNewFuturesEntry (the futures pre-chain skip). trades/actions.ShouldHold orders the holds
// and hands each to tradelog.SaveHoldLog; smarttakeloss alone also forces exits (Apply).
//
// Vocabulary: gate, hold, flag, fill, depth, ladder, row, event, verdict.
package gates

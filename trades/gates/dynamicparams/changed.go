package dynamicparams

import "github.com/giovani-sirbu/mercury/trades/aggragates"

// Changed reports whether the increase the reads raise differs between two
// ticks — the edge the engines write one TransitionMessage row on. It
// compares the increases, not the tiers or the reads: a read moving from
// neutral to bullish, or the bearish read passing from one row to the other,
// raises what it raised before and writes nothing, and while MixedIncrease is
// IncreaseNone neither does a move between the base and the mixed tier. A
// block that stops being read is an edge only when the reads before it raised
// something.
func Changed(previous, current aggragates.DynamicParamsIndicators) bool {
	return changedFor(previous, current, MixedIncrease)
}

// changedFor is Changed with the mixed tier's increase handed in, as raiseFor
// takes it.
func changedFor(previous, current aggragates.DynamicParamsIndicators, mixed Increase) bool {
	return raiseFor(TierOf(previous), mixed) != raiseFor(TierOf(current), mixed)
}

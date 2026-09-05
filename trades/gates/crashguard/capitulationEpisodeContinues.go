package crashguard

// capitulationEpisodeContinues names the non-stopLoss positions a trade can
// pass through WITHOUT leaving its ladder: the force-trailing states only
// re-anchor PositionPrice and hand the rung back to stopLoss. The live
// capitulation episode (hadCrash, quiet-window counter) must survive them;
// every other non-stopLoss position — a profit arm, a close, a smart
// take-loss state — ends it.
func capitulationEpisodeContinues(positionType string) bool {
	switch positionType {
	case "forceTrailingStopLoss", "forceTrailingTakeProfit":
		return true
	}
	return false
}

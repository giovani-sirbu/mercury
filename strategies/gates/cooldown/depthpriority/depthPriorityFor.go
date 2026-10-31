package depthpriority

import "github.com/giovani-sirbu/mercury/trades/aggragates"

// depthPriorityFor names the ladder the wallet is kept for, as far as own is concerned: the
// best-ranked candidate ahead of own with costed depths left (reserveCeiling), else the best-ranked
// full one, which keeps nothing. own ranks from its own depth and costs and its own row in the view
// is skipped by trade id.
func depthPriorityFor(own aggragates.LadderDepth, ladders []aggragates.LadderDepth) (aggragates.LadderDepth, bool) {
	ownRanks := isDepthPriorityCandidate(own)

	var next, full aggragates.LadderDepth
	nextFound, fullFound := false, false

	for _, candidate := range ladders {
		if candidate.TradeID == own.TradeID {
			continue
		}
		if candidate.Asset != own.Asset || !isDepthPriorityCandidate(candidate) {
			continue
		}
		if ownRanks && !outranksDepthPriority(candidate, own) {
			continue
		}

		if candidate.Depth >= reserveCeiling(candidate) {
			if !fullFound || outranksDepthPriority(candidate, full) {
				full, fullFound = candidate, true
			}
			continue
		}

		if !nextFound || outranksDepthPriority(candidate, next) {
			next, nextFound = candidate, true
		}
	}

	if nextFound {
		return next, true
	}

	return full, fullFound
}

// outranksDepthPriority is the gate's one ordering: the deeper ladder first, then the one planned
// cheaper to finish from its last fill (a key that stands still between fills), then the lower trade id.
func outranksDepthPriority(candidate, against aggragates.LadderDepth) bool {
	if candidate.Depth != against.Depth {
		return candidate.Depth > against.Depth
	}

	if candidate.PlannedRemainingCost != against.PlannedRemainingCost {
		return candidate.PlannedRemainingCost < against.PlannedRemainingCost
	}

	return candidate.TradeID < against.TradeID
}

// deepestDepthPriorityCandidate is the front of the view among the ladders spending the asset, full
// or not; the suites pin their wallet fixtures with it.
func deepestDepthPriorityCandidate(ladders []aggragates.LadderDepth, asset string) (aggragates.LadderDepth, bool) {
	var priority aggragates.LadderDepth
	found := false

	for _, candidate := range ladders {
		if candidate.Asset != asset || !isDepthPriorityCandidate(candidate) {
			continue
		}
		if !found || outranksDepthPriority(candidate, priority) {
			priority, found = candidate, true
		}
	}

	return priority, found
}

// isDepthPriorityCandidate is a ladder with at least one fill on a pair whose depths are known; a
// ladder full for the gate (reserveCeiling) still ranks until it closes, keeping nothing.
func isDepthPriorityCandidate(candidate aggragates.LadderDepth) bool {
	if candidate.MaxDepth <= 0 {
		return false
	}

	return candidate.Depth >= 1
}

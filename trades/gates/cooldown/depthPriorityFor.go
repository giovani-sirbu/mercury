package cooldown

import "github.com/giovani-sirbu/mercury/trades/aggragates"

// The ranking: which ladder of the wallet the reserve is kept for, as far as
// the ladder being asked about is concerned.
//
// The deeper ladder is in front. Two at the same depth are split by what
// their remaining depths were planned to cost from their last fill — the one
// cheaper to finish goes first, finishing one ladder being the whole point —
// and at the same cost by the lower trade id. That key has to stand still
// between fills: every engine must name the same ladder, and the hold row,
// deduplicated on its full string, must not change its mind while nothing has
// filled. The amount the reserve is measured in cannot be that key. It is
// priced from the position, which moves when a ladder arms and while it
// trails, so ranking on it would hand the front back and forth between two
// level ladders on every re-arm; the planned cost moves only when a depth
// fills, which is when the depth itself moves.
//
// The reservation ends when the priority LEAVES the wallet view: it closes,
// or it blocks on the very entry the wallet was being kept for and the
// engines drop it. Filling its last depth does not end it. Ending a full
// ladder's candidacy there would release every waiting sibling in the same
// instant, into a wallet the priority has just spent down to its last depth:
// each one would reach the funds gate and be BLOCKED rather than held. A
// blocked ladder leaves the view, so by the time the priority closed and
// refilled the wallet the deepest siblings would no longer be in it, and the
// shallowest ladder still active would inherit the wallet and hold the
// re-admitted deeper ones behind it — the ranking inverted by an accident of
// timing. Held, a sibling stays active, stays in the view, keeps its rank,
// and resumes on the tick the wallet refills.
//
// A full ladder keeps nothing, though, so while it stands in front the wallet
// is kept for the NEXT ladder in line that still has depths left: the
// best-ranked one ahead of the ladder being asked about. The wallet refills
// while a full ladder stands — its own close settles in parts before it
// leaves the view, and other ladders close — and a wallet kept for nobody
// goes to whichever sibling prints first, then to the next one that prints,
// until several have armed the same depth on one refill and none can reach
// its last. With no such ladder ahead nothing is kept back: every entry the
// wallet can pay for buys — that IS the release at the ceiling — and one it
// cannot pay for waits rather than being blocked.
//
// Full means full at the ceiling of the rows the ladder trades (ladder.DepthOf
// reads them): a ladder that opened with a raise has more depths to fill than
// its stored rows say, so standing at the stored ceiling it still ranks by its
// depth and its planned cost and keeps the wallet for what it has left, and
// the ladder being asked about is read the same way as every ladder of the
// view.
//
// A ladder the engine still lets add past its ceiling is its own priority
// under this rule, so nothing here holds it; whether that entry is placed is
// the funds gate's call alone, exactly as it was before this gate existed.
//
// A ladder blocked on its next entry reserves the wallet only once the wallet
// can re-admit it. Until then it cannot take the entry it is blocked on, so
// the others would wait for a retry only a close could fund, and the wallet
// would starve with money in it — the situation this gate exists to prevent.
// The engines own that membership: a blocked ladder joins the view they build
// once its entry is affordable again — production's unblock pass re-activates
// it, a replay reads its block record against the print — and from then on it
// ranks, and reserves, like any other ladder.

// depthPriorityFor names the ladder the wallet is kept for, as far as own is
// concerned: the best-ranked candidate AHEAD of own that still has depths
// left, or — when every candidate ahead is full — the best-ranked full one,
// which keeps nothing and so holds only what the wallet cannot pay for. Ahead
// means outranking own when own is a candidate itself, and any candidate at
// all when it is not. One pass over the view; pure, so the rule can be
// exercised without an event.
//
// own takes part in the ranking whether or not the view carries it. The view
// is what the OTHER ladders see, built on the engine's own schedule, while
// the trade being ticked knows its own depth and costs first-hand — read off
// its own events and rows, a raise included, by the helper that builds the
// view. Judged against the view alone it would be parked behind a shallower
// ladder the moment the wallet could pay for it again, which is the inversion
// this gate exists to avoid.
//
// own's own row is skipped by its trade id, explicitly. With the depth as the
// only key it did not have to be — an active own met itself as the best
// candidate, tied on its own id and outranked it — but a cost key breaks
// that: a view read a moment before this tick, or priced off another copy of
// the trade, can carry own at another depth or cost than the trade itself,
// and a row like that must never hold the trade it describes.
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

		if candidate.Depth >= candidate.MaxDepth {
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

// outranksDepthPriority is the one ordering every part of this gate ranks
// by: the deeper ladder first, then the one planned cheaper to finish from
// its last fill, then the lower trade id — the ranking above says why the
// cost is the planned one.
func outranksDepthPriority(candidate, against aggragates.LadderDepth) bool {
	if candidate.Depth != against.Depth {
		return candidate.Depth > against.Depth
	}

	if candidate.PlannedRemainingCost != against.PlannedRemainingCost {
		return candidate.PlannedRemainingCost < against.PlannedRemainingCost
	}

	return candidate.TradeID < against.TradeID
}

// deepestDepthPriorityCandidate is the view read on its own: the ladder at the
// very front of it among those spending the asset, full or not, by the one
// ordering, with no managed trade to rank against. The gate asks
// depthPriorityFor, which knows the managed trade and looks past a full
// ladder; this is the plainer question the suites pin their wallet fixtures
// with.
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

// isDepthPriorityCandidate is the ladder worth putting in front of the
// wallet: one that has filled at least one entry, on a pair whose configured
// depths are known. A ladder with no fills has no position to finish, and one
// whose pair carries no settings row has no ladder to speak of at all.
//
// A FULL ladder is still a candidate. It needs nothing more, so it keeps
// nothing for itself — the wallet is kept for the next ladder in line behind
// it — but it keeps its place until it closes, for the reason the ranking
// above gives.
func isDepthPriorityCandidate(candidate aggragates.LadderDepth) bool {
	if candidate.MaxDepth <= 0 {
		return false
	}

	return candidate.Depth >= 1
}

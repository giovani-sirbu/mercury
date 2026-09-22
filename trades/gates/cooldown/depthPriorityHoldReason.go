package cooldown

import (
	"fmt"

	"github.com/giovani-sirbu/mercury/events"
	"github.com/giovani-sirbu/mercury/trades/aggragates"
	"github.com/giovani-sirbu/mercury/trades/gates"
	"github.com/giovani-sirbu/mercury/trades/ladder"
)

// Depth priority is the Cooldown flag's third gate, and the only one that
// looks past the trade it is asked about: it reads every ladder of the same
// wallet.
//
// The shape it answers to is a market where the pairs fall together. Every
// ladder of the wallet arms its next entry within hours of the others, the
// funds are spread over all of them, and not one reaches the depth its grid
// was sized for — the depth whose average entry price is low enough to be
// paid back by a small bounce. The wallet ends up holding several
// half-filled ladders, all of them under water, instead of one finished one.
//
// So while a ladder is within the margin of its last configured depth, the
// wallet is reserved for it: the shallower ladders of that wallet wait. When
// the deep one fills its last entry it stops being a candidate — a full
// ladder has no entry left to prioritize — and when it closes it leaves the
// wallet view altogether, so the others are released without anything having
// to remember that they were held.
//
// Blocked siblings take part on BOTH sides. A funds-blocked ladder is still
// a candidate, because the whole point is to keep the wallet free for the
// retry of the deep one, and a blocked ladder is still held, because a
// profitable close elsewhere is the capital it would otherwise race for.
//
// It reads no clock, no indicator and no persisted state: only the depths
// the engine handed it. The engines fill Params.WalletLadders on the ticks
// DepthPriorityApplies allows and leave it nil everywhere else, and a nil or
// empty view holds nothing — the fail-open posture of this whole package. A
// hermes fetch that fails yields nil for the same reason: no wallet is held
// on a guess.

// DepthPriorityApplies reports whether this tick can consume the wallet view
// at all, so an engine knows whether to build or fetch one. It is the gate's
// own transition rule, kept here rather than in each engine so the four
// surfaces cannot drift apart on which ticks are gated.
//
// The gated transitions are the ones that spend NEW capital: the first fill
// (a new trade has the smallest depth of all and its entry competes for the
// same wallet) and every add, including the force-trailing re-anchors and
// re-arms that resolve to a stopLoss. Never a close: a gate on new capital
// must not defer an exit, and here the exit is literally what releases the
// ladders that are waiting.
//
// oldPosition is Params.OldPosition and position is the position the chain is
// about to run; a re-anchor is mapped onto the family it re-arms exactly as
// the gates themselves map it.
func DepthPriorityApplies(params aggragates.StrategyParams, oldPosition, position string) bool {
	if !DepthPriority || !params.Cooldown {
		return false
	}

	return oldPosition == "new" || gates.PositionType(position) == "stopLoss"
}

// DepthPriorityHoldReason is the gate. Empty means the chain may proceed.
// The caller owns the flag, exactly like DepthSpacingHoldReason.
//
// Impasse children are out of it on both sides: they belong to their impasse
// chain and spend what the parent's close freed, not the wallet the parent
// competes for — the same exclusion the smart take loss makes.
func DepthPriorityHoldReason(event events.Events, position string) string {
	if !DepthPriorityApplies(event.Trade.Strategy.Params, event.Params.OldPosition, position) {
		return ""
	}
	if event.Trade.ParentID != 0 {
		return ""
	}

	own := ladder.DepthOf(event.Trade)

	priority, held := depthPriorityOver(own, event.Params.WalletLadders)
	if !held {
		return ""
	}

	// The message must stay byte-identical for as long as the hold stands:
	// gates.SaveHoldLog deduplicates on the full string, so anything that
	// moves tick by tick would write a row per tick. Both depths are frozen
	// while the hold stands — a held entry is precisely one that has not
	// filled, and the priority ladder's own next entry is what the wallet is
	// being kept for.
	return fmt.Sprintf(
		"cooldown: depth priority, %s at depth %d of %d takes the next entry, this ladder waits at depth %d of %d",
		priority.Symbol, priority.Depth, priority.MaxDepth, own.Depth, own.MaxDepth,
	)
}

// depthPriorityOver names the ladder own has to wait for, and reports whether
// it has to wait at all. Pure: the rule is decided on depths alone.
//
// A ladder at the same depth as the priority is not a smaller one and is not
// held — which also means a ladder is never held by itself, since it appears
// in the view it is compared against.
func depthPriorityOver(own aggragates.LadderDepth, ladders []aggragates.LadderDepth) (aggragates.LadderDepth, bool) {
	priority, found := deepestDepthPriorityCandidate(ladders)
	if !found || own.Depth >= priority.Depth {
		return aggragates.LadderDepth{}, false
	}

	return priority, true
}

// deepestDepthPriorityCandidate picks the ladder the wallet is reserved for:
// the deepest candidate, ties broken by the lowest trade id so every engine
// names the same one and the hold row stays stable while two ladders sit at
// the same depth.
func deepestDepthPriorityCandidate(ladders []aggragates.LadderDepth) (aggragates.LadderDepth, bool) {
	var priority aggragates.LadderDepth
	found := false

	for _, candidate := range ladders {
		if !isDepthPriorityCandidate(candidate) {
			continue
		}
		if !found || candidate.Depth > priority.Depth {
			priority, found = candidate, true
			continue
		}
		if candidate.Depth == priority.Depth && candidate.TradeID < priority.TradeID {
			priority = candidate
		}
	}

	return priority, found
}

// isDepthPriorityCandidate is the ladder worth reserving the wallet for: deep
// enough to be within the margin of its last configured depth, and not yet
// there. A ladder whose configured depths are unknown (a pair carrying no
// settings row) reserves nothing — the rule has no ceiling to measure
// against.
func isDepthPriorityCandidate(candidate aggragates.LadderDepth) bool {
	if candidate.MaxDepth <= 0 {
		return false
	}

	return candidate.Depth > candidate.MaxDepth-DepthPriorityMargin && candidate.Depth < candidate.MaxDepth
}

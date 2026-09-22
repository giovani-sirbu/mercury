package cooldown

import (
	"fmt"

	"github.com/giovani-sirbu/mercury/events"
	"github.com/giovani-sirbu/mercury/helpers"
	"github.com/giovani-sirbu/mercury/trades/aggragates"
	"github.com/giovani-sirbu/mercury/trades/gates"
	"github.com/giovani-sirbu/mercury/trades/ladder"
)

// Depth priority is the Cooldown flag's third gate, and the only one that
// looks past the trade it is asked about: it reads every ladder of the same
// wallet and the wallet's own free balance.
//
// The shape it answers to is a market where the pairs fall together. Every
// ladder of the wallet arms its next entry within hours of the others, the
// funds are spread over all of them, and not one reaches the depth its grid
// was sized for — the depth whose average entry price is low enough to be
// paid back by a small bounce. The wallet ends up holding several
// half-filled ladders, all of them under water, instead of one finished one.
//
// So the wallet is RESERVED for the deepest ladder: the funds it still needs
// for its remaining depths are kept out of every other ladder's reach. A
// sibling's entry is held only when placing it would leave the wallet under
// that amount — when the wallet covers both, everybody buys, which is what
// keeps the reserve from being a queue. The ladder the wallet is kept for is
// never held by its own reservation.
//
// The reserve is measured from the deepest ladder's FIRST depth, not from
// some band near its last one. A depth says nothing about money: a ladder one
// entry short of its ceiling can need more than the wallet holds, and one
// five entries short can need almost nothing. Arming only near the ceiling
// reserves the wallet at the point where the remaining entries — the biggest
// ones the ladder places — can no longer be afforded, which is too late to be
// a reserve at all. Measuring the money instead means the gate arms exactly
// when the sums stop fitting and stays out of the way while they do.
//
// The reservation ends when the priority LEAVES the wallet view: it closes,
// or it blocks on the very entry the wallet was being kept for and the
// engines drop it. Filling its last depth does not end it. A full ladder
// needs nothing more, so its reserve falls to nothing and every sibling the
// wallet can pay for buys — that IS the release at the ceiling — but the
// ladder stays in front until it closes, and a sibling the wallet cannot pay
// for keeps waiting instead.
//
// Which is the whole reason a full ladder stays in front. Ending its
// candidacy there releases every waiting sibling in the same instant, into a
// wallet the priority has just spent down to its last depth: each one reaches
// the funds gate and is BLOCKED rather than held. A blocked ladder leaves the
// view, so by the time the priority closes and refills the wallet the deepest
// siblings are no longer in it, and the shallowest ladder still active
// inherits the wallet and holds the re-admitted deeper ones behind it — the
// ranking inverted by an accident of timing. Held, a sibling stays active,
// stays in the view, keeps its rank, and resumes on the tick the close
// refills the wallet.
//
// A ladder the engine still lets add past its ceiling is its own priority
// under this rule, so nothing here holds it; whether that entry is placed is
// the funds gate's call alone, exactly as it was before this gate existed.
//
// A ladder blocked on its next entry never reserves the wallet. It cannot
// take the entry it is blocked on, so the others would wait for a retry that
// only a close could fund, and the wallet would starve with money in it — the
// situation this gate exists to prevent. The engines keep such a ladder out of
// the view they build, so membership is theirs. It costs the blocked ladder
// nothing but the reservation — once the wallet can afford its entry it
// competes again, and while a deeper active ladder stands it is held like any
// other, because the managed trade's own depth and cost are read from the
// trade it is asked about, never from the view.
//
// Ladders are compared only against the ones spending the same asset: a long
// ladder spends the quote side of its pair and an inverse one the base side,
// and two ladders funded from different currencies never take money from each
// other.
//
// It reads no clock and no indicator: only the depths, the costs and the
// balance the engine handed it. The engines fill Params.WalletLadders and
// Params.WalletFree on the ticks DepthPriorityApplies allows and leave them
// nil everywhere else. A nil or empty view holds nothing, and a balance the
// engine could not name holds nothing either — the fail-open posture of this
// whole package. A hermes fetch that fails yields nil for the same reason: no
// wallet is held on a guess.
//
// The hold row names both ladders and never the balance. The balance moves on
// every tick and gates.SaveHoldLog deduplicates on the full string, so a row
// carrying it would be a new row per tick for as long as the reservation
// stood.

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

	free, freeKnown := walletFreeFor(event, own.Asset)
	if !freeKnown {
		return ""
	}

	// Which ladder the wallet is kept for comes first, and the managed
	// trade's own entry is priced only once there is something to weigh it
	// against. Pricing it re-runs the ladder's own sizing, this gate is asked
	// on every entry tick of every ladder, and on most of those ticks the
	// wallet is keeping nothing at all — so the order of these two is the
	// difference between a reserve and a tax on the tick path.
	priority, found := depthPriorityFor(own, event.Params.WalletLadders)
	if !found {
		return ""
	}

	_, ownCost := ladder.NextEntryCost(event.Trade, free)
	if !depthPriorityHolds(ownCost, free, priority.RemainingCost) {
		return ""
	}

	return depthPriorityHoldMessage(priority, own)
}

// depthPriorityHoldMessage says which ladder the wallet is in front of, and
// what it is waiting on: a ladder with depths left is keeping what it still
// needs, while a full one is keeping nothing and is simply not done with the
// wallet until it closes. An operator reading the second row knows no further
// entry will free the funds — only the close will.
//
// The message must stay byte-identical for as long as the hold stands:
// gates.SaveHoldLog deduplicates on the full string, so anything that moves
// tick by tick — the balance above all — would write a row per tick. Both
// depths are frozen while the hold stands: a held entry is precisely one that
// has not filled, and a priority that is keeping the wallet for its own next
// entry has not placed it either.
func depthPriorityHoldMessage(priority, own aggragates.LadderDepth) string {
	if priority.Depth >= priority.MaxDepth {
		return fmt.Sprintf(
			"cooldown: depth priority, %s at depth %d of %d holds the wallet until it closes, this ladder waits at depth %d of %d",
			priority.Symbol, priority.Depth, priority.MaxDepth, own.Depth, own.MaxDepth,
		)
	}

	return fmt.Sprintf(
		"cooldown: depth priority, %s at depth %d of %d keeps the wallet for its remaining depths, this ladder waits at depth %d of %d",
		priority.Symbol, priority.Depth, priority.MaxDepth, own.Depth, own.MaxDepth,
	)
}

// walletFreeFor is the balance the managed trade's next entry would be placed
// from: the engine's reading for the asset that entry spends, less the quote
// this wallet's inverse trades have already committed — exactly what
// funds.GetFundsQuantities subtracts before it compares. An asset the engine
// named no balance for is UNKNOWN, and the gate holds nothing on an unknown
// balance.
func walletFreeFor(event events.Events, asset string) (float64, bool) {
	if asset == "" {
		return 0, false
	}

	for _, balance := range event.Params.WalletFree {
		if balance.Asset != asset {
			continue
		}

		free := balance.Free
		if !event.Trade.Inverse {
			free -= helpers.FindUsedAmount(event.Params.InverseUsedAmount, asset)
		}

		return free, true
	}

	return 0, false
}

// depthPriorityFor names the ladder the wallet is in front of, as far as own
// is concerned: the best candidate of the view spending the same asset, but
// only when own does not outrank it. Pure, so the rule can be exercised
// without an event.
//
// own takes part in the ranking like any other ladder, and that is deliberate
// even though a blocked own is NOT in the view. The view is what the OTHER
// ladders of the wallet see; the trade being ticked knows its own depth
// first-hand, and a ladder that is out of the view only because it could not
// afford its entry has not stopped being the deepest one. Judging it against
// the view alone would park it behind a shallower ladder the moment the
// wallet could pay for it again, which is the inversion this gate exists to
// avoid.
//
// It also makes "a ladder never waits for itself" a consequence rather than a
// rule: an active own IS in the view, so it meets itself as the best
// candidate, ties on its own id, and outranks it.
func depthPriorityFor(own aggragates.LadderDepth, ladders []aggragates.LadderDepth) (aggragates.LadderDepth, bool) {
	priority, found := deepestDepthPriorityCandidate(ladders, own.Asset)
	if !found {
		return aggragates.LadderDepth{}, false
	}

	if isDepthPriorityCandidate(own) && !outranksDepthPriority(priority, own) {
		return aggragates.LadderDepth{}, false
	}

	return priority, true
}

// outranksDepthPriority is the one ordering every part of this gate ranks
// by: the deeper ladder is in front, and ladders at the same depth are split
// by the lower trade id so every engine names the same one and the hold row
// stays stable while two of them sit level.
func outranksDepthPriority(candidate, against aggragates.LadderDepth) bool {
	if candidate.Depth != against.Depth {
		return candidate.Depth > against.Depth
	}

	return candidate.TradeID < against.TradeID
}

// depthPriorityHolds is the reserve itself: the entry waits when placing it
// would leave the wallet under what the reserved ladder still needs. Level
// with it is not under it — a wallet that covers both lets everybody buy,
// which is what keeps the reserve from being a queue.
func depthPriorityHolds(ownCost, free, reserve float64) bool {
	return free-ownCost < reserve
}

// deepestDepthPriorityCandidate picks the ladder of the view the wallet is in
// front of: the best candidate spending the same asset, by the one ordering
// above.
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
// A FULL ladder is still a candidate. It needs nothing more — its remaining
// cost is nothing, so it holds only what the wallet cannot pay for anyway —
// but it keeps its place until it closes, because that close is what refills
// the wallet, and a sibling released before it lands is a sibling the funds
// gate blocks out of the view entirely.
func isDepthPriorityCandidate(candidate aggragates.LadderDepth) bool {
	if candidate.MaxDepth <= 0 {
		return false
	}

	return candidate.Depth >= 1
}

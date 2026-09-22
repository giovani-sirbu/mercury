package cooldown

import (
	"strings"
	"testing"

	"github.com/giovani-sirbu/mercury/trades/aggragates"
	"github.com/giovani-sirbu/mercury/trades/internal/testutil"
	"github.com/giovani-sirbu/mercury/trades/ladder"
)

// deepGridDepths is a grid with room for far more entries than the example
// wallet, so a ladder can sit one entry into it and still be the deepest one
// of its wallet.
const deepGridDepths = 12

// deepGridLadder maps a ladder of that grid into the view.
func deepGridLadder(id uint, symbol string, depth int) aggragates.LadderDepth {
	return ladder.DepthOf(testutil.LadderDepthTrade(id, symbol, depth, deepGridDepths))
}

// The wallet is reserved from the deepest ladder's FIRST depth, not from some
// band near its last one. A ladder one entry into a long grid already has a
// position to finish and an expensive tail to finish it with, and that is
// exactly when the reserve is worth anything: by the time such a ladder is
// near its ceiling the entries it still needs are the biggest it places, and a
// wallet spread over its siblings can no longer pay for them.
func TestDepthPriorityReservesFromTheFirstFilledDepth(t *testing.T) {
	requireDepthPriority(t)

	view := []aggragates.LadderDepth{deepGridLadder(14, "LINK/USDT", 1)}
	if view[0].RemainingCost <= 0 {
		t.Fatal("a ladder one entry into its grid must still name what it needs")
	}

	newcomer := testutil.LadderDepthTrade(21, "ADA/USDT", 0, deepGridDepths)
	reserve := view[0].RemainingCost

	if reason := DepthPriorityHoldReason(priorityEvent(newcomer, "new", view, reserve), "buy"); reason == "" {
		t.Fatal("a wallet that cannot cover the reserve and a first fill must hold the first fill")
	}
	if !strings.Contains(
		DepthPriorityHoldReason(priorityEvent(newcomer, "new", view, reserve), "buy"),
		"LINK/USDT at depth 1 of 12",
	) {
		t.Fatal("the row must name the ladder the wallet is kept for, however shallow it is")
	}
}

// What is reserved is what the ladder has LEFT of its planned grid: a cost
// for every depth still to fill, and nothing at all once it is full. That
// last step is the release nothing has to remember — the fill that finishes
// the ladder ends its reservation on the same tick.
//
// The amounts in between are deliberately NOT asserted to fall one by one.
// Each entry of a grid is the previous one times the multiplier at a price
// one step down, so whether filling a depth leaves more or less to pay for
// depends on which of the two outruns the other. Pinning a direction here
// would pin a tuning, not the rule.
func TestDepthPriorityReserveCountsOnlyTheDepthsLeftToFill(t *testing.T) {
	requireDepthPriority(t)

	for depth := 1; depth < deepGridDepths; depth++ {
		if remaining := deepGridLadder(14, "LINK/USDT", depth).RemainingCost; remaining <= 0 {
			t.Fatalf("depth %d of %d: a ladder with entries left must name a cost", depth, deepGridDepths)
		}
	}

	if full := deepGridLadder(14, "LINK/USDT", deepGridDepths).RemainingCost; full != 0 {
		t.Fatalf("a full ladder reserves %f, want nothing left to keep the wallet for", full)
	}

	// The tick that finishes the ladder: it keeps nothing from then on, so
	// the sibling held a moment earlier arms its next entry on the very same
	// wallet — the wallet can pay for that entry, it was only the reserve on
	// top of it that did not fit.
	nearlyDone := []aggragates.LadderDepth{deepGridLadder(14, "LINK/USDT", deepGridDepths-1)}
	done := []aggragates.LadderDepth{deepGridLadder(14, "LINK/USDT", deepGridDepths)}

	waiting := testutil.LadderDepthTrade(12, "ETH/USDT", 4, deepGridDepths)
	reserve := viewReserve(t, nearlyDone, walletAsset)
	free := reserve + nextEntryCostOf(t, waiting, reserve) - 1

	if reason := DepthPriorityHoldReason(priorityEvent(waiting, "buy", nearlyDone, free), "stopLoss"); reason == "" {
		t.Fatal("the sibling must wait while the ladder still has a depth to fill")
	}
	if reason := DepthPriorityHoldReason(priorityEvent(waiting, "buy", done, free), "stopLoss"); reason != "" {
		t.Fatalf("a full ladder keeps nothing the wallet can already pay for, got %q", reason)
	}
}

// The amount held against is the RESERVED ladder's own remaining cost, never
// the wallet's total need: a shallower ladder of the same wallet has more
// depths to fill and needs more money, and none of that is reserved. The gate
// keeps one ladder's tail out of reach, not everybody's.
func TestDepthPriorityReservesOnlyTheDeepestLaddersOwnRemainder(t *testing.T) {
	requireDepthPriority(t)

	deep := deepGridLadder(14, "LINK/USDT", 10)
	shallower := deepGridLadder(12, "ETH/USDT", 4)
	if shallower.RemainingCost <= deep.RemainingCost {
		t.Fatalf("the fixture must leave the shallower ladder needing more, got %f against %f", shallower.RemainingCost, deep.RemainingCost)
	}

	view := []aggragates.LadderDepth{deep, shallower}
	own := testutil.LadderDepthTrade(11, "BTC/USDT", 2, deepGridDepths)
	ownCost := nextEntryCostOf(t, own, deep.RemainingCost)

	if reason := DepthPriorityHoldReason(priorityEvent(own, "buy", view, deep.RemainingCost+ownCost), "stopLoss"); reason != "" {
		t.Fatalf("only the deepest ladder's remainder is reserved, got %q", reason)
	}
	if reason := DepthPriorityHoldReason(priorityEvent(own, "buy", view, deep.RemainingCost+ownCost-1), "stopLoss"); reason == "" {
		t.Fatal("a wallet one unit under the deepest ladder's remainder must hold the entry")
	}
}

// A pair carrying no settings row has no ceiling to have entries left
// against, so it can never reserve the wallet — but it still spends it, so a
// sibling that is still filling its own grid makes it wait. The two sides are
// separate on purpose: an unknown ceiling is a reason not to reserve for a
// ladder, never a reason to let it spend ahead of one whose ceiling is known.
func TestDepthPriorityHoldsALadderWhoseCeilingIsUnknown(t *testing.T) {
	requireDepthPriority(t)

	unknown := testutil.LadderDepthTrade(12, "ETH/USDT", 4, walletDepths)
	unknown.StrategyPair.StrategySettings = nil

	own := ladder.DepthOf(unknown)
	if own.MaxDepth != 0 {
		t.Fatalf("a pair without settings must report an unknown ceiling, got %d", own.MaxDepth)
	}
	if own.RemainingCost != 0 {
		t.Fatalf("a pair without settings must reserve nothing, got %f", own.RemainingCost)
	}
	if _, found := deepestDepthPriorityCandidate([]aggragates.LadderDepth{own}, own.Asset); found {
		t.Error("a ladder with an unknown ceiling must never reserve the wallet")
	}

	// Its own next entry cannot be priced either — the same missing row —
	// so what holds it is the reserve alone: a wallet under it waits.
	wallet := []aggragates.LadderDepth{own, walletLadder(14, "LINK/USDT", 7)}
	reserve := viewReserve(t, wallet, walletAsset)

	reason := DepthPriorityHoldReason(priorityEvent(unknown, "buy", wallet, reserve-1), "stopLoss")
	if !strings.Contains(reason, "LINK/USDT at depth 7 of 8") {
		t.Fatalf("reason = %q, want the ladder with a known ceiling to keep the wallet", reason)
	}
}

// Depths is configured as a float and the ceiling is the floored one, the
// same read the smart take loss arms on: a grid configured for a fraction
// over eight entries is a grid of eight, so its eighth fill finishes it —
// nothing left to reserve, and the row prints the floored ceiling.
func TestDepthPriorityMeasuresAFractionalCeilingFloored(t *testing.T) {
	requireDepthPriority(t)

	const configured = 8.7

	full := ladder.DepthOf(testutil.LadderDepthTrade(14, "LINK/USDT", 8, configured))
	if full.MaxDepth != 8 {
		t.Fatalf("ceiling = %d, want the fractional one floored", full.MaxDepth)
	}
	if full.RemainingCost != 0 {
		t.Errorf("a ladder that filled its floored ceiling keeps %f, want nothing", full.RemainingCost)
	}

	short := ladder.DepthOf(testutil.LadderDepthTrade(14, "LINK/USDT", 7, configured))
	if !isDepthPriorityCandidate(short) {
		t.Fatal("a ladder short of the floored ceiling still has an entry to keep the wallet for")
	}

	shallow := testutil.LadderDepthTrade(12, "ETH/USDT", 4, configured)
	view := []aggragates.LadderDepth{short}
	free := short.RemainingCost + nextEntryCostOf(t, shallow, short.RemainingCost) - 1

	reason := DepthPriorityHoldReason(priorityEvent(shallow, "buy", view, free), "stopLoss")
	if !strings.Contains(reason, "LINK/USDT at depth 7 of 8") {
		t.Fatalf("reason = %q, want the row to print the floored ceiling", reason)
	}
	if !strings.Contains(reason, "this ladder waits at depth 4 of 8") {
		t.Fatalf("reason = %q, want the held ladder's ceiling floored too", reason)
	}
}

// A ladder blocked on its next entry cannot take it, so the engines drop it
// from the view and nothing is kept for it any more. Two things follow, and
// both are the rule rather than a convenience: the row stops naming it, and
// the reservation passes to whichever ladder of the wallet is deepest now —
// always that ladder's OWN remainder, never the one that left.
func TestDepthPriorityStopsReservingForALadderThatLeavesTheView(t *testing.T) {
	requireDepthPriority(t)

	wallet := walletView()
	trade := testutil.LadderDepthTrade(12, "ETH/USDT", 4, walletDepths)

	reserve := viewReserve(t, wallet, walletAsset)
	ownCost := nextEntryCostOf(t, trade, reserve)

	held := DepthPriorityHoldReason(priorityEvent(trade, "buy", wallet, reserve+ownCost-1), "stopLoss")
	if !strings.Contains(held, "LINK/USDT at depth 7 of 8") {
		t.Fatalf("reason = %q, want the deepest ladder keeping the wallet", held)
	}

	released := withoutSymbol(wallet, "LINK/USDT")
	next := viewReserve(t, released, walletAsset)

	handedOn := DepthPriorityHoldReason(priorityEvent(trade, "buy", released, next+ownCost-1), "stopLoss")
	if !strings.Contains(handedOn, "SOL/USDT at depth 6 of 8") {
		t.Fatalf("reason = %q, want the next deepest ladder keeping the wallet", handedOn)
	}
	if reason := DepthPriorityHoldReason(priorityEvent(trade, "buy", released, next+ownCost), "stopLoss"); reason != "" {
		t.Fatalf("the wallet covers what the remaining ladders need, got %q", reason)
	}
}

// And when the ladder that left was the only one with entries still to fill,
// the wallet is reserved for nothing at all: every sibling arms its next
// entry on the same balance that was holding it a tick earlier.
func TestDepthPriorityReleasesEverybodyWhenTheLastCandidateLeaves(t *testing.T) {
	requireDepthPriority(t)

	wallet := []aggragates.LadderDepth{
		walletLadder(14, "LINK/USDT", 7),
		walletLadder(15, "HBAR/USDT", 0),
	}
	trade := testutil.LadderDepthTrade(12, "ETH/USDT", 4, walletDepths)

	reserve := viewReserve(t, wallet, walletAsset)
	free := reserve + nextEntryCostOf(t, trade, reserve) - 1

	if reason := DepthPriorityHoldReason(priorityEvent(trade, "buy", wallet, free), "stopLoss"); reason == "" {
		t.Fatal("the shallow ladder must wait while the deepest one is in the view")
	}

	released := withoutSymbol(wallet, "LINK/USDT")
	if reason := DepthPriorityHoldReason(priorityEvent(trade, "buy", released, free), "stopLoss"); reason != "" {
		t.Fatalf("a wallet with no ladder left to finish reserves nothing, got %q", reason)
	}
}

// withoutSymbol is the view an engine builds once a ladder blocks on its next
// entry: the trade is untouched and still ticking, it is only out of the
// wallet the gate reads.
func withoutSymbol(ladders []aggragates.LadderDepth, symbol string) []aggragates.LadderDepth {
	remaining := make([]aggragates.LadderDepth, 0, len(ladders))
	for _, candidate := range ladders {
		if candidate.Symbol == symbol {
			continue
		}
		remaining = append(remaining, candidate)
	}
	return remaining
}

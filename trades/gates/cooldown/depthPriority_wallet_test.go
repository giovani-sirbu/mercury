package cooldown

import (
	"strings"
	"testing"

	"github.com/giovani-sirbu/mercury/trades/aggragates"
	"github.com/giovani-sirbu/mercury/trades/internal/testutil"
	"github.com/giovani-sirbu/mercury/trades/ladder"
)

// The band that reserves the wallet is read off DepthPriorityMargin on EVERY
// configured ceiling, not only the eight-deep grid the example uses: a pair
// sized for three entries and one sized for twelve answer the same rule, and a
// literal boundary here would pin today's margin instead of the rule.
func TestDepthPriorityBandFollowsTheMarginOnEveryCeiling(t *testing.T) {
	requireDepthPriority(t)

	for _, ceiling := range []int{3, 8, 12} {
		firstCandidate := ceiling - DepthPriorityMargin + 1
		lastCandidate := ceiling - 1

		for depth := 0; depth <= ceiling+1; depth++ {
			candidate := aggragates.LadderDepth{TradeID: 1, Symbol: "LINK/USDT", Depth: depth, MaxDepth: ceiling}
			want := depth >= firstCandidate && depth <= lastCandidate

			if got := isDepthPriorityCandidate(candidate); got != want {
				t.Errorf("depth %d of %d: candidate = %v, want %v", depth, ceiling, got, want)
			}
		}
	}
}

// The same band decides the hold, not just the predicate: on a twelve-deep
// grid the ladder inside the margin reserves the wallet and the one a single
// depth outside it reserves nothing. Both depths are derived from the margin,
// so widening or narrowing it moves this test with the rule.
func TestDepthPriorityHoldsOnlyForALadderInsideTheMargin(t *testing.T) {
	requireDepthPriority(t)

	const ceiling = 12
	inside := ceiling - DepthPriorityMargin + 1
	outside := ceiling - DepthPriorityMargin

	shallow := testutil.LadderDepthTrade(12, "ETH/USDT", 4, ceiling)

	reserved := []aggragates.LadderDepth{{TradeID: 14, Symbol: "LINK/USDT", Depth: inside, MaxDepth: ceiling}}
	if reason := DepthPriorityHoldReason(priorityEvent(shallow, "buy", reserved), "stopLoss"); reason == "" {
		t.Errorf("a ladder at depth %d of %d is inside the margin and must reserve the wallet", inside, ceiling)
	}

	shy := []aggragates.LadderDepth{{TradeID: 14, Symbol: "LINK/USDT", Depth: outside, MaxDepth: ceiling}}
	if reason := DepthPriorityHoldReason(priorityEvent(shallow, "buy", shy), "stopLoss"); reason != "" {
		t.Errorf("a ladder at depth %d of %d is outside the margin and reserves nothing, got %q", outside, ceiling, reason)
	}
}

// A pair carrying no settings row has no ceiling to measure against, so it can
// never reserve the wallet — but it still spends it, so a sibling that is
// nearly finished still makes it wait. The two sides are separate on purpose:
// an unknown ceiling is a reason not to prioritize a ladder, never a reason to
// let it spend ahead of one whose ceiling is known.
func TestDepthPriorityHoldsALadderWhoseCeilingIsUnknown(t *testing.T) {
	requireDepthPriority(t)

	unknown := testutil.LadderDepthTrade(12, "ETH/USDT", 4, walletDepths)
	unknown.StrategyPair.StrategySettings = nil

	own := ladder.DepthOf(unknown)
	if own.MaxDepth != 0 {
		t.Fatalf("a pair without settings must report an unknown ceiling, got %d", own.MaxDepth)
	}
	if _, found := deepestDepthPriorityCandidate([]aggragates.LadderDepth{own}); found {
		t.Error("a ladder with an unknown ceiling must never reserve the wallet")
	}

	wallet := []aggragates.LadderDepth{own, {TradeID: 14, Symbol: "LINK/USDT", Depth: 7, MaxDepth: walletDepths}}
	reason := DepthPriorityHoldReason(priorityEvent(unknown, "buy", wallet), "stopLoss")
	if !strings.Contains(reason, "LINK/USDT at depth 7 of 8") {
		t.Fatalf("reason = %q, want the ladder with a known ceiling to take the entry", reason)
	}
}

// Depths is configured as a float and the ceiling the band is measured against
// is the floored one, the same read the smart take loss arms on: a grid
// configured for a fraction over eight entries is a grid of eight, so its
// eighth fill finishes it and releases the wallet instead of reserving it for
// an entry the ladder can never place.
func TestDepthPriorityMeasuresAFractionalCeilingFloored(t *testing.T) {
	requireDepthPriority(t)

	const configured = 8.7

	full := ladder.DepthOf(testutil.LadderDepthTrade(14, "LINK/USDT", 8, configured))
	if full.MaxDepth != 8 {
		t.Fatalf("ceiling = %d, want the fractional one floored", full.MaxDepth)
	}
	if isDepthPriorityCandidate(full) {
		t.Error("a ladder that filled its floored ceiling is finished and reserves nothing")
	}

	short := ladder.DepthOf(testutil.LadderDepthTrade(14, "LINK/USDT", 7, configured))
	if !isDepthPriorityCandidate(short) {
		t.Fatal("the entry before the floored ceiling is the one the wallet is reserved for")
	}

	shallow := testutil.LadderDepthTrade(12, "ETH/USDT", 4, configured)
	reason := DepthPriorityHoldReason(priorityEvent(shallow, "buy", []aggragates.LadderDepth{short}), "stopLoss")
	if !strings.Contains(reason, "LINK/USDT at depth 7 of 8") {
		t.Fatalf("reason = %q, want the row to print the floored ceiling", reason)
	}
	if !strings.Contains(reason, "this ladder waits at depth 4 of 8") {
		t.Fatalf("reason = %q, want the held ladder's ceiling floored too", reason)
	}
}

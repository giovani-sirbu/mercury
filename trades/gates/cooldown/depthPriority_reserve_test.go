package cooldown

import (
	"fmt"
	"strings"
	"testing"

	"github.com/giovani-sirbu/mercury/trades/aggragates"
	"github.com/giovani-sirbu/mercury/trades/internal/testutil"
	"github.com/giovani-sirbu/mercury/trades/ladder"
)

// The rule on views written out by hand, with each ladder's remainder chosen
// rather than derived from a fixture.
//
// A view built out of fixture ladders can only say what those ladders happen
// to cost, and on a grid that doubles each entry the deepest ladder is also
// the one with the smallest remainder — so a gate that picked the CHEAPEST
// candidate, or the dearest, would pass every such test. Writing the
// remainders out separates the two questions the rule really asks: which
// ladder the wallet is kept for (a depth, then a trade id), and how much is
// kept for it (that ladder's own amount, whatever any other ladder needs).

// ruleDepths is the grid the managed trades of this file are configured for.
const ruleDepths = 9

// ruleAsset is what the long ladders of this wallet spend.
const ruleAsset = "USDT"

// ruleTrade is the managed trade the gate is asked about: a real ladder, so
// its own depth, asset and next-entry cost are the ones the gate reads off a
// trade rather than numbers written beside it.
func ruleTrade(id uint, symbol string, depth int) aggragates.Trades {
	return testutil.LadderDepthTrade(id, symbol, depth, ruleDepths)
}

// ruleCost is what that trade's next entry spends.
func ruleCost(t *testing.T, trade aggragates.Trades) float64 {
	t.Helper()

	_, cost := ladder.NextEntryCost(trade, 0)
	if cost <= 0 {
		t.Fatalf("%s must cost something to place its next entry", trade.Symbol)
	}

	return cost
}

// ruleLadder is one row of a written-out wallet view.
func ruleLadder(id uint, symbol string, depth, maxDepth int, asset string, remaining float64) aggragates.LadderDepth {
	return aggragates.LadderDepth{
		TradeID:       id,
		Symbol:        symbol,
		Depth:         depth,
		MaxDepth:      maxDepth,
		Asset:         asset,
		RemainingCost: remaining,
	}
}

// ruleKeeps is the fragment of the hold row that names the ladder the wallet
// is being kept for.
func ruleKeeps(keeper aggragates.LadderDepth) string {
	return fmt.Sprintf("%s at depth %d of %d", keeper.Symbol, keeper.Depth, keeper.MaxDepth)
}

// The wallet is kept for the DEEPEST candidate and for that ladder's own
// remainder — not for the ladder that needs the most, and not for the sum of
// what the wallet owes. Here the deepest ladder is the cheapest one in the
// view, so a gate that reserved the largest amount, or added the amounts up,
// would keep a wallet nobody asked it to keep.
func TestDepthPriorityRuleKeepsTheDeepestLaddersOwnRemainder(t *testing.T) {
	requireDepthPriority(t)

	keeper := ruleLadder(14, "LINK/USDT", 6, ruleDepths, ruleAsset, 500)
	view := []aggragates.LadderDepth{
		ruleLadder(11, "BTC/USDT", 3, ruleDepths, ruleAsset, 100000),
		keeper,
		ruleLadder(13, "SOL/USDT", 5, ruleDepths, ruleAsset, 90000),
	}

	own := ruleTrade(12, "ETH/USDT", 2)
	ownCost := ruleCost(t, own)

	reason := DepthPriorityHoldReason(priorityEvent(own, "buy", view, keeper.RemainingCost+ownCost-1), "stopLoss")
	if !strings.Contains(reason, ruleKeeps(keeper)) {
		t.Fatalf("reason = %q, want the deepest ladder of the view keeping the wallet", reason)
	}

	if reason := DepthPriorityHoldReason(priorityEvent(own, "buy", view, keeper.RemainingCost+ownCost), "stopLoss"); reason != "" {
		t.Fatalf("a wallet level with the deepest ladder's own remainder must release the entry, got %q", reason)
	}
}

// Two ladders at the same depth are both worth finishing, so the tie is
// broken by the lowest trade id — and the loser is one of the others, held
// for the WINNER's remainder. The two remainders are far apart here, so a
// gate that named one ladder and reserved the other's amount would show.
func TestDepthPriorityRuleBreaksTiesOnTheLowestTradeIDAndHoldsTheLoser(t *testing.T) {
	requireDepthPriority(t)

	winner := ruleLadder(9, "DOT/USDT", 6, ruleDepths, ruleAsset, 1000)
	loser := ruleLadder(14, "LINK/USDT", 6, ruleDepths, ruleAsset, 80000)
	view := []aggragates.LadderDepth{loser, winner}

	own := ruleTrade(loser.TradeID, loser.Symbol, loser.Depth)
	ownCost := ruleCost(t, own)

	reason := DepthPriorityHoldReason(priorityEvent(own, "buy", view, winner.RemainingCost+ownCost-1), "stopLoss")
	if !strings.Contains(reason, ruleKeeps(winner)) {
		t.Fatalf("reason = %q, want the lowest trade id of the tied depth keeping the wallet", reason)
	}
	if strings.Contains(reason, loser.Symbol+" at depth") {
		t.Fatalf("reason = %q, want the tie loser held rather than exempt", reason)
	}

	// Level with the WINNER's remainder the entry goes through, although the
	// tie loser's own remainder is far larger: what is kept is one ladder's
	// amount, and the loser is not that ladder.
	if reason := DepthPriorityHoldReason(priorityEvent(own, "buy", view, winner.RemainingCost+ownCost), "stopLoss"); reason != "" {
		t.Fatalf("a wallet level with the reserved ladder's remainder must release the tie loser, got %q", reason)
	}
}

// A ladder that cannot be finished keeps nothing: one already at its ceiling
// has no entry left to be kept for, one that has filled nothing has no
// position to finish, and one whose pair carries no settings row has no
// ceiling to have entries left against. Each is given a remainder large
// enough to hold the whole wallet, so what releases the entry is the
// candidacy rule alone.
func TestDepthPriorityRuleKeepsNothingForALadderWithNoPositionOrNoCeiling(t *testing.T) {
	requireDepthPriority(t)

	own := ruleTrade(12, "ETH/USDT", 2)

	for name, ineligible := range map[string]aggragates.LadderDepth{
		"nothing filled yet":    ruleLadder(15, "HBAR/USDT", 0, ruleDepths, ruleAsset, 100000),
		"no ceiling configured": ruleLadder(13, "SOL/USDT", 6, 0, ruleAsset, 100000),
	} {
		view := []aggragates.LadderDepth{ineligible}

		if reason := DepthPriorityHoldReason(priorityEvent(own, "buy", view, 0), "stopLoss"); reason != "" {
			t.Errorf("%s: a ladder with nothing to stand in front for must keep nothing, got %q", name, reason)
		}

		// The same view with one ladder that DOES stand in front — deeper
		// than the managed trade, so the managed trade cannot outrank it —
		// to show the gate is live and the releases above are the candidacy
		// rule rather than an inert gate.
		keeper := ruleLadder(21, "ADA/USDT", 5, ruleDepths, ruleAsset, 100)
		reason := DepthPriorityHoldReason(priorityEvent(own, "buy", append(view, keeper), 0), "stopLoss")
		if !strings.Contains(reason, ruleKeeps(keeper)) {
			t.Errorf("%s: reason = %q, want the one standing ladder keeping the wallet", name, reason)
		}
	}
}

// A ladder that has reached its ceiling keeps its PLACE in front of the
// wallet, and keeps it until it closes. It needs nothing more, so what it
// reserves is nothing and the hold collapses to the one question left: can
// this wallet pay for the entry being weighed at all. It can — the sibling
// buys, and that is the release at the ceiling. It cannot — the sibling
// WAITS.
//
// Waiting is the point. Ending a full ladder's turn instead releases every
// sibling at once into a wallet the ladder has just spent down to its last
// depth: each one reaches the funds gate, is blocked rather than held, and a
// blocked ladder leaves the view. By the time the close refills the wallet
// the deepest siblings are no longer in it, and the shallowest ladder still
// active inherits a wallet it never earned.
//
// A ladder whose configured ceiling has been retuned BELOW the entries it
// already filled is the same ladder for this purpose: it has no entry left to
// take and nothing left to reserve, and it goes on holding its place until it
// closes.
func TestDepthPriorityRuleKeepsAFullLadderInFrontUntilItCloses(t *testing.T) {
	requireDepthPriority(t)

	own := ruleTrade(12, "ETH/USDT", 2)
	ownCost := ruleCost(t, own)

	// Nothing is reserved by either: a ladder with no entry left to place
	// names no amount, which is what DepthOf reports for both of these.
	for name, full := range map[string]aggragates.LadderDepth{
		"at its ceiling":               ruleLadder(14, "LINK/USDT", ruleDepths, ruleDepths, ruleAsset, 0),
		"past a ceiling since retuned": ruleLadder(11, "BTC/USDT", 6, 4, ruleAsset, 0),
	} {
		view := []aggragates.LadderDepth{full}

		// A wallet that covers the entry lets it through: nothing is being
		// kept back, so there is nothing for the sibling to break into.
		if reason := DepthPriorityHoldReason(priorityEvent(own, "buy", view, ownCost), "stopLoss"); reason != "" {
			t.Errorf("%s: a wallet that can pay for the entry must let it through, got %q", name, reason)
		}

		// A wallet that does not is held HERE, and so stays active, stays in
		// the view and keeps its rank until the close refills the wallet.
		reason := DepthPriorityHoldReason(priorityEvent(own, "buy", view, ownCost-1), "stopLoss")
		want := fmt.Sprintf(
			"cooldown: depth priority, %s at depth %d of %d holds the wallet until it closes, this ladder waits at depth 2 of %d",
			full.Symbol, full.Depth, full.MaxDepth, ruleDepths,
		)
		if reason != want {
			t.Errorf("%s: reason = %q, want %q", name, reason, want)
		}
	}
}

// The managed trade is ranked with the rest, by the same ordering, so a
// ladder level with the view's best is split from it on the trade id — and
// when the split goes the managed trade's way it waits for nobody.
//
// It matters because a ladder is NOT in the view while it is blocked on its
// next entry, and the tick it is re-admitted on is the tick it competes
// again. Judged against the view alone it would find a level sibling in front
// of it and wait there, whatever its id — the ordering would depend on which
// ladder happened to be out of the view at that moment rather than on the
// ladders themselves.
func TestDepthPriorityRuleSplitsALevelManagedTradeOnTheTradeID(t *testing.T) {
	requireDepthPriority(t)

	const level = 5

	for name, ids := range map[string]struct{ own, view uint }{
		"the managed trade holds the lower id": {own: 9, view: 14},
		"the view's ladder holds it":           {own: 14, view: 9},
	} {
		own := ruleTrade(ids.own, "ETH/USDT", level)
		ownCost := ruleCost(t, own)

		keeper := ruleLadder(ids.view, "LINK/USDT", level, ruleDepths, ruleAsset, 100000)
		view := []aggragates.LadderDepth{keeper}

		// A wallet far under both the reserve and the entry, so nothing but
		// the ordering decides.
		reason := DepthPriorityHoldReason(priorityEvent(own, "buy", view, ownCost-1), "stopLoss")

		if ids.own < ids.view {
			if reason != "" {
				t.Errorf("%s: a level ladder with the lower id waits for nobody, got %q", name, reason)
			}
			continue
		}
		if !strings.Contains(reason, ruleKeeps(keeper)) {
			t.Errorf("%s: reason = %q, want the level ladder with the lower id in front", name, reason)
		}
	}
}

// Ladders funded from different currencies never take money from each other.
// The deepest ladder of the view spends another asset here, so the wallet is
// kept for the deepest one spending the managed trade's asset — and a view
// holding nothing but foreign ladders keeps nothing at all, however empty the
// wallet is.
func TestDepthPriorityRuleScopesTheViewToTheAssetTheEntrySpends(t *testing.T) {
	requireDepthPriority(t)

	foreign := ruleLadder(31, "BTC/EUR", 8, ruleDepths, "EUR", 100000)
	keeper := ruleLadder(13, "SOL/USDT", 5, ruleDepths, ruleAsset, 900)

	own := ruleTrade(12, "ETH/USDT", 2)
	ownCost := ruleCost(t, own)

	reason := DepthPriorityHoldReason(
		priorityEvent(own, "buy", []aggragates.LadderDepth{foreign, keeper}, keeper.RemainingCost+ownCost-1),
		"stopLoss",
	)
	if !strings.Contains(reason, ruleKeeps(keeper)) {
		t.Fatalf("reason = %q, want the deepest ladder spending this trade's asset", reason)
	}

	if reason := DepthPriorityHoldReason(priorityEvent(own, "buy", []aggragates.LadderDepth{foreign}, 0), "stopLoss"); reason != "" {
		t.Fatalf("a view of ladders spending another asset must keep nothing, got %q", reason)
	}

	// A ladder whose symbol cannot be split names no asset, and an entry with
	// no asset matches no other — otherwise every unparsable pair would be in
	// every wallet at once.
	anonymous := ruleLadder(41, "LINKUSDT", 8, ruleDepths, "", 100000)
	if reason := DepthPriorityHoldReason(priorityEvent(own, "buy", []aggragates.LadderDepth{anonymous}, 0), "stopLoss"); reason != "" {
		t.Fatalf("a ladder naming no asset must keep nothing, got %q", reason)
	}
}

// The quote an inverse trade of this wallet has already committed is not free
// to spend, and the funds gate subtracts it before it compares — so the
// reserve is measured against the same number. The deduction is the long
// ladder's alone: an inverse ladder spends base, and taking its own
// commitment off its base balance would charge it twice.
func TestDepthPriorityRuleSubtractsTheQuoteCommittedByInverseTrades(t *testing.T) {
	requireDepthPriority(t)

	keeper := ruleLadder(14, "LINK/USDT", 6, ruleDepths, ruleAsset, 5000)
	own := ruleTrade(12, "ETH/USDT", 2)
	ownCost := ruleCost(t, own)

	covered := priorityEvent(own, "buy", []aggragates.LadderDepth{keeper}, keeper.RemainingCost+ownCost)
	if reason := DepthPriorityHoldReason(covered, "stopLoss"); reason != "" {
		t.Fatalf("the wallet covers both before any commitment, got %q", reason)
	}

	committed := covered
	committed.Params.InverseUsedAmount = []aggragates.UsedAmountResult{{QuoteCurrency: ruleAsset, UsedAmount: 1}}
	if reason := DepthPriorityHoldReason(committed, "stopLoss"); reason == "" {
		t.Fatal("quote an inverse trade has committed must not count towards the reserve")
	}

	// A commitment in some other currency is not this wallet's.
	elsewhere := covered
	elsewhere.Params.InverseUsedAmount = []aggragates.UsedAmountResult{{QuoteCurrency: "EUR", UsedAmount: 100000}}
	if reason := DepthPriorityHoldReason(elsewhere, "stopLoss"); reason != "" {
		t.Fatalf("a commitment in another currency must not shrink this balance, got %q", reason)
	}

	// The inverse side: the managed ladder spends base, and its balance is
	// read whole.
	inverseOwn := ruleTrade(32, "BTC/USDT", 2)
	inverseOwn.Inverse = true
	for index := range inverseOwn.History {
		inverseOwn.History[index].Type = "SELL"
	}

	inverseKeeper := ruleLadder(31, "BTC/USDT", 6, ruleDepths, "BTC", 4)
	_, inverseCost := ladder.NextEntryCost(inverseOwn, 0)
	if inverseCost <= 0 {
		t.Fatalf("the inverse fixture must cost something to place its next entry, got %f", inverseCost)
	}

	inverseEvent := blindPriorityEvent(inverseOwn, "buy", []aggragates.LadderDepth{inverseKeeper})
	inverseEvent.Params.WalletFree = []aggragates.AssetFree{{Asset: "BTC", Free: inverseKeeper.RemainingCost + inverseCost}}
	inverseEvent.Params.InverseUsedAmount = []aggragates.UsedAmountResult{{QuoteCurrency: "BTC", UsedAmount: 100000}}

	if reason := DepthPriorityHoldReason(inverseEvent, "stopLoss"); reason != "" {
		t.Fatalf("an inverse ladder's base balance must be read whole, got %q", reason)
	}
}

// A balance the engine could not name holds nothing. Three ways to have none
// — no balances at all, an empty set, and a set naming every asset but this
// one — and all of them are the same posture: the gate never parks a wallet
// on a guess, which is what a failed hermes read degrades to.
func TestDepthPriorityRuleHoldsNothingWithoutABalanceItCanName(t *testing.T) {
	requireDepthPriority(t)

	view := []aggragates.LadderDepth{ruleLadder(14, "LINK/USDT", 6, ruleDepths, ruleAsset, 100000)}
	own := ruleTrade(12, "ETH/USDT", 2)

	for name, balances := range map[string][]aggragates.AssetFree{
		"no balances at all":     nil,
		"an empty set":           {},
		"every asset but this":   {{Asset: "EUR", Free: 100000}, {Asset: "BTC", Free: 4}},
		"an asset named nothing": {{Asset: "", Free: 100000}},
	} {
		event := blindPriorityEvent(own, "buy", view)
		event.Params.WalletFree = balances

		if reason := DepthPriorityHoldReason(event, "stopLoss"); reason != "" {
			t.Errorf("%s: an unknown balance must hold nothing, got %q", name, reason)
		}
	}

	// Named and empty is not unknown: a wallet the engine says is empty is a
	// wallet that cannot cover the reserve.
	known := blindPriorityEvent(own, "buy", view)
	known.Params.WalletFree = []aggragates.AssetFree{{Asset: ruleAsset, Free: 0}}
	if reason := DepthPriorityHoldReason(known, "stopLoss"); reason == "" {
		t.Fatal("a balance the engine named as empty must hold the entry")
	}

	// And a managed trade whose own symbol cannot be split names no asset to
	// look a balance up by, so it is held by nothing however full the view
	// is — the same unknown, reached from the trade's side rather than the
	// wallet's.
	anonymous := ruleTrade(41, "ETHUSDT", 2)
	unnamed := blindPriorityEvent(anonymous, "buy", view)
	unnamed.Params.WalletFree = []aggragates.AssetFree{{Asset: ruleAsset, Free: 0}}
	if reason := DepthPriorityHoldReason(unnamed, "stopLoss"); reason != "" {
		t.Fatalf("a trade whose pair cannot be split has no balance to weigh, got %q", reason)
	}
}

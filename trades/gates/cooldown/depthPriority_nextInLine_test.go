package cooldown

import (
	"fmt"
	"testing"

	"github.com/giovani-sirbu/mercury/trades/aggragates"
	"github.com/giovani-sirbu/mercury/trades/internal/testutil"
	"github.com/giovani-sirbu/mercury/trades/ladder"
)

// The wallet a refill finds while a full ladder still stands in front of it:
// the full ladder keeps its place until it closes and keeps nothing, and the
// ladders behind it all want what the wallet holds. The wallet is kept for
// the NEXT ladder in line that has depths left, and only that ladder has
// nothing but the full one ahead of it.
//
// The level ladders are real trades, built so that the plan, the reserve and
// the trade id do not all agree on which of them is in front: the gate is
// right only if it ranks on the plan, falls back to the id, and keeps the
// reserve of the ladder it ranked first.

// refillDepths is the grid every ladder of this wallet is configured for.
const refillDepths = 9

// refillLevel is the depth the level ladders stand at.
const refillLevel = 6

// refillFull is a ladder that has filled its last depth and is still in the
// view, its close not settled yet.
func refillFull(id uint, symbol string) aggragates.LadderDepth {
	return ladder.DepthOf(testutil.LadderDepthTrade(id, symbol, refillDepths, refillDepths))
}

// refillLevelLadder is a ladder at the level depth: firstQuantity scales what
// it planned from its fills, positionPrice moves what it reserves.
func refillLevelLadder(id uint, symbol string, firstQuantity, positionPrice float64) aggragates.Trades {
	return testutil.RefillLadderTrade(id, symbol, refillLevel, refillDepths, firstQuantity, positionPrice)
}

// refillRow is the row of a ladder waiting on named: for its remaining
// depths, or — named being full — for nothing but its close.
func refillRow(named aggragates.LadderDepth, ownDepth int) string {
	waitsFor := "keeps the wallet for its remaining depths"
	if named.Depth >= named.MaxDepth {
		waitsFor = "holds the wallet until it closes"
	}
	return fmt.Sprintf("cooldown: depth priority, %s at depth %d of %d %s, this ladder waits at depth %d of %d",
		named.Symbol, named.Depth, named.MaxDepth, waitsFor, ownDepth, refillDepths)
}

// refillCost is what the ladder's next entry spends.
func refillCost(t *testing.T, trade aggragates.Trades) float64 {
	t.Helper()

	_, cost := ladder.NextEntryCost(trade, 0)
	if cost <= 0 {
		t.Fatalf("%s must cost something to place its next entry", trade.Symbol)
	}
	return cost
}

// assertRefillHold fails unless the ladder, arming on a wallet of free, is
// held with exactly want — or, with want empty, let through.
func assertRefillHold(t *testing.T, own aggragates.Trades, view []aggragates.LadderDepth, free float64, want string) {
	t.Helper()

	if got := DepthPriorityHoldReason(priorityEvent(own, "buy", view, free), "stopLoss"); got != want {
		t.Fatalf("%s on a wallet of %v: reason = %q, want %q", own.Symbol, free, got, want)
	}
}

// assertNextInLine reads one refill from both sides, on the view the engines
// build with both ladders in it: first is in front and needs only its own
// entry, second waits for first's remaining depths — on a wallet that pays
// for its own entry too — until the wallet covers them and that entry.
func assertNextInLine(t *testing.T, full aggragates.LadderDepth, first, second aggragates.Trades) {
	t.Helper()

	firstView := ladder.DepthOf(first)
	view := []aggragates.LadderDepth{full, firstView, ladder.DepthOf(second)}

	firstCost := refillCost(t, first)
	assertRefillHold(t, first, view, firstCost, "")
	assertRefillHold(t, first, view, firstCost-1, refillRow(full, refillLevel))

	secondCost := refillCost(t, second)
	assertRefillHold(t, second, view, secondCost, refillRow(firstView, refillLevel))
	assertRefillHold(t, second, view, firstView.RemainingCost+secondCost-1, refillRow(firstView, refillLevel))
	assertRefillHold(t, second, view, firstView.RemainingCost+secondCost, "")
}

// Two ladders at the same depth, and the one planned cheaper to finish from
// its last fill is in front, although it reserves MORE of the wallet from
// where its position sits. Once it holds the higher trade id and once the
// lower, so neither the id nor the reserve can pass for the plan.
func TestDepthPriorityPutsTheLevelLadderPlannedCheaperInFront(t *testing.T) {
	requireDepthPriority(t)

	full := refillFull(13, "SOL/USDT")

	for name, pair := range map[string]struct{ cheaper, dearer aggragates.Trades }{
		"the higher trade id planned cheaper": {
			cheaper: refillLevelLadder(14, "LINK/USDT", 1, 92),
			dearer:  refillLevelLadder(12, "ETH/USDT", 1.05, 80),
		},
		"the lower trade id planned cheaper": {
			cheaper: refillLevelLadder(12, "ETH/USDT", 1, 92),
			dearer:  refillLevelLadder(14, "LINK/USDT", 1.05, 80),
		},
	} {
		t.Run(name, func(t *testing.T) {
			cheaper, dearer := ladder.DepthOf(pair.cheaper), ladder.DepthOf(pair.dearer)
			if cheaper.PlannedRemainingCost >= dearer.PlannedRemainingCost || cheaper.RemainingCost <= dearer.RemainingCost {
				t.Fatalf("the fixture must put %s ahead on the plan and behind on the reserve: %+v against %+v", cheaper.Symbol, cheaper, dearer)
			}

			assertNextInLine(t, full, pair.cheaper, pair.dearer)
		})
	}
}

// Level on the plan as well, the lower trade id is in front — and the
// reserve, smaller on the other ladder here, decides nothing.
func TestDepthPriorityFallsBackToTheTradeIDOnALevelPlan(t *testing.T) {
	requireDepthPriority(t)

	lower := refillLevelLadder(12, "ETH/USDT", 1, 92)
	higher := refillLevelLadder(14, "LINK/USDT", 1, 80)

	lowerView, higherView := ladder.DepthOf(lower), ladder.DepthOf(higher)
	if lowerView.PlannedRemainingCost != higherView.PlannedRemainingCost || higherView.RemainingCost >= lowerView.RemainingCost {
		t.Fatalf("the fixture must plan both alike and reserve less on the higher id: %+v against %+v", lowerView, higherView)
	}

	assertNextInLine(t, refillFull(13, "SOL/USDT"), lower, higher)
}

// With nothing but full ladders ahead, the ladder next in line is under
// today's rule and row: it buys on any wallet that pays for its entry and
// otherwise waits on the row that names the close. The ladders behind it —
// shallower, or level and planned dearer — keep nothing from it, and of two
// full ladders the row names the one the ordering puts first.
func TestDepthPriorityHoldsTheNextInLineOnlyForItsOwnEntry(t *testing.T) {
	requireDepthPriority(t)

	own := refillLevelLadder(14, "LINK/USDT", 1, 92)
	first := refillFull(11, "BTC/USDT")
	view := []aggragates.LadderDepth{
		refillFull(13, "SOL/USDT"),
		first,
		ladder.DepthOf(refillLevelLadder(12, "ETH/USDT", 1.05, 80)),
		ladder.DepthOf(testutil.RefillLadderTrade(15, "HBAR/USDT", refillLevel-2, refillDepths, 1, 92)),
		ladder.DepthOf(own),
	}

	ownCost := refillCost(t, own)
	assertRefillHold(t, own, view, ownCost, "")
	assertRefillHold(t, own, view, ownCost-1, refillRow(first, refillLevel))
}

// A first fill has no depth to rank with, so every ladder of the view is
// ahead of it — and while a full ladder stands in front, what it waits for is
// the next one's remaining depths, not the full ladder's close. Its entry is
// the initial bid off the wallet it is sized against.
func TestDepthPriorityHoldsAFirstFillForTheNextInLine(t *testing.T) {
	requireDepthPriority(t)

	next := ladder.DepthOf(refillLevelLadder(14, "LINK/USDT", 1, 92))
	view := []aggragates.LadderDepth{refillFull(13, "SOL/USDT"), next}
	newcomer := testutil.LadderDepthTrade(21, "ADA/USDT", 0, refillDepths)

	firstFill := func(free float64) string {
		return DepthPriorityHoldReason(priorityEvent(newcomer, "new", view, free), "buy")
	}

	// The remainder and not a unit more: any bid at all breaks into it.
	if got, want := firstFill(next.RemainingCost), refillRow(next, 0); got != want {
		t.Fatalf("reason = %q, want %q", got, want)
	}

	// Twice the remainder carries the bid too — checked here, not assumed.
	free := 2 * next.RemainingCost
	if _, bid := ladder.NextEntryCost(newcomer, free); bid <= 0 || free-bid < next.RemainingCost {
		t.Fatalf("a wallet of %v with a bid of %v must leave the remainder of %v covered", free, bid, next.RemainingCost)
	}
	if got := firstFill(free); got != "" {
		t.Fatalf("a wallet that carries the remainder and the bid must let a first fill through, got %q", got)
	}
}

// The managed trade's own row in the view never holds it, whatever it says:
// a view read a moment before this tick, or priced off another copy of the
// trade, can carry it at another depth or cost than the trade itself. Every
// copy below WOULD hold the trade under any other id — shown by renumbering
// it — so what releases the trade is its own id and nothing else.
func TestDepthPriorityNeverHoldsATradeBehindItsOwnRow(t *testing.T) {
	requireDepthPriority(t)

	own := refillLevelLadder(14, "LINK/USDT", 1, 92)
	current := ladder.DepthOf(own)

	cheaper, deeper, shallower := current, current, current
	cheaper.PlannedRemainingCost /= 2
	deeper.Depth++
	shallower.Depth--

	// A trade whose pair lost its settings row ranks with nobody, so a copy
	// priced while the row was still there is ahead of it even a depth
	// shallower than the trade.
	unranked := own
	unranked.StrategyPair.StrategySettings = nil

	for name, tc := range map[string]struct {
		own  aggragates.Trades
		copy aggragates.LadderDepth
	}{
		"planned cheaper":                       {own, cheaper},
		"a depth deeper":                        {own, deeper},
		"a depth shallower, the trade unranked": {unranked, shallower},
	} {
		_, free := ladder.NextEntryCost(tc.own, 0)

		stale := []aggragates.LadderDepth{refillFull(13, "SOL/USDT"), tc.copy}
		if got := DepthPriorityHoldReason(priorityEvent(tc.own, "buy", stale, free), "stopLoss"); got != "" {
			t.Errorf("%s: the trade's own row held it: %q", name, got)
		}

		renumbered := tc.copy
		renumbered.TradeID = 12
		other := []aggragates.LadderDepth{refillFull(13, "SOL/USDT"), renumbered}
		if got := DepthPriorityHoldReason(priorityEvent(tc.own, "buy", other, free), "stopLoss"); got == "" {
			t.Errorf("%s: the same row under another trade id must hold the trade", name)
		}
	}
}
